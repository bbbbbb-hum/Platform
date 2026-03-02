package servers

import (
	helperConfig "AgentEarth_AgentPlatform/src/helpers/config"
	"AgentEarth_AgentPlatform/src/models"
	"AgentEarth_AgentPlatform/src/servers/pools"
	"AgentEarth_AgentPlatform/src/servers/task_nodes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
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

// appendDebugD37A85 是临时排障输出（直接 stdout），
// 保留用于对齐某次线上问题的追踪字段。后续稳定后建议收敛到统一 logger。
func appendDebugD37A85(runID, hypothesisID, location, message string, data map[string]interface{}) {
	payload := map[string]interface{}{
		"sessionId":    "d37a85",
		"runId":        runID,
		"hypothesisId": hypothesisID,
		"location":     location,
		"message":      message,
		"data":         data,
		"timestamp":    time.Now().UnixMilli(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	fmt.Println(string(raw))
}

func RuntimeConnectHandler(w http.ResponseWriter, r *http.Request) {
	// #region agent log
	appendDebugD37A85("pre-fix", "H4", "runtime_verification.go:RuntimeConnectHandler", "platform api runtime connect entered", map[string]interface{}{
		"remote_addr":   r.RemoteAddr,
		"forwarded_for": r.Header.Get("X-Forwarded-For"),
		"method":        r.Method,
		"path":          r.URL.Path,
	})
	// #endregion
	if !allowVerificationRequest(r) {
		writeRuntimeJSON(w, http.StatusForbidden, map[string]interface{}{
			"success": false,
			"error":   "verification_forbidden",
		})
		return
	}
	var req runtimeConnectReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// 请求体格式错误（非 JSON / 字段类型不匹配）
		writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid_request"})
		return
	}
	if req.NodeID <= 0 {
		writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "node_id_required"})
		return
	}

	start := time.Now()
	//根据 nodeID 查数据库
	var node models.AeMcpTaskNode
	if err := models.GetDB().Where("id = ?", int32(req.NodeID)).First(&node).Error; err != nil {
		writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	// 判断是否是聚合节点
	if strings.TrimSpace(node.NodeHandle) == "aggregate_handle" {
		// 1. 解析配置，得到子节点名称列表 ["天气节点", "搜索节点"]
		names, err := task_nodes.ParseAggregateConfig(node.NodeConfig)
		if err != nil {
			writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		timeoutMS := 0
		if req.Timeout > 0 {
			timeoutMS = req.Timeout * 1000
		}
		type subInfo struct {
			name string
			id   int32
		}
		subNodes := make([]subInfo, 0, len(names))
		// 2. 遍历每个子节点
		for _, n := range names {
			// 查数据库获取子节点信息
			sub, e := (&models.AeMcpTaskNode{}).GetByNodeName(n)
			if e != nil || sub == nil || !sub.Enabled {
				continue
			}
			cfg, e := task_nodes.ParseNodeRuntimeConfig(sub.NodeConfig)
			if e != nil {
				continue
			}
			tms := cfg.TimeoutMS
			if timeoutMS > 0 {
				tms = timeoutMS
			}
			if err := pools.GetConnectPool().InitializeNode(sub.Id, cfg.Protocol, cfg.URL, tms, cfg.MaxConnect); err != nil {
				continue
			}
			subNodes = append(subNodes, subInfo{name: strings.ReplaceAll(n, " ", "_"), id: sub.Id})
		}
		toolList := make([]map[string]interface{}, 0, 16)
		// 5. 获取每个子节点的工具
		for _, si := range subNodes {
			tools := pools.GetConnectPool().GetNodeTools(si.id)
			for _, tool := range tools {
				if tool == nil {
					continue
				}
				// 6. 加前缀！
				name := si.name + "_" + tool.Name
				item := map[string]interface{}{
					"name":        name,
					"description": tool.Description,
				}
				if tool.InputSchema != nil {
					item["inputSchema"] = tool.InputSchema
				}
				toolList = append(toolList, item)
			}
		}
		resp := map[string]interface{}{
			"success":     true,
			"node_id":     req.NodeID,
			"service_url": "",
			"tools":       toolList,
			"tools_count": len(toolList),
		}
		writeRuntimeJSON(w, http.StatusOK, resp)
		return
	}
	nodeRec, cfg, err := loadRuntimeNodeConfig(int32(req.NodeID))
	if err != nil {
		writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	timeoutMS := cfg.TimeoutMS
	if req.Timeout > 0 {
		timeoutMS = req.Timeout * 1000
	}
	if err := pools.GetConnectPool().InitializeNode(nodeRec.Id, cfg.Protocol, cfg.URL, timeoutMS, cfg.MaxConnect); err != nil {
		writeRuntimeJSON(w, http.StatusBadGateway, map[string]interface{}{"success": false, "error": normalizeRuntimeInitError(err, cfg.URL)})
		return
	}
	tools := pools.GetConnectPool().GetNodeTools(nodeRec.Id)
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
	resp := map[string]interface{}{
		"success":     true,
		"node_id":     req.NodeID,
		"service_url": cfg.URL,
		"tools":       toolList,
		"tools_count": len(toolList),
		"duration_ms": time.Since(start).Milliseconds(),
	}
	if poolState := pools.GetConnectPool().GetNodePoolState(nodeRec.Id); poolState != nil {
		resp["node_service_key"] = poolState.NodeServiceKey
		resp["active_connections"] = poolState.ActiveConnections
		resp["target_connections"] = poolState.TargetConnections
	}
	writeRuntimeJSON(w, http.StatusOK, resp)
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

	// runtime_verification.go 第 215-299 行
	// 调用聚合节点工具的接口

	start := time.Now() // 记录开始时间

	// ===== 第1步：根据 nodeID 查数据库，找到节点 =====
	var node models.AeMcpTaskNode
	if err := models.GetDB().Where("id = ?", int32(req.NodeID)).First(&node).Error; err != nil {
		writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	// ===== 第2步：判断是不是聚合节点 =====
	if strings.TrimSpace(node.NodeHandle) == "aggregate_handle" {

		// ===== 第3步：解析工具名 =====  (第222行)
		// 用户调用的工具名可能是 "天气节点_get_weather"
		// 需要用 "_" 分割成两部分：["天气节点", "get_weather"]
		// SplitN 限制分割2次，所以结果是 ["天气节点", "get_weather"]
		parts := strings.SplitN(strings.TrimSpace(req.ToolName), "_", 2)

		// ===== 第4步：校验格式 =====  (第224行)
		// 必须正好分成2部分，且两部分都不能为空
		// 比如 "_get_weather" 或 "天气节点_" 都是不合法的
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid_aggregated_tool"})
			return
		}

		// ===== 第5步：提取前缀和原工具名 =====  (第228-229行)
		// parts[0] = "天气节点" (前缀，节点名称)
		// parts[1] = "get_weather" (原工具名)
		targetClean := strings.TrimSpace(parts[0]) // 提取前缀并去空格: "天气节点"
		origTool := strings.TrimSpace(parts[1])    // 提取原工具名: "get_weather"

		// ===== 第6步：解析配置，获取子节点名称列表 =====  (第231-235行)
		// 从数据库的 node_config 字段解析出配置的子节点名称
		// 比如配置是 ["天气节点", "搜索节点"]，names 就是 ["天气节点", "搜索节点"]
		names, err := task_nodes.ParseAggregateConfig(node.NodeConfig)
		if err != nil {
			writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}

		// ===== 第7步：找到对应的子节点 =====  (第237-246行)
		// 遍历配置的子节点名称列表，找到和前缀匹配的节点
		var sub *models.AeMcpTaskNode // 子节点对象
		for _, n := range names {
			// 将配置中的节点名称空格换成下划线，然后和前缀比较
			// 比如配置是 "天气 节点"，换成下划线后是 "天气_节点"，和 targetClean "天气_节点" 匹配
			if strings.ReplaceAll(n, " ", "_") == targetClean {
				// 找到匹配的节点，通过节点名称查询数据库获取完整节点信息
				sub, err = (&models.AeMcpTaskNode{}).GetByNodeName(n)
				// 如果查询失败、节点为空或未启用，则设为 nil
				if err != nil || sub == nil || !sub.Enabled {
					sub = nil
				}
				break // 找到后退出循环
			}
		}

		// ===== 第8步：检查子节点是否存在 =====  (第248-252行)
		// 如果没找到对应的子节点，返回错误
		if sub == nil {
			writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "aggregated_target_not_found"})
			return
		}

		// ===== 第9步：解析子节点的配置 =====  (第254-258行)
		// 解析子节点的 node_config，获取协议、URL、超时等配置
		cfgSub, err := task_nodes.ParseNodeRuntimeConfig(sub.NodeConfig)
		if err != nil {
			writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}

		// ===== 第10步：设置超时时间 =====  (第260-264行)
		// 默认用子节点的配置超时时间
		timeoutMS := cfgSub.TimeoutMS
		// 如果请求中指定了超时时间，则使用请求中的超时时间
		if req.Timeout > 0 {
			timeoutMS = req.Timeout * 1000 // 转换为毫秒
		}

		// ===== 第11步：初始化子节点连接 =====  (第266-270行)
		// 建立与子节点服务的连接
		if err := pools.GetConnectPool().InitializeNode(sub.Id, cfgSub.Protocol, cfgSub.URL, timeoutMS, cfgSub.MaxConnect); err != nil {
			writeRuntimeJSON(w, http.StatusBadGateway, map[string]interface{}{"success": false, "error": normalizeRuntimeInitError(err, cfgSub.URL)})
			return
		}

		// ===== 第12步：解析调用参数 =====  (第272-279行)
		// 用户调用工具时传入的参数，比如 {"city": "北京"}
		args := make(map[string]interface{})
		if len(req.Arguments) > 0 {
			// 将 JSON 格式的参数解析成 Go 的 map
			if err := json.Unmarshal(req.Arguments, &args); err != nil {
				writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid_arguments_json"})
				return
			}
		}

		// ===== 第13步：调用子节点的工具 =====  (第281-289行)
		// 关键：传入的是 origTool ("get_weather")，不是带前缀的工具名！
		// 调用子节点的 CallToolByNode 方法执行工具
		result, err := pools.GetConnectPool().CallToolByNode(sub.Id, origTool, args)
		if err != nil {
			writeRuntimeJSON(w, http.StatusBadGateway, map[string]interface{}{
				"success":     false,
				"tool_name":   req.ToolName,
				"error":       err.Error(),
				"duration_ms": time.Since(start).Milliseconds(),
			})
			return
		}

		// ===== 第14步：处理返回结果 =====  (第291-300行)
		// 处理返回的内容，转换为通用格式
		contentAny := interface{}(nil)
		if result != nil && len(result.Content) > 0 {
			// 将返回的内容序列化为 JSON，再反序列化为 interface{} 类型
			if raw, err := json.Marshal(result.Content); err == nil {
				_ = json.Unmarshal(raw, &contentAny)
			}
		}

		// 判断是否返回了错误
		isError := false
		if result != nil {
			isError = result.IsError
		}

		// ===== 第15步：返回成功响应 =====  (第302-308行)
		resp := map[string]interface{}{
			"success":     true,                             // 成功标志
			"tool_name":   req.ToolName,                     // 工具名称（带前缀）
			"is_error":    isError,                          // 是否返回错误
			"content":     contentAny,                       // 返回的内容
			"duration_ms": time.Since(start).Milliseconds(), // 耗时
		}
		writeRuntimeJSON(w, http.StatusOK, resp)
		return
	}

	//总结，用户需要我要天气节点_get_weather，我用前缀找对应的子节点"天气节点"，
	// 然后用 origTool ("get_weather") 调用子节点的工具。
	
	nodeRec, cfg, err := loadRuntimeNodeConfig(int32(req.NodeID))
	if err != nil {
		writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": err.Error()})
		return
	}
	timeoutMS := cfg.TimeoutMS
	if req.Timeout > 0 {
		timeoutMS = req.Timeout * 1000
	}
	if err := pools.GetConnectPool().InitializeNode(nodeRec.Id, cfg.Protocol, cfg.URL, timeoutMS, cfg.MaxConnect); err != nil {
		writeRuntimeJSON(w, http.StatusBadGateway, map[string]interface{}{"success": false, "error": normalizeRuntimeInitError(err, cfg.URL)})
		return
	}

	args := make(map[string]interface{})
	if len(req.Arguments) > 0 {
		// arguments 是原始 json，支持对象参数直接透传到 MCP Tool。
		if err := json.Unmarshal(req.Arguments, &args); err != nil {
			writeRuntimeJSON(w, http.StatusBadRequest, map[string]interface{}{"success": false, "error": "invalid_arguments_json"})
			return
		}
	}

	result, err := pools.GetConnectPool().CallToolByNode(nodeRec.Id, strings.TrimSpace(req.ToolName), args)
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
		// result.Content 可能含接口类型，这里做一轮 json 归一化，方便前端直接展示。
		if raw, err := json.Marshal(result.Content); err == nil {
			_ = json.Unmarshal(raw, &contentAny)
		}
	}
	isError := false
	if result != nil {
		isError = result.IsError
	}

	resp := map[string]interface{}{
		"success":     true,
		"tool_name":   req.ToolName,
		"is_error":    isError,
		"content":     contentAny,
		"duration_ms": time.Since(start).Milliseconds(),
	}
	if poolState := pools.GetConnectPool().GetNodePoolState(node.Id); poolState != nil {
		resp["node_service_key"] = poolState.NodeServiceKey
		resp["active_connections"] = poolState.ActiveConnections
		resp["target_connections"] = poolState.TargetConnections
	}
	writeRuntimeJSON(w, http.StatusOK, resp)
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

	// disconnect 语义是显式释放该 node 的连接资源（连接池项可重建）。
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
	// 这里以主键 id 定位节点，避免引入 config_id / 外部配置表耦合。
	if err := models.GetDB().Where("id = ?", nodeID).First(&node).Error; err != nil {
		return nil, nil, err
	}
	// ParseNodeRuntimeConfig 内会做 url 必填 + 协议限定 + timeout 兼容。
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
	// 配置了 token 时采用双条件：token 正确 + 内网来源。
	headerToken := strings.TrimSpace(r.Header.Get("X-Verification-Token"))
	if headerToken == "" || headerToken != token {
		return false
	}
	return isPrivateRequest(r)
}

func isPrivateRequest(r *http.Request) bool {
	// 优先用 X-Forwarded-For 的首个 IP，兼容经由网关转发的请求。
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
	// 统一 runtime 接口响应写法，避免分散编码导致返回格式不一致。
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

func normalizeRuntimeInitError(err error, url string) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if strings.Contains(msg, "protocol_not_supported") {
		// 协议错误给用户可执行提示，减少“看日志才能知道”。
		return "仅支持 HTTP 协议，请使用内网 service URL（示例：http://ae-xxx-service:8081）"
	}
	if strings.Contains(msg, "http_connect_failed") || strings.Contains(msg, "no_instance_created") {
		// 建连错误带上 url，方便一眼确认 service 名称/端口是否写错。
		return fmt.Sprintf("HTTP连接失败，请检查 service 名称/端口/网络：url=%s, err=%s", strings.TrimSpace(url), msg)
	}
	return msg
}
