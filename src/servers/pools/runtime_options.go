package pools

import (
	helperConfig "AgentEarth_AgentPlatform/src/helpers/config"
	"strconv"
	"strings"
)

const (
	defaultEnabledProtocols = "http"
	defaultConnectTimeoutMS = 30000
	defaultListToolsTimeout = 15000
	defaultCallTimeoutMS    = 30000
	nodeServicePrefix       = "node:"
)

func getEnabledProtocols() map[string]bool {
	raw := strings.TrimSpace(helperConfig.GetString("ENABLED_PROTOCOLS", defaultEnabledProtocols))
	if raw == "" {
		raw = defaultEnabledProtocols
	}
	items := strings.Split(strings.ToLower(raw), ",")
	result := make(map[string]bool, len(items))
	for _, item := range items {
		p := strings.TrimSpace(item)
		if p != "" {
			result[p] = true
		}
	}
	return result
}

func isProtocolEnabled(protocol string) bool {
	p := strings.ToLower(strings.TrimSpace(protocol))
	if p == "" {
		p = "http"
	}
	return getEnabledProtocols()[p]
}

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

func buildNodeServiceID(nodeID int32) string {
	return nodeServicePrefix + strconv.FormatInt(int64(nodeID), 10)
}

func parseNodeIDFromServiceID(serviceID string) int32 {
	if !strings.HasPrefix(serviceID, nodeServicePrefix) {
		return 0
	}
	raw := strings.TrimPrefix(serviceID, nodeServicePrefix)
	id, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0
	}
	return int32(id)
}

