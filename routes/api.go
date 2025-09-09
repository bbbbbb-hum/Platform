package routes

import (
	"AgentEarth_AgentPlatform/app/controller"
	"github.com/gin-gonic/gin"
)

func RegisterAPIRoutes(r *gin.Engine) {
	//v1版本
	sc := new(controller.SSEController)
	r.GET("/sse", sc.Handle)
	r.GET("/health", sc.Handle)

}
