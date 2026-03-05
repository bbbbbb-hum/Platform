package task_nodes

import (
	"encoding/json"
	"fmt"
	"strings"
)

// NodeRuntimeConfig 节点运行配置（最小可用字段）。
// 说明：
// - URL：上游 MCP 的 httpStreamable 地址（必填）。
// - Protocol：固定为 http（不支持其它协议）。
// - Timeout：统一超时（毫秒），用于 connect/listtools/call。
// - MaxConnect：目标连接数（单实例多连接的并发度）。
type NodeRuntimeConfig struct {
	URL        string `json:"url"`
	Protocol   string `json:"protocol"`
	Timeout    int    `json:"timeout"`
	MaxConnect int    `json:"max_connect"`
}

// ParseNodeRuntimeConfig 解析 node_config（JSON 字符串），最少只需要 URL。
// 说明：
// - 空字符串或缺 URL 均视为非法。
// - Protocol 强制归一为 http，避免协议分支散落。
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

	cfg.Protocol = "http"
	return cfg, nil
}

// ParseAggregateConfigNames 解析聚合节点配置，返回子节点名称列表
func ParseAggregateConfigNames(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("invalid_aggregate_config: config is empty")
	}

	var nodeNames []string
	if err := json.Unmarshal([]byte(trimmed), &nodeNames); err != nil {
		return nil, fmt.Errorf("invalid_aggregate_config: parse json failed: %w", err)
	}

	if len(nodeNames) == 0 {
		return nil, fmt.Errorf("invalid_aggregate_config: node list is empty")
	}
	return nodeNames, nil
}
