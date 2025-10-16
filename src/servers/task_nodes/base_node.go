package task_nodes

import (
	"AgentEarth_AgentPlatform/src/servers/types"
)

// NodeRegistry 节点注册表 - 根据node_type创建对应的节点实例
var NodeRegistry = map[string]func() types.Processor{
	"echo_handle": func() types.Processor {
		return &EchoNode{}
	},
	"logs_handle": func() types.Processor {
		return &LogsNode{}
	},
	"empty_handle": func() types.Processor {
		return &EmptyNode{}
	},
	"proxy_handle": func() types.Processor {
		return &ProxyNode{}
	},
	"statistic_handle": func() types.Processor {
		return &StatisticNode{}
	},
}

// CreateNodeByType 根据node_type创建节点实例
func CreateNodeByType(nodeType string) (types.Processor, bool) {
	if factory, exists := NodeRegistry[nodeType]; exists {
		return types.WithPre(factory()), true
	}
	return nil, false
}
