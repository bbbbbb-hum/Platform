package main

import (
	"AgentEarth_AgentPlatform/boot"
	"AgentEarth_AgentPlatform/config"
	"AgentEarth_AgentPlatform/server"
	"flag"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	helperConfig "github.com/wcs1010270451/helpers/config"
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
	if err := server.InitializeMcpServices(); err != nil {
		logger.Error("初始化MCP服务失败", zap.Error(err))
		return
	}

	// 初始化 mcp 服务
	//test1Server := server.NewServer()
	sseHandler := mcp.NewSSEHandler(func(r *http.Request) *mcp.Server {
		pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		var serverID string

		// 检查路径格式：/mcp-server/{server_id}/sse
		if len(pathParts) >= 3 && pathParts[0] == "mcp-server" && pathParts[2] == "sse" {
			serverID = pathParts[1]
		}

		mcpServer, ok := server.McpServicesMap[serverID]
		if !ok {
			return nil
		}
		return mcpServer.GetServer()
	})
	// 设置路由
	mux := http.NewServeMux()
	// 修改路由格式：/mcp-server/{server_id}/sse
	mux.Handle("/mcp-server/", sseHandler)

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
