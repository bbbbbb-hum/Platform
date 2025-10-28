# Grafana 配置指南

## 第一步：添加 Prometheus 数据源

### 1. 登录 Grafana
- 访问：`http://your-server:3000`
- 用户名：`admin`
- 密码：`admin`
- 首次登录会要求修改密码

### 2. 添加数据源
1. 点击左侧菜单 **⚙️ Configuration** → **Data Sources**
2. 点击 **Add data source**
3. 选择 **Prometheus**

### 3. 配置 Prometheus 数据源
- **Name**: `Prometheus-Test`
- **URL**: 
  - 如果 Grafana 和 Prometheus 都是 Docker：`http://prometheus容器IP:9090`
  - 或使用：`http://host.docker.internal:9090`
  - 安装脚本会显示正确的 URL
- **Access**: `Server (default)`
- 其他保持默认

### 4. 测试并保存
- 点击页面底部 **Save & Test**
- 应该看到绿色提示：✅ Data source is working

---

## 第二步：创建 Dashboard

### 方式 A：导入现成的 Dashboard（推荐）

#### 1. 导入 Go 应用监控 Dashboard

1. 点击左侧菜单 **➕** → **Import**
2. 输入 Dashboard ID：`10826`（Go Metrics）
3. 点击 **Load**
4. 选择数据源：`Prometheus-Test`
5. 点击 **Import**

#### 2. 推荐的 Dashboard ID
- **10826**: Go Metrics - Go 应用监控
- **6417**: Prometheus 2.0 Stats
- **11074**: Node Exporter Full（如果监控服务器）
- **13639**: Kubernetes Pods（如果用 K8s）

### 方式 B：创建自定义 Dashboard

#### 1. 创建新 Dashboard
1. 点击左侧菜单 **➕** → **Dashboard**
2. 点击 **Add new panel**

#### 2. 配置第一个面板 - QPS（每秒请求数）

**指标查询：**
```promql
rate(http_requests_total[1m])
```

**配置：**
- **Title**: HTTP QPS
- **Legend**: `{{method}} {{path}}`
- **Type**: Time series (折线图)
- **Unit**: reqps (requests per second)

#### 3. 添加第二个面板 - 响应时间 P95

**指标查询：**
```promql
histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, path))
```

**配置：**
- **Title**: HTTP P95 延迟
- **Legend**: `{{path}}`
- **Unit**: s (seconds)

#### 4. 添加第三个面板 - 内存使用

**指标查询：**
```promql
go_memstats_alloc_bytes / 1024 / 1024
```

**配置：**
- **Title**: 内存使用
- **Unit**: MB
- **Type**: Stat（数字面板）

#### 5. 添加第四个面板 - Goroutine 数量

**指标查询：**
```promql
go_goroutines
```

**配置：**
- **Title**: Goroutines
- **Unit**: short
- **Type**: Time series

#### 6. 添加第五个面板 - 错误率

**指标查询：**
```promql
sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) * 100
```

**配置：**
- **Title**: 错误率
- **Unit**: percent (0-100)
- **Type**: Gauge（仪表盘）
- **Thresholds**: 
  - Green: 0-1%
  - Yellow: 1-5%
  - Red: > 5%

---

## 第三步：常用 PromQL 查询

### HTTP 监控

```promql
# HTTP 请求总数
http_requests_total

# 每秒请求数（QPS）
rate(http_requests_total[1m])

# 按状态码分组的 QPS
sum(rate(http_requests_total[1m])) by (status)

# 按路径分组的 QPS
sum(rate(http_requests_total[1m])) by (path)

# 错误请求数
sum(rate(http_requests_total{status=~"5.."}[5m]))

# 错误率（百分比）
sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) * 100
```

### 延迟监控

```promql
# P50 延迟
histogram_quantile(0.50, rate(http_request_duration_seconds_bucket[5m]))

# P95 延迟
histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))

# P99 延迟
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))

# 平均延迟
rate(http_request_duration_seconds_sum[5m]) / rate(http_request_duration_seconds_count[5m])
```

### Go 运行时监控

```promql
# Goroutine 数量
go_goroutines

# 内存分配（MB）
go_memstats_alloc_bytes / 1024 / 1024

# 堆内存（MB）
go_memstats_heap_alloc_bytes / 1024 / 1024

# GC 次数
rate(go_gc_duration_seconds_count[5m])

# CPU 使用率（%）
rate(process_cpu_seconds_total[1m]) * 100
```

### MCP 服务监控

