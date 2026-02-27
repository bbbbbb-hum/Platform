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

