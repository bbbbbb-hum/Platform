package main

import (
	"AgentEarth_AgentPlatform/bootstrap"
	"AgentEarth_AgentPlatform/config"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	helperConfig "github.com/wcs1010270451/helpers/config"
)

func init() {
	// 加载 config 目录下的配置信息
	config.Initialize()
}

func main() {
	// 配置初始化，依赖命令行 --env 参数
	var env string
	flag.StringVar(&env, "env", "", "加载 .env 文件，如 --env=testing 加载的是 .env.testing 文件")
	flag.Parse()
	helperConfig.InitConfig(env)

	// 初始化 Logger
	bootstrap.SetupLogger()
	gin.SetMode(gin.ReleaseMode)

	// 初始化 Gin 实例
	r := gin.New()

	//初始化路由绑定
	bootstrap.SetupRoute(r)
	
	// 运行 PROXY 服务
	var proxyPort = ":" + helperConfig.Get("app.port")
	fmt.Println(helperConfig.Get("app.env") + " 服务启动,端口" + proxyPort)
	err := r.Run(proxyPort)
	if err != nil {
		fmt.Println(err.Error())
	}
}