```promql
# MCP 请求总数
mcp_requests_total

# MCP 请求速率
rate(mcp_requests_total[1m])

# MCP 活跃连接数
mcp_connections_active

# MCP 错误率
sum(rate(mcp_requests_total{status="error"}[5m])) by (server_id) / sum(rate(mcp_requests_total[5m])) by (server_id)
```

---

## 第四步：配置告警

### 1. 创建告警规则

在 Dashboard 面板中：
1. 编辑面板（点击面板标题 → Edit）
2. 切换到 **Alert** 标签
3. 点击 **Create alert rule from this panel**

### 2. 常用告警示例

#### 高错误率告警
```yaml
条件: 错误率 > 5%
持续时间: 5 分钟
严重程度: Critical
通知渠道: Email / 钉钉 / 企业微信
```

#### 高延迟告警
```yaml
条件: P95 延迟 > 1 秒
持续时间: 5 分钟
严重程度: Warning
```

#### 内存使用告警
```yaml
条件: 内存 > 2GB
持续时间: 10 分钟
严重程度: Warning
```

---

## 第五步：Dashboard 管理

### 保存 Dashboard
1. 点击右上角 **💾 Save dashboard**
2. 输入名称：`AgentPlatform 监控`
3. 点击 **Save**

### 分享 Dashboard
1. 点击右上角 **🔗 Share**
2. 选择 **Link** 标签
3. 复制链接发送给团队

### 导出 Dashboard
1. 点击右上角 **⚙️** → **JSON Model**
2. 复制 JSON
3. 保存为文件（可以版本控制）

### 设置为首页
1. 点击右上角 **⭐ Star**
2. **⚙️ Configuration** → **Preferences**
3. **Home Dashboard**: 选择你的 Dashboard

---

## 第六步：配置告警通知渠道

### 邮件通知

1. **⚙️ Configuration** → **Notification channels**
2. **Add channel**
3. 选择 **Email**
4. 填写邮箱地址
5. 测试并保存

### 钉钉机器人

1. 在钉钉群创建自定义机器人
2. 获取 Webhook URL
3. Grafana 中添加 **Webhook** 类型通知渠道
4. 填写 Webhook URL

### 企业微信

类似钉钉，使用企业微信机器人的 Webhook

---

## 第七步：Dashboard 模板示例

### 完整的服务监控 Dashboard 布局

```
┌─────────────────────────────────────────────────────────┐
│  AgentPlatform 服务监控 - 测试环境                       │
├──────────────┬──────────────┬──────────────┬───────────┤
│ QPS          │ 平均延迟      │ 错误率        │ 在线状态  │
│ 125 req/s    │ 45 ms        │ 0.05 %       │ UP       │
├──────────────┴──────────────┴──────────────┴───────────┤
│  [HTTP QPS 趋势图 - 折线图]                              │
├─────────────────────────────────────────────────────────┤
│  [P50/P95/P99 延迟对比 - 多折线图]                       │
├────────────────────────┬────────────────────────────────┤
│ [内存使用趋势 - 折线图]  │ [Goroutine 数量 - 折线图]      │
├────────────────────────┴────────────────────────────────┤
│  [请求状态码分布 - 柱状图]                               │
├─────────────────────────────────────────────────────────┤
│  [Top 10 慢接口 - 表格]                                  │
└─────────────────────────────────────────────────────────┘
```

---

## 常见问题

### 1. 数据源连接失败
- 检查 Prometheus 是否运行
- 检查 URL 是否正确
- 尝试使用容器 IP 而不是 localhost

### 2. 没有数据显示
- 确认 AgentPlatform 服务正在运行
- 确认 Prometheus 已经抓取到数据
- 检查时间范围是否合适（右上角）

### 3. Dashboard 导入失败
- 确认已添加 Prometheus 数据源
- 尝试手动创建面板
- 检查 Grafana 版本兼容性

### 4. 告警不触发
- 检查告警规则配置
- 确认通知渠道已配置并测试
- 查看告警历史记录

---

## 有用的快捷键

- `d + k`: 打开/关闭 kiosk 模式（全屏）
- `t + z`: 放大时间范围
- `t + ←/→`: 时间范围前后移动
- `Ctrl + S`: 保存 Dashboard
- `?`: 显示所有快捷键

---

## 下一步

1. ✅ 熟悉 Grafana 界面
2. ✅ 创建你的第一个 Dashboard
3. 📊 添加更多有用的面板
4. 🔔 配置告警规则
5. 📈 导入社区 Dashboard
6. 🎨 美化和优化 Dashboard

