package servers

import (
	helperConfig "AgentEarth_AgentPlatform/src/helpers/config"
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/models"
	"AgentEarth_AgentPlatform/src/servers/pools"
	"AgentEarth_AgentPlatform/src/servers/task_nodes"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

type runtimeConnectReq struct {
	NodeID  int64 `json:"node_id"`
	Timeout int   `json:"timeout"`
}

type runtimeCallReq struct {
	NodeID    int64           `json:"node_id"`
	ToolName  string          `json:"tool_name"`
	Arguments json.RawMessage `json:"arguments"`
	Timeout   int             `json:"timeout"`
}

type runtimeDisconnectReq struct {
	NodeID int64 `json:"node_id"`
}

func RuntimeConnectHandler(w http.ResponseWriter, r *http.Request) {
	if !allowVerificationRequest(r) {
		writeRuntimeJSON(w, http.StatusForbidden, map[string]interface{}{
			"success": false,
			"error":   "verification_forbidden",
		})
		return
	}
	var req runtimeConnectReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid_request"})
		return
	}
	if req.NodeID <= 0 {
		writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "node_id_required"})
		return
	}

	start := time.Now()
	node, cfg, err := loadRuntimeNodeConfig(int32(req.NodeID))
	if err != nil {
		writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	timeoutMS := cfg.TimeoutMS
	if req.Timeout > 0 {
		timeoutMS = req.Timeout * 1000
	}
	if err := pools.GetConnectPool().InitializeNode(node.Id, cfg.Protocol, cfg.URL, timeoutMS); err != nil {
		logger.Error("runtime connect failed", zap.Int64("node_id", req.NodeID), zap.Error(err))
		writeRuntimeJSON(w, http.StatusBadGateway, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	tools := pools.GetConnectPool().GetNodeTools(node.Id)
	toolList := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		if tool == nil {
			continue
		}
		item := map[string]interface{}{
			"name":        tool.Name,
			"description": tool.Description,
		}
		if tool.InputSchema != nil {
			item["inputSchema"] = tool.InputSchema
		}
		toolList = append(toolList, item)
	}

	writeRuntimeJSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"node_id":     req.NodeID,
		"service_url": cfg.URL,
		"tools":       toolList,
		"tools_count": len(toolList),
		"duration_ms": time.Since(start).Milliseconds(),
	})
}

func RuntimeCallHandler(w http.ResponseWriter, r *http.Request) {
	if !allowVerificationRequest(r) {
		writeRuntimeJSON(w, http.StatusForbidden, map[string]interface{}{
			"success": false,
			"error":   "verification_forbidden",
		})
		return
	}
	var req runtimeCallReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid_request"})
		return
	}
	if req.NodeID <= 0 || strings.TrimSpace(req.ToolName) == "" {
		writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "node_id_and_tool_name_required"})
		return
	}

	start := time.Now()
	node, cfg, err := loadRuntimeNodeConfig(int32(req.NodeID))
	if err != nil {
		writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	timeoutMS := cfg.TimeoutMS
	if req.Timeout > 0 {
		timeoutMS = req.Timeout * 1000
	}
	if err := pools.GetConnectPool().InitializeNode(node.Id, cfg.Protocol, cfg.URL, timeoutMS); err != nil {
		logger.Error("runtime call initialize failed", zap.Int64("node_id", req.NodeID), zap.Error(err))
		writeRuntimeJSON(w, http.StatusBadGateway, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	args := make(map[string]interface{})
	if len(req.Arguments) > 0 {
		if err := json.Unmarshal(req.Arguments, &args); err != nil {
			writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid_arguments_json"})
			return
		}
	}

	result, err := pools.GetConnectPool().CallToolByNode(node.Id, strings.TrimSpace(req.ToolName), args)
	if err != nil {
		writeRuntimeJSON(w, http.StatusBadGateway, map[string]interface{}{
			"success":     false,
			"tool_name":   req.ToolName,
			"error":       err.Error(),
			"duration_ms": time.Since(start).Milliseconds(),
		})
		return
	}

	contentAny := interface{}(nil)
	if result != nil && len(result.Content) > 0 {
		if raw, err := json.Marshal(result.Content); err == nil {
			_ = json.Unmarshal(raw, &contentAny)
		}
	}
	isError := false
	if result != nil {
		isError = result.IsError
	}

	writeRuntimeJSON(w, http.StatusOK, map[string]interface{}{
		"success":     true,
		"tool_name":   req.ToolName,
		"is_error":    isError,
		"content":     contentAny,
		"duration_ms": time.Since(start).Milliseconds(),
	})
}

func RuntimeDisconnectHandler(w http.ResponseWriter, r *http.Request) {
	if !allowVerificationRequest(r) {
		writeRuntimeJSON(w, http.StatusForbidden, map[string]interface{}{
			"success": false,
			"error":   "verification_forbidden",
		})
		return
	}
	var req runtimeDisconnectReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid_request"})
		return
	}
	if req.NodeID <= 0 {
		writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "node_id_required"})
		return
	}

	if err := pools.GetConnectPool().RemoveNode(int32(req.NodeID)); err != nil {
		writeRuntimeJSON(w, http.StatusInternalServerError, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	writeRuntimeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"node_id": req.NodeID,
	})
}

func loadRuntimeNodeConfig(nodeID int32) (*models.AeMcpTaskNode, *task_nodes.NodeRuntimeConfig, error) {
	var node models.AeMcpTaskNode
	if err := models.GetDB().Where("id = ?", nodeID).First(&node).Error; err != nil {
		return nil, nil, err
	}
	cfg, err := task_nodes.ParseNodeRuntimeConfig(node.NodeConfig)
	if err != nil {
		return nil, nil, err
	}
	return &node, cfg, nil
}

func allowVerificationRequest(r *http.Request) bool {
	token := strings.TrimSpace(helperConfig.GetString("VERIFICATION_TOKEN"))
	if token == "" {
		// 未配置 token 时，至少要求来源是内网地址。
		return isPrivateRequest(r)
	}
	headerToken := strings.TrimSpace(r.Header.Get("X-Verification-Token"))
	if headerToken == "" || headerToken != token {
		return false
	}
	return isPrivateRequest(r)
}

func isPrivateRequest(r *http.Request) bool {
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP != "" {
		parts := strings.Split(clientIP, ",")
		clientIP = strings.TrimSpace(parts[0])
	} else {
		host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
		if err != nil {
			host = strings.TrimSpace(r.RemoteAddr)
		}
		clientIP = host
	}
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return false
	}
	return ip.IsPrivate() || ip.IsLoopback()
}

func writeRuntimeJSON(w http.ResponseWriter, status int, data map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func GetRuntimeVerificationDefaultTimeoutSec() int {
	timeout := helperConfig.GetInt("VERIFICATION_DEFAULT_TIMEOUT_SEC", 30)
	if timeout <= 0 {
		return 30
	}
	return timeout
}

func BuildRuntimeVerificationURL(baseURL, path string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "http://127.0.0.1:9001"
	}
	return base + "/" + strings.TrimLeft(path, "/")
}

func ParseNodeIDFromString(v string) int32 {
	id, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 32)
	return int32(id)
}
