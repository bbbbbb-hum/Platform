# AgentPlatform 实现与难点说明（详细版）

## 1. 项目定位与职责边界
AgentPlatform 是“对外 MCP 网关 + 任务链运行时”：
- 对外统一入口：`/mcp-server/{server_id}`
- 内部执行链路：`服务 -> 任务链 -> 节点 -> 连接池 -> 上游 MCP`
- 不负责上游服务进程编排，只负责连接与调用

## 2. 启动流程与基础设施初始化
启动入口：`src/main.go`
核心步骤：
1. 读取配置（DB/Redis/NATS/Server）
2. 初始化日志、DB、Redis、NATS
3. 初始化连接池维护协程
4. 初始化 MCP 服务列表（读取 DB 并构建内存实例）
5. 注册 HTTP 路由与 MCP Streamable Handler

说明：
- MCP 网关路由以 `server_id` 为 key 查 `McpServicesMap`
- 运行态以内存缓存为准，启动会全量重建

## 3. 服务实例（Server）与初始化逻辑
服务实例结构（关键字段）：
- `mcpServer`：对外 MCP 协议处理
- `ChainInstance`：任务链执行器
- `toolDescList`：链路汇总工具列表
- `LimitCalls/LimitType`：调用限制

初始化路径：
1. `servers.Initialize()` 从 `ae_mcp_services` 读取服务列表
2. 逐条调用 `createMcpServer(service)`
3. 以 `server_id` 写入 `McpServicesMap`

关键约束：
- `is_created=true` 才进入运行态
- 工具列表为空会自动 `enabled=false`
- 工具列表会写入 `ae_mcp_tools` 快照

## 4. 任务链与节点模型
任务链结构：
- `ae_mcp_task_chain`：链路定义
- `ae_mcp_task_node`：节点定义（顺序在链里）

初始化过程：
1. `ChainInstance.Init` 获取链路节点列表
2. 按 `node_handle` 创建节点实例
3. 调用节点 `Init`，写入 `NodeInstances`

节点类型：
- ProxyNode：核心节点，负责对接上游 MCP
- 统计类节点：调用前后采集
- 空节点/日志节点：占位或扩展

## 5. ProxyNode 与 node_config
`node_config` 是运行时连接配置（URL/Protocol/Timeout/MaxConnect）：
- `ProxyNode.Init` 解析 node_config
- 成功后调用连接池 `InitializeNode(node_id, ...)`
- 初始化失败会返回明确错误（node_config 缺失或无效）

语义：
- node_config 是连接池的唯一来源
- 不经过 node_config 的节点不会建连

## 6. 连接池设计（节点级）
连接池以“节点”为粒度，而不是服务：
- key = `node:{node_id}`
- node_id 来源：`ae_mcp_task_node.id`
- ProxyNode 初始化时调用 `InitializeNode(node_id, ...)`

原因：
- 同一服务在不同链路/节点配置不同（URL/Timeout/MaxConnect）
- 节点级隔离可以避免配置串扰
- 节点失败只重建该节点连接，不影响其他链路

运行规则：
- 初始化时先建连 → ListTools → 成功后写入 pool map
- 失败不会写入 map，避免半初始化对象

## 7. 协议与连接模型
当前支持协议：
- `http`（MCP Streamable HTTP）
- 非 http 协议直接拒绝

连接模型：
- 单节点单实例，多连接
- 每条连接独立 transport
- 初始化时会拉取工具列表做“预热”

## 8. 超时与重试
节点配置中主要超时：
- ConnectTimeout：建立连接超时
- CallTimeout：调用上游 MCP 工具超时

行为：
- 初始化时 ListTools 预热
- 调用失败会尝试重建连接并重试一次

## 9. 请求执行与工具注册
执行链路：
1. 外部请求进入 `server_id` 对应的 `Server`
2. Server 执行 `ChainInstance`
3. ProxyNode 调用连接池 `CallToolByNode`
4. 连接池选择连接并执行上游 MCP 工具

工具注册：
- 从链路汇总 `toolDescList`
- 注册到 MCP Server
- 同步写入 `ae_mcp_tools` 快照

## 10. 数据模型关键字段语义
- `ae_mcp_services.is_created`：是否进入运行态加载
- `ae_mcp_services.enabled`：是否对外上线（无工具自动下线）
- `server_id`：对外网关路由标识
- `ae_mcp_task_node.id`：连接池节点 key 的唯一标识

## 11. 运行态缓存与日志
运行态缓存：
- `McpServicesMap`：`server_id -> Server`
- 连接池 `services map`：`node_key -> ExternalService`
- `RequestLogs`：链路日志缓存

日志流：
- 可用 NATS 批量入库
- 也可降级为 DB 直写

## 12. 公网场景必备边界
若对公网开放，需要确保：
- API Key 鉴权启用
- 限流与配额（按 key/user/server_id）
- 上游 URL 白名单/校验（防止成为跳板）
- 调用超时/重试策略收敛
- 监控与日志可按 node/server 维度追踪

## 13. 重点难点（对总监说明口径）
1. **服务加载策略**：is_created 决定是否进入运行态；enabled 代表是否上线
2. **节点级连接池**：按节点隔离配置与故障，避免链路互相干扰
3. **工具快照写库**：初始化时全量重建工具列表并落库
4. **节点初始化失败处理**：ProxyNode 无可用 node_config 会拒绝初始化
5. **公网安全边界**：鉴权/限流/白名单/观测缺一不可
