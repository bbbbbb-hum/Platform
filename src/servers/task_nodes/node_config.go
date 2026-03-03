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

// ParseAggregateConfigIDs 解析聚合节点配置，返回子节点ID列表
func ParseAggregateConfigIDs(raw string) ([]int32, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, fmt.Errorf("invalid_aggregate_config: config is empty")
	}
	//把 JSON 格式的数字数组 "[1, 2, 3]" 解析成 Go 的整数切片 []int32{1, 2, 3} 。
	//JSON 格式的数字数组 "[1, 2, 3]"是数据库里存的
	var nodeIDs []int32
	if err := json.Unmarshal([]byte(trimmed), &nodeIDs); err != nil {
		return nil, fmt.Errorf("invalid_aggregate_config: parse json failed: %w", err)
	}

	if len(nodeIDs) == 0 {
		return nil, fmt.Errorf("invalid_aggregate_config: node list is empty")
	}
	return nodeIDs, nil
}

// ExtractServiceNameFromURL 从 node_config 的 URL 中提取服务名
// 规则：提取第二个和第三个 - 之间的字段
// 例如：http://xxx-Serpapi-xxx -> Serpapi
func ExtractServiceNameFromURL(nodeConfig string) (string, error) {
	cfg, err := ParseNodeRuntimeConfig(nodeConfig)
	if err != nil {
		return "", err
	}

	url := cfg.URL
	// 查找所有 - 的位置
	dashPositions := []int{}
	for i, ch := range url {
		if ch == '-' {
			dashPositions = append(dashPositions, i)
		}
	}

	// 需要至少3个横杠才能提取第二个和第三个之间的内容
	if len(dashPositions) < 3 {
		return "", fmt.Errorf("url format invalid: need at least 3 dashes, got %d", len(dashPositions))
	}

	// 提取第二个和第三个 - 之间的字段（索引1和2）
	start := dashPositions[1] + 1
	end := dashPositions[2]
	serviceName := url[start:end]

	if serviceName == "" {
		return "", fmt.Errorf("extracted service name is empty")
	}

	return serviceName, nil
}
