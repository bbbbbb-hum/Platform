package task_chain

import (
	"AgentEarth_AgentPlatform/servers/ctx"
	"AgentEarth_AgentPlatform/servers/task_nodes"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type TaskChain interface {
	Init(config InitConfig) error
	Process(ctx *ctx.RunningContext, userCmd string, userParamMap map[string]interface{}) (currentResp map[string]*ctx.CallToolResult, err error)
	GetTools(ctx *ctx.RunningContext) []*mcp.Tool
	GetChain() *Chain
}

var (
	chainOne sync.Once
	chainMap *ChainMap
)

type (
	ChainMap struct {
		chains map[string]*Chain // map[serverId]Chain 链实例Map
		mutex  sync.RWMutex
	}
	Chain struct {
		TaskChain TaskChain
		ChainInfo *ChainInfo
		Nodes     []task_nodes.Node
	}
	ChainInfo struct {
		ChainID   int32  `json:"chain_id"`
		ServiceID string `json:"service_id"`
	}
)

func GetChainMap() *ChainMap {
	chainOne.Do(func() {
		chainMap = &ChainMap{
			chains: make(map[string]*Chain),
		}
	})
	return chainMap
}

// GetChainByServiceId 获取指定服务ID的链
func (c *ChainMap) GetChainByServiceId(serviceId string) *Chain {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if chain, exists := c.chains[serviceId]; exists {
		return chain
	}
	return nil
}

// AddChain 添加链到映射中
func (c *ChainMap) AddChain(serviceId string, chain *Chain) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.chains[serviceId] = chain
}

// RemoveChain 从映射中移除链
func (c *ChainMap) RemoveChain(serviceId string) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exists := c.chains[serviceId]; exists {
		delete(c.chains, serviceId)
		return true
	}
	return false
}

// HasChain 检查是否存在指定服务的链
func (c *ChainMap) HasChain(serviceId string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	_, exists := c.chains[serviceId]
	return exists
}

// GetAllChains 获取所有链的副本
func (c *ChainMap) GetAllChains() map[string]*Chain {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// 返回副本以避免并发修改
	result := make(map[string]*Chain)
	for serviceId, chain := range c.chains {
		result[serviceId] = chain
	}
	return result
}

// GetServiceIds 获取所有服务ID列表
func (c *ChainMap) GetServiceIds() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	serviceIds := make([]string, 0, len(c.chains))
	for serviceId := range c.chains {
		serviceIds = append(serviceIds, serviceId)
	}
	return serviceIds
}

// Count 获取链的总数
func (c *ChainMap) Count() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return len(c.chains)
}
