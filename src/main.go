package main

import (
	"AgentEarth_AgentPlatform/src/boot"
	"AgentEarth_AgentPlatform/src/config"
	helperConfig "AgentEarth_AgentPlatform/src/helpers/config"
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/middleware"
	"AgentEarth_AgentPlatform/src/servers"
	"AgentEarth_AgentPlatform/src/servers/pools"
	"context"
	"flag"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func init() {
	// 加载 config 目录下的配置信息
	config.Initialize()
}

// healthHandler 健康检查处理器 - 用于 Kubernetes liveness probe
// 检查服务是否存活，不检查依赖服务
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
}

// readyHandler 就绪检查处理器 - 用于 Kubernetes readiness probe
// 检查服务是否准备好接收流量（数据库连接、MCP服务等）
func readyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// 检查数据库连接
	db := boot.GetDB()
	if db == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unavailable","reason":"database not initialized","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
		return
	}

	sqlDB, err := db.DB()
	if err != nil || sqlDB.Ping() != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unavailable","reason":"database connection failed","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
		return
	}

	// 检查 MCP 服务是否已初始化
	if len(servers.McpServicesMap) == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unavailable","reason":"mcp services not initialized","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
		return
	}

	// 所有检查通过
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready","mcp_services":` + strconv.Itoa(len(servers.McpServicesMap)) + `,"timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
}

func main() {

	// 配置初始化，依赖命令行 --configfile 参数
	var configfile string
	flag.StringVar(&configfile, "configfile", "", "加载配置文件，如 --configfile=agent_plat_form.yml 加载的是 ./agent_plat_form.yml 文件")
	flag.Parse()
	helperConfig.InitConfig(configfile)

	// 初始化 Logger
	boot.SetupLogger()
	// 初始化 DB
	boot.SetupDB()
	// 初始化 Redis
	boot.SetupRedis()

	// 初始化 MCP 服务映射表
	//if err := server.InitializeMcpServices(); err != nil {
	if err := servers.Initialize(); err != nil {
		logger.Error("初始化MCP服务失败", zap.Error(err))
		return
	}

	// 初始化 Prometheus Metrics
	middleware.InitMetrics()
	logger.Info("Prometheus metrics 初始化完成")
	env := helperConfig.GetString("SERVER_ENV")
	// 启动连接池维护协程（按需创建服务/实例，所以全局维护线程可以提前启动）
	pools.GetConnectPool().StartMaintainer(60 * time.Second)

	// 启动请求日志批量插入协程（每分钟同步一次）
	pools.GetRequestLogsPool().Start(60 * time.Second)

	// 初始化 mcp 服务
	httpStreamableHandler := mcp.NewStreamableHTTPHandler(func(request *http.Request) *mcp.Server {
		path := request.URL.Path
		pathParts := strings.Split(strings.Trim(path, "/"), "/")

		var serverID string
		// 新路径定义：/mcp-server/{server_id}
		if len(pathParts) >= 2 && pathParts[0] == "mcp-server" {
			serverID = pathParts[1]
		}

		mcpServer, ok := servers.McpServicesMap[serverID]
		if !ok || mcpServer == nil || mcpServer.GetServer() == nil {
			return nil
		}
		return mcpServer.GetServer()
	}, nil)
	// 增加权限校验
	authMiddleware := middleware.NewAuth()
	// 设置路由
	mux := http.NewServeMux()

	// Prometheus metrics 端点（不需要认证）
	mux.Handle("/metrics", promhttp.Handler())

	// 健康检查端点（不需要认证）
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/ready", readyHandler)

	// 生产环境不开启
	if env != "prod" {
		// pprof 性能分析端点（不需要认证，生产环境建议关闭或加认证）
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

		// 初始化单个服务接口
		mux.HandleFunc("/debug/mcp-server/init/{id}", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			pathId := r.URL.Path[len("/debug/mcp-server/init/"):]
			id, err := strconv.ParseInt(pathId, 10, 32)
			if err != nil {
				w.Write([]byte(err.Error()))
				return
			}
			err = servers.InitializeByServiceId(int32(id))
			if err != nil {
				logger.Error("初始化MCP服务失败", zap.Error(err))
				w.Write([]byte(err.Error()))
				return
			}
			w.Write([]byte("MCP服务初始化完成"))
		})
	}

	// 使用 Prometheus 中间件包装（先包装 Handler，再添加认证，最后添加指标收集）
	mcpHandler := middleware.PrometheusMiddleware(authMiddleware.Auth(httpStreamableHandler.ServeHTTP))
	mux.Handle("/mcp-server/", mcpHandler)

	// 创建 HTTP 服务器
	host := helperConfig.GetString("server.host")
	port := helperConfig.GetString("server.port")
	addr := host + ":" + port

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// 启动信号监听，处理优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// 在独立 goroutine 中启动 HTTP 服务
	go func() {
		logger.Info("MCP服务启动...", zap.String("addr", addr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("启动MCP服务失败", zap.Error(err))
			os.Exit(1)
		}
	}()

	// 等待终止信号
	sig := <-sigChan
	logger.Info("收到终止信号，开始优雅关闭...", zap.String("signal", sig.String()))

	// 创建关闭超时上下文
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer shutdownCancel()

	// 关闭所有 MCP 连接池（会清理所有 npx/uvx 子进程）
	logger.Info("正在关闭连接池，清理子进程...")
	pools.GetConnectPool().Close()
	pools.GetRequestLogsPool().Stop()
	logger.Info("连接池关闭完成")

	// 优雅关闭 HTTP 服务器
	logger.Info("正在关闭HTTP服务器...")
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP服务器关闭失败", zap.Error(err))
	} else {
		logger.Info("HTTP服务器关闭完成")
	}
	logger.Info("服务已安全退出")
}
