package task_nodes

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/servers/types"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"

	"AgentEarth_AgentPlatform/src/helpers"
	"reflect"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
)

// EchoNode (简单的业务节点)回声工具节点 - 提供工具 B
type EchoNode struct {
	NodeInfo *NodeInfo //节点信息
}

func (e *EchoNode) Init(config InitConfig) error {
	logger.Info("初始化回声工具节点", zap.String("node_id", string(config.NodeModel.Id)))
	e.NodeInfo = &NodeInfo{
		NodeID:      config.NodeModel.Id,
		NodeHandle:  config.NodeModel.NodeHandle,
		NodeName:    config.NodeModel.NodeName,
		Description: config.NodeModel.Description,
	}
	return nil
}

func (e *EchoNode) GetTools(rc *types.RunningContext, lastStepToolList []*ToolDesc) (currentToolList []*ToolDesc) {
	currentToolList = lastStepToolList
	// 增加一个工具
	echoParamsSchema, err := jsonschema.ForType(reflect.TypeOf(EchoParams{}), &jsonschema.ForOptions{
		IgnoreInvalidTypes: true,
	})
	if err != nil {
		logger.Error("转换工具参数失败", zap.Error(err))
		return
	}
	currentToolList = append(currentToolList, &ToolDesc{
		ToolName:        "Echo",
		ToolDesc:        "回声工具",
		ToolInputSchema: echoParamsSchema,
	})
	e.NodeInfo.ToolNames = append(e.NodeInfo.ToolNames, "Echo")
	logger.Info("Echo 新增了1个工具")
	return currentToolList
}
func (e *EchoNode) Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}, lastStepResp map[string]*types.CallToolResult) (currentResp map[string]*types.CallToolResult, err error) {
	if lastStepResp != nil {
		currentResp = lastStepResp
	} else {
		currentResp = make(map[string]*types.CallToolResult)
	}
	//check userCmd
	if len(e.NodeInfo.ToolNames) > 0 && helpers.InStrArray(userCmd, e.NodeInfo.ToolNames) {
		logger.Debug("当前节点开始处理...", zap.String("tool_name", userCmd), zap.Int32("node_id", e.NodeInfo.NodeID))
		// 检查是否为当前节点的 echo 工具调用参数
		var text string
		if v, ok := userParamMap["text"]; ok {
			if s, ok := v.(string); ok {
				text = s
			}
		}
		if text == "" {
			err = fmt.Errorf("echo工具缺少参数")
			return
		}
		result := fmt.Sprintf("Echo: %s", text)
		currentResp[userCmd] = &types.CallToolResult{
			Result: &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: result},
				},
			},
			StructuredResult: nil,
		}
		// 将结果保存到节点上下文
		rc.ResultMap[e.NodeInfo.NodeID] = currentResp
		logger.Debug("当前节点处理完成", zap.String("tool_name", userCmd), zap.Int32("node_id", e.NodeInfo.NodeID))
	} else {
		logger.Debug("当前节点不处理", zap.String("tool_name", userCmd), zap.Int32("node_id", e.NodeInfo.NodeID))
	}
	return
}

// GetNodeInfo 获取节点信息
func (e *EchoNode) GetNodeInfo() *NodeInfo {
	return e.NodeInfo
}

type EchoParams struct {
	Text string `json:"text"`
}
