package agentapi

import (
	"AgentEarth_AgentPlatform/src/helpers"
	"AgentEarth_AgentPlatform/src/servers"
	"AgentEarth_AgentPlatform/src/servers/types"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// agent-api 使用的 Skill 服务 ID（超级集合服务，写死）
const agentSkillServerID = "server_0000408"

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, `{"error":"failed to encode response"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}

func getSkillServer() (string, *servers.Server, error) {
	// 当前 agent-api 统一路由到一个“超级集合服务”，
	// 因此直接使用固定 server id 从已加载的服务映射中取实例。
	server := servers.McpServicesMap[agentSkillServerID]
	if server == nil || server.ChainInstance == nil {
		return "", nil, errors.New("skill server not found")
	}

	return agentSkillServerID, server, nil
}

func findToolByName(toolList []*types.ToolDesc, toolName string) *types.ToolDesc {
	for _, tool := range toolList {
		if tool != nil && tool.ToolName == toolName {
			return tool
		}
	}
	return nil
}

func buildRunningContext(r *http.Request, server *servers.Server) (*types.RunningContext, error) {
	userID, ok := r.Context().Value(helpers.ContextKeyUserID).(string)
	if !ok || userID == "" {
		return nil, errors.New("user information not obtained")
	}

	keyID, ok := r.Context().Value("key_id").(int64)
	if !ok {
		return nil, errors.New("key_id not obtained")
	}

	return &types.RunningContext{
		ServiceID: server.ChainInstance.ServerID,
		ChainID:   server.ChainInstance.ChainID,
		Stats: map[string]interface{}{
			"user_id": userID,
			"key_id":  keyID,
		},
	}, nil
}

func decodeJSONBody(r *http.Request, target interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("invalid request body: %w", err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("invalid request body: multiple JSON values")
	}
	return nil
}

func prepareMatchQuery(query string) string {
	query = strings.TrimSpace(query)
	if query == "" {
		return ""
	}

	// agent-api 作为机器调用接口，不再依赖外部翻译服务
	// 统一做大小写标准化，由调用方决定是否先进行中英转换。
	return strings.ToLower(query)
}

func splitWords(input string) []string {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	// 去重后保留词序，避免同一关键词重复加权。
	seen := make(map[string]struct{}, len(parts))
	words := make([]string, 0, len(parts))
	for _, part := range parts {
		word := strings.TrimSpace(part)
		if word == "" {
			continue
		}
		if _, ok := seen[word]; ok {
			continue
		}
		seen[word] = struct{}{}
		words = append(words, word)
	}
	return words
}
