package pools

import (
	"strconv"
	"strings"
)

// 连接池运行参数与键规则的集中封装：
// - 默认超时与并发
// - 超时统一获取（节点优先、默认兜底）
// - 并发归一化策略
// - 节点服务键生成/解析
const (
	defaultConnectTimeout   = 30000
	defaultListToolsTimeout = 15000
	defaultCallTimeout      = 30000
	defaultNodeMaxConnect   = 1
	nodeServicePrefix       = "node:"
)

func getConnectTimeout(connectInfo *ConnectInfo) int {
	if connectInfo != nil && connectInfo.ConnectTimeout > 0 {
		return connectInfo.ConnectTimeout
	}
	return defaultConnectTimeout
}

func getListToolsTimeout() int {
	return defaultListToolsTimeout
}

func getCallTimeout(connectInfo *ConnectInfo) int {
	if connectInfo != nil && connectInfo.CallTimeout > 0 {
		return connectInfo.CallTimeout
	}
	return defaultCallTimeout
}

func normalizeNodeMaxConnect(requested int) (int, string) {
	val := requested
	if val <= 0 {
		val = defaultNodeMaxConnect
	}
	if requested <= 0 {
		return val, "fallback_global"
	}
	return val, "as_requested"
}

func getNodeMaxConnect(requested int) int {
	val, _ := normalizeNodeMaxConnect(requested)
	return val
}

func buildNodeServiceKey(nodeID int32) string {
	return nodeServicePrefix + strconv.FormatInt(int64(nodeID), 10)
}

func parseNodeIDFromServiceKey(nodeServiceKey string) int32 {
	if !strings.HasPrefix(nodeServiceKey, nodeServicePrefix) {
		return 0
	}
	raw := strings.TrimPrefix(nodeServiceKey, nodeServicePrefix)
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0
	}
	return int32(id)
}
