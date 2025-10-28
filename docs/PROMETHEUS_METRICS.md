# Prometheus Metrics 使用指南

## 概述

本项目已集成 Prometheus 监控，可以通过 `/metrics` 端点获取服务运行指标。

## 访问 Metrics 端点

启动服务后，可以通过以下 URL 访问 metrics：

```
http://your-server:9001/metrics
```

**示例：**
```bash
# 查看所有指标
curl http://localhost:9001/metrics

# 查看特定指标
curl http://localhost:9001/metrics | grep http_requests_total
```

## 内置指标

### 1. HTTP 请求指标

#### `http_requests_total`
- **类型**: Counter
- **描述**: HTTP 请求总数
- **标签**:
  - `method`: HTTP 方法 (GET, POST, etc.)
  - `path`: 请求路径
  - `status`: HTTP 状态码

**示例：**
```
http_requests_total{method="POST",path="/mcp-server/service1/sse",status="200"} 1523
```

#### `http_request_duration_seconds`
- **类型**: Histogram
- **描述**: HTTP 请求延迟（秒）
- **标签**: method, path, status
- **分桶**: 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10

#### `http_request_size_bytes`
- **类型**: Histogram
- **描述**: HTTP 请求体大小（字节）
- **标签**: method, path

#### `http_response_size_bytes`
- **类型**: Histogram
- **描述**: HTTP 响应体大小（字节）
- **标签**: method, path, status

### 2. MCP 服务指标

#### `mcp_connections_active`
- **类型**: Gauge
- **描述**: 当前活跃的 MCP 连接数

#### `mcp_requests_total`
- **类型**: Counter
- **描述**: MCP 请求总数
- **标签**:
  - `server_id`: MCP 服务 ID
  - `method`: 调用方法
  - `status`: 状态 (success/error)

#### `mcp_request_duration_seconds`
- **类型**: Histogram
- **描述**: MCP 请求延迟（秒）
- **标签**: server_id, method

### 3. 任务处理指标

#### `task_processing_total`
- **类型**: Counter
- **描述**: 任务处理总数
- **标签**:
  - `task_type`: 任务类型
  - `status`: 状态 (success/error)

#### `task_processing_duration_seconds`
- **类型**: Histogram
- **描述**: 任务处理延迟（秒）
- **标签**: task_type
- **分桶**: 0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300

#### `task_queue_size`
- **类型**: Gauge
- **描述**: 当前任务队列大小

### 4. Go 运行时指标（自动收集）

- `go_goroutines`: Goroutine 数量
- `go_threads`: 系统线程数
- `go_memstats_alloc_bytes`: 已分配内存
- `go_memstats_heap_alloc_bytes`: 堆内存分配
- `go_gc_duration_seconds`: GC 耗时
- `process_cpu_seconds_total`: CPU 使用时间
- `process_resident_memory_bytes`: 常驻内存大小

## 在代码中使用指标

### HTTP 请求（自动记录）

HTTP 请求指标通过 `PrometheusMiddleware` 自动记录，无需手动调用。

### 记录 MCP 请求

```go
import (
    "AgentEarth_AgentPlatform/src/middleware"
    "time"
)

func handleMCPRequest(serverID, method string) {
    start := time.Now()
    
    // 处理请求...
    status := "success" // 或 "error"
    
    // 记录指标
    metrics := middleware.GetMetrics()
    metrics.RecordMCPRequest(serverID, method, status, time.Since(start))
}
```

### 记录任务处理

```go
func processTask(taskType string) {
    start := time.Now()
    
    // 处理任务...
    status := "success" // 或 "error"
    
    // 记录指标
    metrics := middleware.GetMetrics()
    metrics.RecordTaskProcessing(taskType, status, time.Since(start))
}
```

### 设置活跃连接数

```go
func updateConnections(count int) {
    metrics := middleware.GetMetrics()
    metrics.SetMCPConnectionsActive(float64(count))
}
```

### 设置队列大小

```go
func updateQueueSize(size int) {
    metrics := middleware.GetMetrics()
    metrics.SetTaskQueueSize(float64(size))
}
```

## PromQL 查询示例

### HTTP 请求速率（QPS）

```promql
# 每秒请求数
rate(http_requests_total[1m])

# 按路径分组的 QPS
sum(rate(http_requests_total[1m])) by (path)

# 错误率
sum(rate(http_requests_total{status=~"5.."}[1m])) / sum(rate(http_requests_total[1m]))
```

### 延迟分析

```promql
# P95 延迟
histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, path))

# P99 延迟
histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, path))

# 平均延迟
rate(http_request_duration_seconds_sum[5m]) / rate(http_request_duration_seconds_count[5m])
```

### 资源使用

```promql
# Goroutine 数量
go_goroutines

# 内存使用（MB）
go_memstats_alloc_bytes / 1024 / 1024

# CPU 使用率
rate(process_cpu_seconds_total[1m]) * 100
```

### MCP 服务监控

```promql
# MCP 请求速率
rate(mcp_requests_total[1m])

# MCP 错误率
sum(rate(mcp_requests_total{status="error"}[1m])) by (server_id) / sum(rate(mcp_requests_total[1m])) by (server_id)

# 活跃连接数
mcp_connections_active
```

## 下一步：配置 Prometheus 服务器

创建 Prometheus 配置文件 `prometheus.yml`：

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'agent-platform'
    static_configs:
      - targets: ['your-server:9001']
    metrics_path: '/metrics'
```

启动 Prometheus：

```bash
docker run -d \
  --name prometheus \
  -p 9090:9090 \
  -v /path/to/prometheus.yml:/etc/prometheus/prometheus.yml \
  prom/prometheus
```

访问 Prometheus UI：`http://localhost:9090`

## Grafana 可视化

推荐导入以下 Grafana Dashboard：

1. **Go Metrics**: Dashboard ID `10826`
2. **Prometheus Stats**: Dashboard ID `2`
3. 或创建自定义 Dashboard

常用面板：
- HTTP QPS 趋势
- 响应时间分布
- 错误率监控
- 资源使用情况
- MCP 连接状态

## 告警规则示例

创建 `alert.rules.yml`：

```yaml
groups:
  - name: agent_platform
    rules:
      # 高错误率告警
      - alert: HighErrorRate
        expr: sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value | humanizePercentage }}"

      # 高延迟告警
      - alert: HighLatency
        expr: histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le)) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency detected"
          description: "P95 latency is {{ $value }}s"

      # 内存使用告警
      - alert: HighMemoryUsage
        expr: go_memstats_alloc_bytes / 1024 / 1024 / 1024 > 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage"
          description: "Memory usage is {{ $value }}GB"
```

## 最佳实践

1. **合理设置采集间隔**：通常 15-60 秒
2. **使用标签过滤**：避免高基数标签（如用户 ID）
3. **定期检查指标**：确保关键指标正常记录
4. **设置告警**：针对关键业务指标设置告警
5. **数据保留**：配置合理的数据保留策略

