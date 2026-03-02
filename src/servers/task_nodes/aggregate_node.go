package task_nodes

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/models"
	"AgentEarth_AgentPlatform/src/servers/types"
	"fmt"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// AggregateNode 聚合节点，将多个子节点的工具聚合到一个服务中
type AggregateNode struct {
	mutex          sync.RWMutex
	NodeInfo       *types.NodeInfo
	mapToolInfos   map[string]*AggregatedTool // 聚合后工具名 -> 工具信息
	mapNodeHandles map[string]types.Processor // 节点名 -> 节点实例
	subNodeNames   []string                   // 配置的子节点名称列表
	initConfig     types.InitConfig           // 保存初始化配置用于热更新
	currentConfig  string                     // 当前配置的原始字符串，用于热更新时对比
}

// AggregatedTool 聚合后的工具信息
type AggregatedTool struct {
	OriginalName string          // 原始工具名
	NodeName     string          // 所属节点名（去空格后）
	ToolDesc     *types.ToolDesc // 工具描述
	Node         types.Processor // 对应的节点实例
}

// Init 初始化聚合节点
func (a *AggregateNode) Init(config types.InitConfig) error {
	logger.Info("初始化聚合节点", zap.Int32("node_id", config.NodeModel.Id))

	a.initConfig = config
	// 1. 保存节点基本信息
	a.NodeInfo = &types.NodeInfo{
		ServiceID:   config.ServiceID,
		ChainID:     config.ChainModel.Id,
		NodeID:      config.NodeModel.Id,
		NodeHandle:  config.NodeModel.NodeHandle,
		NodeName:    config.NodeModel.NodeName,
		Description: config.NodeModel.Description,
		Enabled:     true,
	}

	// 2. 解析 node_config，得到子节点名称列表 ["天气节点", "搜索节点"]
	subNodeNames, err := ParseAggregateConfig(config.NodeModel.NodeConfig) //ParseAggregateConfig是解析聚合节点配置，返回子节点名称列表(去空格)
	if err != nil {
		return fmt.Errorf("parse_aggregate_config_failed: %w", err)
	}
	a.subNodeNames = subNodeNames

	// 3. 保存当前配置用于热更新时对比
	a.currentConfig = config.NodeModel.NodeConfig

	// 初始化子节点
	return a.initSubNodes(config)
}

// initSubNodes 初始化所有子节点
func (a *AggregateNode) initSubNodes(config types.InitConfig) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 1. 初始化两个map
	a.mapNodeHandles = make(map[string]types.Processor) // 存节点实例
	a.mapToolInfos = make(map[string]*AggregatedTool)   // 存工具信息

	nodeModel := &models.AeMcpTaskNode{}
	for _, nodeName := range a.subNodeNames {
		// 根据节点名称查询节点信息
		subNode, err := nodeModel.GetByNodeName(nodeName)
		if err != nil {
			logger.Error("查询子节点失败", zap.String("node_name", nodeName), zap.Error(err))
			continue
		}

		// 防止循环依赖：子节点不能是聚合节点
		if subNode.NodeHandle == "aggregate_handle" {
			logger.Error("子节点不能是聚合节点", zap.String("node_name", nodeName))
			continue
		}

		// 创建子节点实例
		processor, ok := CreateNodeByType(subNode.NodeHandle)
		if !ok {
			logger.Error("不支持的子节点类型", zap.String("node_handle", subNode.NodeHandle))
			continue
		}

		// 初始化子节点
		err = processor.Init(types.InitConfig{
			ServiceID:  config.ServiceID,
			ChainModel: config.ChainModel,
			NodeModel:  subNode,
		})
		if err != nil {
			logger.Error("初始化子节点失败", zap.String("node_name", nodeName), zap.Error(err))
			continue
		}

		// 存储子节点实例
		cleanName := strings.ReplaceAll(nodeName, " ", "_")
		a.mapNodeHandles[cleanName] = processor
		logger.Info("成功初始化子节点", zap.String("node_name", nodeName))
	}

	// 构建工具映射表
	a.buildToolInfos()

	return nil
}

//map_NodeHandles的意义是让节点名称对应到具体的节点实例
//map_ToolInfos的意义是让聚合后的工具名对应具体的工具信息，有多少工具可用，每个工具的详细信息

