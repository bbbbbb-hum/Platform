package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics 持有所有 Prometheus 指标
type Metrics struct {
	// HTTP 请求相关指标
	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec
	httpRequestSize     *prometheus.HistogramVec
	httpResponseSize    *prometheus.HistogramVec

	// MCP 相关指标
	mcpConnectionsActive prometheus.Gauge
	mcpRequestsTotal     *prometheus.CounterVec
	mcpRequestDuration   *prometheus.HistogramVec

	// 任务处理相关指标
	taskProcessingTotal    *prometheus.CounterVec
	taskProcessingDuration *prometheus.HistogramVec
	taskQueueSize          prometheus.Gauge
}

var metricsInstance *Metrics

// InitMetrics 初始化 Prometheus 指标
func InitMetrics() *Metrics {
	if metricsInstance != nil {
		return metricsInstance
	}

	metricsInstance = &Metrics{
		// HTTP 请求总数
		httpRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),

		// HTTP 请求延迟
		httpRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request latency in seconds",
				Buckets: prometheus.DefBuckets, // 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10
			},
			[]string{"method", "path", "status"},
		),

		// HTTP 请求体大小
		httpRequestSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_size_bytes",
				Help:    "HTTP request size in bytes",
				Buckets: []float64{100, 1000, 10000, 100000, 1000000},
			},
			[]string{"method", "path"},
		),

		// HTTP 响应体大小
		httpResponseSize: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_response_size_bytes",
				Help:    "HTTP response size in bytes",
				Buckets: []float64{100, 1000, 10000, 100000, 1000000},
			},
			[]string{"method", "path", "status"},
		),

		// MCP 活跃连接数
		mcpConnectionsActive: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "mcp_connections_active",
				Help: "Number of active MCP connections",
			},
		),

		// MCP 请求总数
		mcpRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "mcp_requests_total",
				Help: "Total number of MCP requests",
			},
			[]string{"server_id", "method", "status"},
		),

		// MCP 请求延迟
		mcpRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "mcp_request_duration_seconds",
				Help:    "MCP request latency in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"server_id", "method"},
		),

		// 任务处理总数
		taskProcessingTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "task_processing_total",
				Help: "Total number of tasks processed",
			},
			[]string{"task_type", "status"},
		),

		// 任务处理延迟
		taskProcessingDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "task_processing_duration_seconds",
				Help:    "Task processing latency in seconds",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120, 300},
			},
			[]string{"task_type"},
		),

		// 任务队列大小
		taskQueueSize: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "task_queue_size",
				Help: "Current size of task queue",
			},
		),
	}

	return metricsInstance
}

// GetMetrics 获取 Metrics 实例
func GetMetrics() *Metrics {
	if metricsInstance == nil {
		return InitMetrics()
	}
	return metricsInstance
}

// responseWriter 包装 http.ResponseWriter 以捕获状态码和响应大小
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// PrometheusMiddleware 是一个 HTTP 中间件，用于记录 HTTP 请求指标
func PrometheusMiddleware(next http.Handler) http.Handler {
	metrics := GetMetrics()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 包装 ResponseWriter
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// 记录请求大小
		requestSize := float64(r.ContentLength)
		if requestSize > 0 {
			metrics.httpRequestSize.WithLabelValues(r.Method, r.URL.Path).Observe(requestSize)
		}

		// 处理请求
		next.ServeHTTP(rw, r)

		// 记录指标
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(rw.statusCode)

		metrics.httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, status).Inc()
		metrics.httpRequestDuration.WithLabelValues(r.Method, r.URL.Path, status).Observe(duration)
		metrics.httpResponseSize.WithLabelValues(r.Method, r.URL.Path, status).Observe(float64(rw.bytesWritten))
	})
}

// RecordMCPRequest 记录 MCP 请求指标
func (m *Metrics) RecordMCPRequest(serverID, method, status string, duration time.Duration) {
	m.mcpRequestsTotal.WithLabelValues(serverID, method, status).Inc()
	m.mcpRequestDuration.WithLabelValues(serverID, method).Observe(duration.Seconds())
}

// SetMCPConnectionsActive 设置活跃 MCP 连接数
func (m *Metrics) SetMCPConnectionsActive(count float64) {
	m.mcpConnectionsActive.Set(count)
}

// RecordTaskProcessing 记录任务处理指标
func (m *Metrics) RecordTaskProcessing(taskType, status string, duration time.Duration) {
	m.taskProcessingTotal.WithLabelValues(taskType, status).Inc()
	m.taskProcessingDuration.WithLabelValues(taskType).Observe(duration.Seconds())
}

// SetTaskQueueSize 设置任务队列大小
func (m *Metrics) SetTaskQueueSize(size float64) {
	m.taskQueueSize.Set(size)
}
