package task_nodes

import (
	"AgentEarth_AgentPlatform/models"
	"AgentEarth_AgentPlatform/servers/ctx"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Node 节点接口 - 定义节点的基本行为
type Node interface {
	// 初始化单个task_nodes数据库记录的实例
	Init(config InitConfig) error
	// 获取该节点支持的工具列表
	GetTools(ctx *ctx.RunningContext) (currentToolList []*mcp.Tool)
	// 处理工具调用
	Process(ctx *ctx.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp map[string]*ctx.CallToolResult) (currentResp map[string]*ctx.CallToolResult, err error)
	// 获取节点信息
	GetNodeInfo() *NodeInfo
}

type InitConfig struct {
	ChianID   int32
	ServerID  string
	NodeModel *models.AeMcpTaskNode
}

// NodeInfo 节点基本信息
type NodeInfo struct {
	NodeID      int32  `json:"node_id"`     // 数据库中的节点ID
	NodeType    string `json:"node_type"`   // 节点类型
	NodeName    string `json:"node_name"`   // 节点名称
	Description string `json:"description"` // 节点描述
	Enabled     bool   `json:"enabled"`     // 节点是否启用
}

// NodeInstance 节点实例
type NodeInstance struct {
	Node     Node        //节点实现的接口
	NodeInfo *NodeInfo   //节点信息
	Tools    []*mcp.Tool // 节点支持的工具
}

// NodeInstanceMap 节点实例映射
type NodeInstanceMap struct {
	nodes map[int32][]*NodeInstance // map[chainId]NodeInstance 节点实例Map
	mutex sync.RWMutex
}

var (
	nodeOne sync.Once
	nodeMap *NodeInstanceMap
)

// NodeRegistry 节点注册表 - 根据node_type创建对应的节点实例
var NodeRegistry = map[string]func() Node{
	"echo":   func() Node { return &EchoNode{} },
	"logger": func() Node { return &LoggerNode{} },
}

// CreateNodeByType 根据node_type创建节点实例
func CreateNodeByType(nodeType string) (Node, bool) {
	if factory, exists := NodeRegistry[nodeType]; exists {
		return factory(), true
	}
	return nil, false
}

func GetNodeInstanceMap() *NodeInstanceMap {
	nodeOne.Do(func() {
		nodeMap = &NodeInstanceMap{
			nodes: make(map[int32][]*NodeInstance),
		}
	})
	return nodeMap
}

// GetNodesByChainId 获取指定链的节点实例
func (n *NodeInstanceMap) GetNodesByChainId(chainId int32) ([]*NodeInstance, bool) {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	if nodes, exists := n.nodes[chainId]; exists {
		// 返回副本以避免并发修改
		result := make([]*NodeInstance, len(nodes))
		copy(result, nodes)
		return result, true
	}
	return nil, false
}

// AddNode 添加节点实例到指定链
func (n *NodeInstanceMap) AddNode(chainId int32, nodeInstance *NodeInstance) {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	if n.nodes[chainId] == nil {
		n.nodes[chainId] = make([]*NodeInstance, 0)
	}
	n.nodes[chainId] = append(n.nodes[chainId], nodeInstance)
}

// RemoveNode 从指定链中移除节点实例
func (n *NodeInstanceMap) RemoveNode(chainId int32, nodeId int32) bool {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	if nodes, exists := n.nodes[chainId]; exists {
		for i, node := range nodes {
			if node.NodeInfo.NodeID == nodeId {
				// 移除节点
				n.nodes[chainId] = append(nodes[:i], nodes[i+1:]...)
				// 如果链没有节点了，删除链条目
				if len(n.nodes[chainId]) == 0 {
					delete(n.nodes, chainId)
				}
				return true
			}
		}
	}
	return false
}

// RemoveChain 移除指定链的所有节点
func (n *NodeInstanceMap) RemoveChain(chainId int32) bool {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	if _, exists := n.nodes[chainId]; exists {
		delete(n.nodes, chainId)
		return true
	}
	return false
}

// HasChain 检查是否存在指定链的节点
func (n *NodeInstanceMap) HasChain(chainId int32) bool {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	_, exists := n.nodes[chainId]
	return exists
}

// GetNodeByChainAndId 获取指定链中的指定节点
func (n *NodeInstanceMap) GetNodeByChainAndId(chainId int32, nodeId int32) (*NodeInstance, bool) {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	if nodes, exists := n.nodes[chainId]; exists {
		for _, node := range nodes {
			if node.NodeInfo.NodeID == nodeId {
				return node, true
			}
		}
	}
	return nil, false
}

// GetAllNodes 获取所有节点的副本
func (n *NodeInstanceMap) GetAllNodes() map[int32][]*NodeInstance {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	// 返回副本以避免并发修改
	result := make(map[int32][]*NodeInstance)
	for chainId, nodes := range n.nodes {
		result[chainId] = make([]*NodeInstance, len(nodes))
		copy(result[chainId], nodes)
	}
	return result
}

// GetChainIds 获取所有链ID列表
func (n *NodeInstanceMap) GetChainIds() []int32 {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	chainIds := make([]int32, 0, len(n.nodes))
	for chainId := range n.nodes {
		chainIds = append(chainIds, chainId)
	}
	return chainIds
}

// GetNodeCount 获取指定链的节点数量
func (n *NodeInstanceMap) GetNodeCount(chainId int32) int {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	if nodes, exists := n.nodes[chainId]; exists {
		return len(nodes)
	}
	return 0
}

// GetTotalNodeCount 获取所有节点的总数
func (n *NodeInstanceMap) GetTotalNodeCount() int {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	count := 0
	for _, nodes := range n.nodes {
		count += len(nodes)
	}
	return count
}

// GetChainCount 获取链的总数
func (n *NodeInstanceMap) GetChainCount() int {
	n.mutex.RLock()
	defer n.mutex.RUnlock()

	return len(n.nodes)
}
