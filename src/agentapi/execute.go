package agentapi

import (
	"net/http"
	"strings"
)

type executeRequest struct {
	ToolName  string                 `json:"tool_name"`
	Arguments map[string]interface{} `json:"arguments"`
	SessionID string                 `json:"session_id,omitempty"`
}

func HandleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req executeRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	req.ToolName = strings.TrimSpace(req.ToolName)
	if req.ToolName == "" {
		writeError(w, http.StatusBadRequest, "tool_name is required")
		return
	}

	if req.Arguments == nil {
		req.Arguments = map[string]interface{}{}
	}

	_, server, err := getSkillServer()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 仅允许执行当前服务已注册工具，避免调用不存在或未授权的工具名。
	if findToolByName(server.GetToolDescList(), req.ToolName) == nil {
		writeError(w, http.StatusNotFound, "tool not found")
		return
	}

	rc, err := buildRunningContext(r, server)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result, err := server.ChainInstance.Process(rc, req.ToolName, req.Arguments)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if result == nil {
		writeError(w, http.StatusInternalServerError, "empty tool result")
		return
	}

	writeJSON(w, http.StatusOK, result)
}
