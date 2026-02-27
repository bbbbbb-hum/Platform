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