// buildToolInfos 构建聚合后的工具映射表
func (a *AggregateNode) buildToolInfos() {
	a.mapToolInfos = make(map[string]*AggregatedTool)

	// 遍历每个子节点
	for nodeName, processor := range a.mapNodeHandles {
		// 获取这个子节点的所有工具
		tools := processor.GetTools(&types.RunningContext{})

		// 遍历每个工具
		for _, tool := range tools {
			// 关键：加前缀防重名！
			// 原工具名: get_weather
			// 加前缀后: 天气节点_get_weather
			aggregatedName := nodeName + "_" + tool.ToolName

			// 存到 map_ToolInfos
			a.mapToolInfos[aggregatedName] = &AggregatedTool{
				OriginalName: tool.ToolName, // 原名
				NodeName:     nodeName,      // 属于哪个节点
				ToolDesc:     tool,          // 工具详情
				Node:         processor,     // 节点实例（能干活的人）
			}
		}
	}
}

// GetTools 获取聚合后的工具列表
func (a *AggregateNode) GetTools(rc *types.RunningContext) []*types.ToolDesc {
	a.mutex.RLock() // 读锁，多人可以同时读
	defer a.mutex.RUnlock()

	var toolList []*types.ToolDesc

	// 遍历 map_ToolInfos，返回所有工具
	for aggregatedName, aggTool := range a.mapToolInfos {
		toolList = append(toolList, &types.ToolDesc{
			ToolName:        aggregatedName, // "天气节点_get_weather"
			ToolDesc:        aggTool.ToolDesc.ToolDesc,
			ToolInputSchema: aggTool.ToolDesc.ToolInputSchema,
		})
	}

	logger.Info("聚合节点获取工具列表", zap.Int("工具数量", len(toolList)))
	return toolList
}

// Process 处理工具调用，路由到对应子节点
func (a *AggregateNode) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp *mcp.CallToolResult) (*mcp.CallToolResult, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 查找对应的聚合工具
	aggTool, ok := a.mapToolInfos[userCmd]
	if !ok {
		return nil, fmt.Errorf("tool_not_found: %s", userCmd)
	}

	// 2. 调用子节点处理
	// 关键：传入 OriginalName (原工具名)，不是带前缀的！
	return aggTool.Node.Process(rc, aggTool.OriginalName, userParamMap, lastStepResp)
}

// GetNodeInfo 获取节点信息
func (a *AggregateNode) GetNodeInfo() *types.NodeInfo {
	return a.NodeInfo
}

// GetCurrentConfig 获取当前配置
func (a *AggregateNode) GetCurrentConfig() string {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.currentConfig
}

// ReloadFromDatabase 从数据库重新加载配置并热更新
// 这是对外暴露的热更新入口，封装了数据库读取和配置对比逻辑
func (a *AggregateNode) ReloadFromDatabase() error {
	nodeID := a.NodeInfo.NodeID
	logger.Info("从数据库重新加载聚合节点配置", zap.Int32("node_id", nodeID))

	// 1. 从数据库读取最新配置
	nodeModel := &models.AeMcpTaskNode{}
	err, nodes := nodeModel.GetChianNodes([]int32{nodeID})
	if err != nil || len(nodes) == 0 {
		return fmt.Errorf("get_node_config_from_db_failed: %w", err)
	}

	newConfig := nodes[0].NodeConfig

	// 2. 检查配置是否变化
	a.mutex.RLock()
	oldConfig := a.currentConfig
	a.mutex.RUnlock()

	if newConfig == oldConfig {
		logger.Info("配置未变化，跳过刷新",
			zap.Int32("node_id", nodeID),
			zap.String("config", newConfig))
		return nil
	}

	logger.Info("检测到配置变化，开始刷新",
		zap.Int32("node_id", nodeID),
		zap.String("old_config", oldConfig),
		zap.String("new_config", newConfig))

	// 3. 刷新配置
	return a.RefreshConfig(newConfig)
}

// RefreshConfig 热更新配置
// 注意：此方法会获取写锁，因为需要修改节点的内部状态
func (a *AggregateNode) RefreshConfig(newConfig string) error {
	logger.Info("开始热更新聚合节点配置", zap.Int32("node_id", a.NodeInfo.NodeID))

	// 解析新配置（在加锁前先验证，减少锁持有时间）
	newSubNodeNames, err := ParseAggregateConfig(newConfig)
	if err != nil {
		return fmt.Errorf("parse_new_config_failed: %w", err)
	}

	// 先更新配置字段（这些字段在 initSubNodes 外部，需要单独保护）
	a.mutex.Lock()
	a.subNodeNames = newSubNodeNames
	a.initConfig.NodeModel.NodeConfig = newConfig
	a.currentConfig = newConfig
	a.mutex.Unlock()

	// 重新初始化子节点（initSubNodes 内部会再次获取写锁）
	return a.initSubNodes(a.initConfig)
}
