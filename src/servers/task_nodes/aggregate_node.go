package task_nodes

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/models"
	"AgentEarth_AgentPlatform/src/servers/types"
	"fmt"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// AggregateNode 聚合节点，将多个子节点的工具聚合到一个服务中
type AggregateNode struct {
	toolList       []*types.ToolDesc          // 预计算的工具列表
	toolToNode     map[string]types.Processor // 聚合工具名 -> 节点实例
	NodeInfo       *types.NodeInfo
	mutex          sync.RWMutex
}

// Init 初始化聚合节点
func (a *AggregateNode) Init(config types.InitConfig) error {
	logger.Info("初始化聚合节点", zap.Int32("node_id", config.NodeModel.Id))

	a.NodeInfo = &types.NodeInfo{
		ServiceID:   config.ServiceID,
		ChainID:     config.ChainModel.Id,
		NodeID:      config.NodeModel.Id,
		NodeHandle:  config.NodeModel.NodeHandle,
		NodeName:    config.NodeModel.NodeName,
		Description: config.NodeModel.Description,
		Enabled:     true,
	}

	subNodeNames, err := ParseAggregateConfigNames(config.NodeModel.NodeConfig)
	if err != nil {
		return fmt.Errorf("parse_aggregate_config_failed: %w", err)
	}

	return a.initSubNodes(config, subNodeNames)
}

// initSubNodes 初始化所有子节点
func (a *AggregateNode) initSubNodes(config types.InitConfig, subNodeNames []string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.toolToNode = make(map[string]types.Processor)
	a.toolList = make([]*types.ToolDesc, 0)

	nodeModel := &models.AeMcpTaskNode{}
	err, subNodes := nodeModel.GetNodesByNames(subNodeNames)
	if err != nil {
		return fmt.Errorf("query_sub_nodes_failed: %w", err)
	}

	for _, subNode := range subNodes {
		if subNode.NodeHandle == "aggregate_handle" {
			logger.Error("子节点不能是聚合节点", zap.String("node_name", subNode.NodeName))
			continue
		}

		processor, ok := CreateNodeByType(subNode.NodeHandle)
		if !ok {
			logger.Error("不支持的子节点类型", zap.String("node_handle", subNode.NodeHandle))
			continue
		}

		err = processor.Init(types.InitConfig{
			ServiceID:  config.ServiceID,
			ChainModel: config.ChainModel,
			NodeModel:  subNode,
		})
		if err != nil {
			logger.Error("初始化子节点失败", zap.String("node_name", subNode.NodeName), zap.Error(err))
			continue
		}

		tools := processor.GetTools(&types.RunningContext{})
		for _, tool := range tools {
			aggregatedName := "E_" + subNode.NodeName + "_" + tool.ToolName
			a.toolToNode[aggregatedName] = processor
			a.toolList = append(a.toolList, &types.ToolDesc{
				ToolName:        aggregatedName,
				ToolDesc:        tool.ToolDesc,
				ToolInputSchema: tool.ToolInputSchema,
			})
		}

		logger.Info("成功初始化子节点", zap.String("node_name", subNode.NodeName))
	}

	return nil
}

// GetTools 获取聚合后的工具列表
func (a *AggregateNode) GetTools(rc *types.RunningContext) []*types.ToolDesc {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.toolList
}

// Process 处理工具调用，路由到对应子节点
func (a *AggregateNode) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp *mcp.CallToolResult) (*mcp.CallToolResult, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	processor, ok := a.toolToNode[userCmd]
	if !ok {
		return nil, fmt.Errorf("tool_not_found: %s", userCmd)
	}

	originalName := parseOriginalToolName(userCmd)
	return processor.Process(rc, originalName, userParamMap, lastStepResp)
}

// GetNodeInfo 获取节点信息
func (a *AggregateNode) GetNodeInfo() *types.NodeInfo {
	return a.NodeInfo
}

// parseOriginalToolName 解析原始工具名：E_NodeName_ToolName -> ToolName
func parseOriginalToolName(aggregatedName string) string {
	// E_NodeName_ToolName，找到第二个 _ 后的部分
	if idx := strings.Index(aggregatedName[2:], "_"); idx != -1 {
		return aggregatedName[2+idx+1:]
	}
	return aggregatedName
}
