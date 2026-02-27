package pools

import (
	helperConfig "AgentEarth_AgentPlatform/src/helpers/config"
	"strconv"
	"strings"
)

const (
	defaultConnectTimeoutMS = 30000
	defaultListToolsTimeout = 15000
	defaultCallTimeoutMS    = 30000
	defaultNodeMaxConnect   = 1
	maxNodeMaxConnect       = 8
	nodeServicePrefix       = "node:"
)

func getConnectTimeoutMS(connectInfo *ConnectInfo) int {
	if connectInfo != nil && connectInfo.ConnectTimeout > 0 {
		return connectInfo.ConnectTimeout
	}
	return helperConfig.GetInt("MCP_CONNECT_TIMEOUT_MS", defaultConnectTimeoutMS)
}

func getListToolsTimeoutMS() int {
	return helperConfig.GetInt("MCP_LISTTOOLS_TIMEOUT_MS", defaultListToolsTimeout)
}

func getCallTimeoutMS(connectInfo *ConnectInfo) int {
	if connectInfo != nil && connectInfo.CallTimeout > 0 {
		return connectInfo.CallTimeout
	}
	return helperConfig.GetInt("MCP_CALL_TIMEOUT_MS", defaultCallTimeoutMS)
}

func normalizeNodeMaxConnect(requested int) (int, string) {
	val := requested
	if val <= 0 {
		val = helperConfig.GetInt("MCP_NODE_MAX_CONNECT", defaultNodeMaxConnect)
		if val <= 0 {
			return defaultNodeMaxConnect, "fallback_default"
		}
	}
	if val > maxNodeMaxConnect {
		return maxNodeMaxConnect, "clamped_max"
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
