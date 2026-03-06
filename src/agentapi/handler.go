package agentapi

import (
	"AgentEarth_AgentPlatform/src/middleware"
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, auth *middleware.AuthMiddleware) {
	if mux == nil || auth == nil {
		return
	}

	// Skill 接口为机器调用入口，统一走 API Key 鉴权中间件。
	mux.HandleFunc("/agent-api/v1/tool/recommend", auth.Auth(HandleRecommend))
	mux.HandleFunc("/agent-api/v1/tool/execute", auth.Auth(HandleExecute))
}
