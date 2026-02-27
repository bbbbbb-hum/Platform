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
	mutex          sync.RWMutex
	NodeInfo       *types.NodeInfo
	mapToolInfos   map[string]*AggregatedTool // 聚合后工具名 -> 工具信息
	mapNodeHandles map[string]types.Processor // 节点名 -> 节点实例
	subNodeNames   []string                   // 配置的子节点名称列表
	initConfig     types.InitConfig           // 保存初始化配置用于热更新
}

// AggregatedTool 聚合后的工具信息
type AggregatedTool struct {
	OriginalName string          // 原始工具名
	NodeName     string          // 所属节点名（去空格后）
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

	// 解析 node_config 获取子节点名称列表
	subNodeNames, err := ParseAggregateConfig(config.NodeModel.NodeConfig)
	if err != nil {
		return fmt.Errorf("parse_aggregate_config_failed: %w", err)
	}
	a.subNodeNames = subNodeNames

	// 初始化子节点
	return a.initSubNodes(config)
}

// initSubNodes 初始化所有子节点
func (a *AggregateNode) initSubNodes(config types.InitConfig) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.mapNodeHandles = make(map[string]types.Processor)
	a.mapToolInfos = make(map[string]*AggregatedTool)

	nodeModel := &models.AeMcpTaskNode{}
	for _, nodeName := range a.subNodeNames {
		// 根据节点名称查询节点信息
		subNode, err := nodeModel.GetByNodeName(nodeName)
		if err != nil {
			logger.Error("查询子节点失败", zap.String("node_name", nodeName), zap.Error(err))
			continue
		}

		// 防止循环依赖：子节点不能是聚合节点
		if subNode.NodeHandle == "aggregate_handle" {
			logger.Error("子节点不能是聚合节点", zap.String("node_name", nodeName))
			continue
		}

		// 创建子节点实例
		processor, ok := CreateNodeByType(subNode.NodeHandle)
		if !ok {
			logger.Error("不支持的子节点类型", zap.String("node_handle", subNode.NodeHandle))
			continue
		}

		// 初始化子节点
		err = processor.Init(types.InitConfig{
			ServiceID:  config.ServiceID,
			ChainModel: config.ChainModel,
			NodeModel:  subNode,
		})
		if err != nil {
			logger.Error("初始化子节点失败", zap.String("node_name", nodeName), zap.Error(err))
			continue
		}

		// 存储子节点实例
		cleanName := strings.ReplaceAll(nodeName, " ", "")
		a.mapNodeHandles[cleanName] = processor
		logger.Info("成功初始化子节点", zap.String("node_name", nodeName))
	}

	// 构建工具映射表
	a.buildToolInfos()

	return nil
}

// buildToolInfos 构建聚合后的工具映射表
func (a *AggregateNode) buildToolInfos() {
	a.mapToolInfos = make(map[string]*AggregatedTool)

	for nodeName, processor := range a.mapNodeHandles {
		tools := processor.GetTools(&types.RunningContext{})
		for _, tool := range tools {
			// 聚合后工具名 = 节点名(去空格) + "_" + 原工具名
			aggregatedName := nodeName + "_" + tool.ToolName
			a.mapToolInfos[aggregatedName] = &AggregatedTool{
				OriginalName: tool.ToolName,
				NodeName:     nodeName,
				ToolDesc:     tool,
				Node:         processor,
			}
		}
	}
}

// GetTools 获取聚合后的工具列表
func (a *AggregateNode) GetTools(rc *types.RunningContext) []*types.ToolDesc {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	var toolList []*types.ToolDesc
	for aggregatedName, aggTool := range a.mapToolInfos {
		toolList = append(toolList, &types.ToolDesc{
			ToolName:        aggregatedName,
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

	// 查找对应的聚合工具
	aggTool, ok := a.mapToolInfos[userCmd]
	if !ok {
		return nil, fmt.Errorf("tool_not_found: %s", userCmd)
	}

	// 调用子节点处理，使用原始工具名
	return aggTool.Node.Process(rc, aggTool.OriginalName, userParamMap, lastStepResp)
}

// GetNodeInfo 获取节点信息
func (a *AggregateNode) GetNodeInfo() *types.NodeInfo {
	return a.NodeInfo
}

// RefreshConfig 热更新配置
func (a *AggregateNode) RefreshConfig(newConfig string) error {
	logger.Info("开始热更新聚合节点配置", zap.Int32("node_id", a.NodeInfo.NodeID))

	// 解析新配置
	newSubNodeNames, err := ParseAggregateConfig(newConfig)
	if err != nil {
		return fmt.Errorf("parse_new_config_failed: %w", err)
	}

	// 更新配置
	a.subNodeNames = newSubNodeNames
	a.initConfig.NodeModel.NodeConfig = newConfig

	// 重新初始化子节点
	return a.initSubNodes(a.initConfig)
}
