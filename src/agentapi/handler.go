package agentapi

import (
	"AgentEarth_AgentPlatform/src/middleware"
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, auth *middleware.AuthMiddleware) {
	if mux == nil || auth == nil {
		return
	}

	mux.HandleFunc("/agent-api/v1/tool/recommend", auth.Auth(HandleRecommend))
	mux.HandleFunc("/agent-api/v1/tool/execute", auth.Auth(HandleExecute))
}
