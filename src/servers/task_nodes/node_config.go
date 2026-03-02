package task_nodes

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NodeRuntimeConfig 节点运行配置（最小可用字段）。
type NodeRuntimeConfig struct {
	URL        string `json:"url"`
	Protocol   string `json:"protocol"`
	TimeoutMS  int    `json:"timeout_ms"`
	Timeout    int    `json:"timeout"`
	MaxConnect int    `json:"max_connect"`
}

// ParseNodeRuntimeConfig 解析 node_config，兼容仅配置 url 的场景。
func ParseNodeRuntimeConfig(raw string) (*NodeRuntimeConfig, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("invalid_node_config: node_config is empty")
	}

	cfg := &NodeRuntimeConfig{}
	if err := json.Unmarshal([]byte(trimmed), cfg); err != nil {
		return nil, fmt.Errorf("invalid_node_config: parse json failed: %w", err)
	}

	cfg.URL = strings.TrimSpace(cfg.URL)
	if cfg.URL == "" {
		return nil, fmt.Errorf("invalid_node_config: url is required")
	}

	// 协议规范化：未填写按 http；填写非 http 直接报错，避免隐式降级导致误判。
	cfg.Protocol = strings.ToLower(strings.TrimSpace(cfg.Protocol))
	if cfg.Protocol == "" || cfg.Protocol == "http" {
		cfg.Protocol = "http"
	} else {
		return nil, fmt.Errorf("protocol_not_supported: only http is allowed")
	}
	// 兼容历史字段 timeout（老数据可能传 timeout，语义与 timeout_ms 一致）。
	if cfg.TimeoutMS <= 0 && cfg.Timeout > 0 {
		cfg.TimeoutMS = cfg.Timeout
	}

	return cfg, nil
}

// ParseAggregateConfig 解析聚合节点配置，返回子节点名称列表

// 输入：数据库里存储的 node_config 字符串，比如 '"[\"天气节点\", \"搜索节点\"]"'
// 输出：Go 的字符串数组，比如 ["天气节点", "搜索节点"]

func ParseAggregateConfig(raw string) ([]string, error) {

	// ===== 第1步：去除首尾空格 =====
	// raw 是从数据库读取的字符串，可能包含多余空格
	// 比如 '  ["天气节点", "搜索节点"]  '
	// TrimSpace 会去掉首尾的空白字符
	trimmed := strings.TrimSpace(raw)

	// 如果去掉空格后是空字符串，说明配置为空
	if trimmed == "" {
		// 返回错误
		return nil, fmt.Errorf("invalid_aggregate_config: config is empty")
	}

	// ===== 第2步：解析 JSON 数组 =====
	// 声明一个字符串切片，用来存放解析后的节点名称
	var nodeNames []string

	// 使用 json.Unmarshal 将 JSON 字符串解析成 Go 的切片
	// 输入: trimmed = '"[\"天气节点\", \"搜索节点\"]"'
	// 输出: nodeNames = ["天气节点", "搜索节点"]
	if err := json.Unmarshal([]byte(trimmed), &nodeNames); err != nil {
		// 如果解析失败（比如 JSON 格式不对），返回错误
		return nil, fmt.Errorf("invalid_aggregate_config: parse json failed: %w", err)
	}

	// ===== 第3步：检查数组是否为空 =====
	// 如果解析后数组长度为 0，说明配置错误
	if len(nodeNames) == 0 {
		return nil, fmt.Errorf("invalid_aggregate_config: node list is empty")
	}

	// ===== 第4步：去除每个元素的首尾空格 =====
	// 遍历数组中的每个元素，去掉首尾空格
	// 比如 " 天气节点 " 变成 "天气节点"
	// 这样可以防止配置中有多余空格导致匹配失败
	for i, name := range nodeNames {
		nodeNames[i] = strings.TrimSpace(name)
	}

	// ===== 第5步：返回结果 =====
	// 返回解析后的数组和 nil 错误
	return nodeNames, nil
}

// 总结：ParseAggregateConfig 函数负责解析聚合节点的配置字符串，将其转换为 Go 语言中的字符串数组。
// 它会进行多个步骤的校验和处理，确保配置的正确性和完整性。 todo，如果比方说是天气 服务 节点，那么子节点名称就是"天气节点"，还是改成天气_服务
