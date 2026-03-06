package agentapi

import (
	"AgentEarth_AgentPlatform/src/helpers/cache"
	"net/http"
	"sort"
	"strings"
)

type recommendRequest struct {
	Query       string `json:"query"`
	TaskContext string `json:"task_context,omitempty"`
}

type recommendItem struct {
	ToolName        string      `json:"tool_name"`
	Description     string      `json:"description"`
	InputSchema     interface{} `json:"input_schema,omitempty"`
	EstimatedPoints float64     `json:"estimated_points"`
}

type recommendResponse struct {
	Tools []recommendItem `json:"tools"`
}

type scoredTool struct {
	tool  recommendItem
	score int
}

func HandleRecommend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req recommendRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	serverID, server, err := getSkillServer()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	matchQuery := prepareMatchQuery(req.Query)
	if matchQuery == "" {
		writeJSON(w, http.StatusOK, recommendResponse{Tools: []recommendItem{}})
		return
	}

	words := splitWords(matchQuery)
	if len(words) == 0 {
		writeJSON(w, http.StatusOK, recommendResponse{Tools: []recommendItem{}})
		return
	}

	scored := make([]scoredTool, 0, len(server.GetToolDescList()))
	for _, tool := range server.GetToolDescList() {
		if tool == nil {
			continue
		}

		nameMatched := false
		descMatched := false
		score := 0
		toolNameLower := strings.ToLower(tool.ToolName)
		toolDescLower := strings.ToLower(tool.ToolDesc)

		for _, word := range words {
			if !nameMatched && strings.Contains(toolNameLower, word) {
				score += 2
				nameMatched = true
			}
			if !descMatched && strings.Contains(toolDescLower, word) {
				score += 1
				descMatched = true
			}
			if nameMatched && descMatched {
				break
			}
		}

		if score == 0 {
			continue
		}

		scored = append(scored, scoredTool{
			score: score,
			tool: recommendItem{
				ToolName:        tool.ToolName,
				Description:     tool.ToolDesc,
				InputSchema:     tool.ToolInputSchema,
				EstimatedPoints: cache.GetToolsPrice(serverID, tool.ToolName),
			},
		})
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		if len(scored[i].tool.ToolName) != len(scored[j].tool.ToolName) {
			return len(scored[i].tool.ToolName) < len(scored[j].tool.ToolName)
		}
		return scored[i].tool.ToolName < scored[j].tool.ToolName
	})

	if len(scored) > 3 {
		scored = scored[:3]
	}

	resp := recommendResponse{
		Tools: make([]recommendItem, 0, len(scored)),
	}
	for _, item := range scored {
		resp.Tools = append(resp.Tools, item.tool)
	}

	writeJSON(w, http.StatusOK, resp)
}
