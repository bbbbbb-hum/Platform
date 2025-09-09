package bootstrap

import (
	"AgentEarth_AgentPlatform/routes"
	"github.com/gin-gonic/gin"
)

func SetupRoute(router *gin.Engine) {
	// 注册全局中间件
	//registerGlobalMiddleWare(router)
	//  注册 API 路由
	routes.RegisterAPIRoutes(router)
	//  配置 404 路由
	//setup404Handler(router)
}
