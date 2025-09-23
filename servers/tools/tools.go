package tools

import (
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	onceTools sync.Once
	toolsMap  *Map
)

type Map struct {
	tools map[string]map[string]*mcp.Tool // map[serverId]map[toolName]mcp工具
	mutex sync.RWMutex
}

func GetToolsMap() *Map {
	onceTools.Do(func() {
		toolsMap = &Map{
			tools: make(map[string]map[string]*mcp.Tool),
		}
	})
	return toolsMap
}

// AddTool 添加工具到指定服务器
func (m *Map) AddTool(serverId, toolName string, tool *mcp.Tool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.tools[serverId] == nil {
		m.tools[serverId] = make(map[string]*mcp.Tool)
	}
	m.tools[serverId][toolName] = tool
}

// GetTool 获取指定服务器的指定工具
func (m *Map) GetTool(serverId, toolName string) (*mcp.Tool, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if serverTools, exists := m.tools[serverId]; exists {
		if tool, exists1 := serverTools[toolName]; exists1 {
			return tool, true
		}
	}
	return nil, false
}

// GetServerTools 获取指定服务器的所有工具
func (m *Map) GetServerTools(serverId string) (map[string]*mcp.Tool, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if serverTools, exists := m.tools[serverId]; exists {
		// 返回副本以避免并发修改
		result := make(map[string]*mcp.Tool)
		for name, tool := range serverTools {
			result[name] = tool
		}
		return result, true
	}
	return nil, false
}

// GetAllTools 获取所有工具
func (m *Map) GetAllTools() map[string]map[string]*mcp.Tool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 返回副本以避免并发修改
	result := make(map[string]map[string]*mcp.Tool)
	for serverId, serverTools := range m.tools {
		result[serverId] = make(map[string]*mcp.Tool)
		for toolName, tool := range serverTools {
			result[serverId][toolName] = tool
		}
	}
	return result
}

// GetAllToolNames 获取所有工具名称列表
func (m *Map) GetAllToolNames() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var toolNames []string
	for _, serverTools := range m.tools {
		for toolName := range serverTools {
			toolNames = append(toolNames, toolName)
		}
	}
	return toolNames
}

// GetServerIds 获取所有服务器ID列表
func (m *Map) GetServerIds() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	var serverIds []string
	for serverId := range m.tools {
		serverIds = append(serverIds, serverId)
	}
	return serverIds
}

// RemoveTool 移除指定服务器的指定工具
func (m *Map) RemoveTool(serverId, toolName string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if serverTools, exists := m.tools[serverId]; exists {
		if _, exists = serverTools[toolName]; exists {
			delete(serverTools, toolName)
			// 如果服务器没有工具了，删除服务器条目
			if len(serverTools) == 0 {
				delete(m.tools, serverId)
			}
			return true
		}
	}
	return false
}

// RemoveServer 移除指定服务器的所有工具
func (m *Map) RemoveServer(serverId string) bool {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.tools[serverId]; exists {
		delete(m.tools, serverId)
		return true
	}
	return false
}

// HasTool 检查是否存在指定工具
func (m *Map) HasTool(serverId, toolName string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if serverTools, exists := m.tools[serverId]; exists {
		_, exists := serverTools[toolName]
		return exists
	}
	return false
}

// Count 获取工具总数
func (m *Map) Count() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	count := 0
	for _, serverTools := range m.tools {
		count += len(serverTools)
	}
	return count
}
