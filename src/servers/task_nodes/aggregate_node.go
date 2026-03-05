package task_nodes

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/models"
	"AgentEarth_AgentPlatform/src/servers/types"
	"fmt"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// AggregateNode 聚合节点，将多个子节点的工具聚合到一个服务中
type AggregateNode struct {
	mutex           sync.RWMutex
	NodeInfo        *types.NodeInfo
	mapToolInfos    map[string]*AggregatedTool // 聚合后工具名 -> 工具信息
	mapNodeHandles  map[int32]types.Processor  // 节点ID -> 节点实例
	mapServiceNames map[int32]string           // 节点ID -> 服务名（从URL提取）
	subNodeIDs      []int32                    // 配置的子节点ID列表
	initConfig      types.InitConfig           // 保存初始化配置
}

// AggregatedTool 聚合后的工具信息
type AggregatedTool struct {
	OriginalName string          // 原始工具名
	ToolDesc     *types.ToolDesc // 工具描述
	Node         types.Processor // 对应的节点实例
}

// Init 初始化聚合节点
func (a *AggregateNode) Init(config types.InitConfig) error {
	logger.Info("初始化聚合节点", zap.Int32("node_id", config.NodeModel.Id))

	a.initConfig = config
	a.NodeInfo = &types.NodeInfo{
		ServiceID:   config.ServiceID,
		ChainID:     config.ChainModel.Id,
		NodeID:      config.NodeModel.Id,
		NodeHandle:  config.NodeModel.NodeHandle,
		NodeName:    config.NodeModel.NodeName,
		Description: config.NodeModel.Description,
		Enabled:     true,
	}

	// 解析 node_config，得到子节点ID列表 [1, 2, 3]
	subNodeIDs, err := ParseAggregateConfigIDs(config.NodeModel.NodeConfig)
	if err != nil {
		return fmt.Errorf("parse_aggregate_config_failed: %w", err)
	}
	a.subNodeIDs = subNodeIDs

	return a.initSubNodes(config)
}

// initSubNodes 初始化所有子节点
func (a *AggregateNode) initSubNodes(config types.InitConfig) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.mapNodeHandles = make(map[int32]types.Processor)
	a.mapToolInfos = make(map[string]*AggregatedTool)
	a.mapServiceNames = make(map[int32]string)

	nodeModel := &models.AeMcpTaskNode{}
	err, subNodes := nodeModel.GetChianNodes(a.subNodeIDs)
	if err != nil {
		return fmt.Errorf("query_sub_nodes_failed: %w", err)
	}

	for _, subNode := range subNodes {
		if subNode.NodeHandle == "aggregate_handle" {
			logger.Error("子节点不能是聚合节点", zap.Int32("node_id", subNode.Id))
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
			logger.Error("初始化子节点失败", zap.Int32("node_id", subNode.Id), zap.Error(err))
			continue
		}

		serviceName, err := ExtractServiceNameFromURL(subNode.NodeConfig)
		if err != nil {
			logger.Error("提取服务名失败", zap.Int32("node_id", subNode.Id), zap.Error(err))
			continue
		}

		a.mapNodeHandles[subNode.Id] = processor
		a.mapServiceNames[subNode.Id] = serviceName

		logger.Info("成功初始化子节点",
			zap.Int32("node_id", subNode.Id),
			zap.String("service_name", serviceName))
	}

	a.buildToolInfos()
	return nil
}

// buildToolInfos 构建聚合后的工具映射表
func (a *AggregateNode) buildToolInfos() {
	a.mapToolInfos = make(map[string]*AggregatedTool)

	for nodeID, processor := range a.mapNodeHandles {
		tools := processor.GetTools(&types.RunningContext{})
		serviceName := a.mapServiceNames[nodeID]

		for _, tool := range tools {
			aggregatedName := "E_" + serviceName + "_" + tool.ToolName

			a.mapToolInfos[aggregatedName] = &AggregatedTool{
				OriginalName: tool.ToolName,
				ToolDesc:     tool,
				Node:         processor,
			}
		}
	}
}

// GetTools 获取聚合后的工具列表
func (a *AggregateNode) GetTools(rc *types.RunningContext) []*types.ToolDesc {
	a.mutex.RLock() // 读锁，多人可以同时读
	defer a.mutex.RUnlock()

	var toolList []*types.ToolDesc

	// 遍历 map_ToolInfos，返回所有工具
	for aggregatedName, aggTool := range a.mapToolInfos {
		toolList = append(toolList, &types.ToolDesc{
			ToolName:        aggregatedName, // "E_Serpapi_search"
			ToolDesc:        aggTool.ToolDesc.ToolDesc,
			ToolInputSchema: aggTool.ToolDesc.ToolInputSchema,
		})
	}

	logger.Info("聚合节点获取工具列表", zap.Int("工具数量", len(toolList)))
	return toolList
}

// Process 处理工具调用，路由到对应子节点
func (a *AggregateNode) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp *mcp.CallToolResult) (*mcp.CallToolResult, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	aggTool, ok := a.mapToolInfos[userCmd]
	if !ok {
		return nil, fmt.Errorf("tool_not_found: %s", userCmd)
	}

	return aggTool.Node.Process(rc, aggTool.OriginalName, userParamMap, lastStepResp)
}

// GetNodeInfo 获取节点信息
func (a *AggregateNode) GetNodeInfo() *types.NodeInfo {
	return a.NodeInfo
}
