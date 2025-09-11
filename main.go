package main

import (
	"AgentEarth_AgentPlatform/boot"
	"AgentEarth_AgentPlatform/config"
	"AgentEarth_AgentPlatform/server"
	"flag"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	helperConfig "github.com/wcs1010270451/helpers/config"
	"log"
	"net/http"
	"strings"
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

	// 初始化 DB
	boot.SetupDB()

	// 初始化 MCP 服务映射表
	if err := server.InitializeMcpServices(); err != nil {
		log.Fatalf("初始化MCP服务失败: %v", err)
	}

	// 初始化 mcp 服务
	//test1Server := server.NewServer()
	handler := mcp.NewSSEHandler(func(r *http.Request) *mcp.Server {
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
	log.Fatal(http.ListenAndServe("0.0.0.0:9001", handler))
}
