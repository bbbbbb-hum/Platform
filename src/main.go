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
	"strings"
	"time"

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
	// 修改路由格式：/mcp-server/{server_id}/sse
	mux.Handle("/mcp-server/", authMiddleware.Auth(sseHandler.ServeHTTP))

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
