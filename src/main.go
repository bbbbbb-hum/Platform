package main

import (
	"AgentEarth_AgentPlatform/src/boot"
	"AgentEarth_AgentPlatform/src/config"
	helperConfig "AgentEarth_AgentPlatform/src/helpers/config"
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/middleware"
	"AgentEarth_AgentPlatform/src/servers"
	"AgentEarth_AgentPlatform/src/servers/pools"
	"flag"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func init() {
	// 加载 config 目录下的配置信息
	config.Initialize()
}

func main() {

	// 配置初始化，依赖命令行 --env 参数
	var env string
	flag.StringVar(&env, "env", "", "加载 .env 文件，如 --env=testing 加载的是 .env_testing 文件")
	flag.Parse()
	helperConfig.InitConfig(env)

	// 初始化 Logger
	boot.SetupLogger()
	// 初始化 DB
	boot.SetupDB()

	// 初始化 MCP 服务映射表
	//if err := server.InitializeMcpServices(); err != nil {
	if err := servers.Initialize(); err != nil {
		logger.Error("初始化MCP服务失败", zap.Error(err))
		return
	}

	// 初始化 Prometheus Metrics
	middleware.InitMetrics()
	logger.Info("Prometheus metrics 初始化完成")

	// 启动连接池维护协程（按需创建服务/实例，所以全局维护线程可以提前启动）
	pools.GetConnectPool().StartMaintainer(60 * time.Second)

	// 初始化 mcp 服务
	//test1Server := server.NewServer()
	sseHandler := mcp.NewSSEHandler(func(r *http.Request) *mcp.Server {
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		var serverID string

		// 检查路径格式：/mcp-server/{server_id}/sse
		if len(pathParts) >= 3 && pathParts[0] == "mcp-server" && pathParts[2] == "sse" {
			serverID = pathParts[1]
		}

		mcpServer, ok := servers.McpServicesMap[serverID]
		if !ok {
			return nil
		}
		return mcpServer.GetServer()
	})
	// 增加权限校验
	authMiddleware := middleware.NewAuth()
	// 设置路由
	mux := http.NewServeMux()

	// Prometheus metrics 端点（不需要认证）
	mux.Handle("/metrics", promhttp.Handler())

	// pprof 性能分析端点（不需要认证，生产环境建议关闭或加认证）
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// 修改路由格式：/mcp-server/{server_id}/sse
	// 使用 Prometheus 中间件包装（先包装 SSE Handler，再添加认证，最后添加指标收集）
	mcpHandler := middleware.PrometheusMiddleware(http.HandlerFunc(authMiddleware.Auth(sseHandler.ServeHTTP)))
	mux.Handle("/mcp-server/", mcpHandler)

	// 启动 HTTP 服务
	host := helperConfig.GetString("server.host")
	port := helperConfig.GetString("server.port")
	addr := host + ":" + port
	logger.Info("MCP服务启动...", zap.String("addr", addr))
	err := http.ListenAndServe(addr, mux)
	if err != nil {
		logger.Error("启动MCP服务失败", zap.Error(err))
		return
	}
}
