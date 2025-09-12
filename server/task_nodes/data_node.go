package task_nodes

import (
	"context"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
	"time"
)

// DataNodeProcessor 数据处理节点处理器
type DataNodeProcessor struct{}

func (d *DataNodeProcessor) GetNodeName() string {
	return "数据处理器"
}

func (d *DataNodeProcessor) ProcessNode(nodeCtx *NodeContext) error {
	logger.Debug("上个节点数据", zap.Any("data", nodeCtx.Data))
	// 其他逻辑

	// 注册工具
	err := d.registerTools(nodeCtx.McpServer)
	if err != nil {
		return err
	}
	// 更新节点数据
	nodeCtx.Data = map[string]interface{}{
		"processed_at": time.Now().Unix(),
		"status":       "processed",
		"data":         d.GetNodeName(),
	}
	return nil
}

func (d *DataNodeProcessor) registerTools(mcpServer *mcp.Server) error {
	// 注册数据转换工具
	mcp.AddTool[map[string]interface{}](mcpServer, &mcp.Tool{
		Name:        "data_transform",
		Title:       "数据转换",
		Description: "转换数据格式",
	}, d.DataTransformTool)

	// 注册数据验证工具
	mcp.AddTool[map[string]interface{}](mcpServer, &mcp.Tool{
		Name:        "data_validate",
		Title:       "数据验证",
		Description: "验证数据格式和内容",
	}, d.DataValidateTool)

	logger.Info("数据处理节点处理器注册了 2 个工具")
	return nil
}

// DataTransformTool 数据转换工具
func (d *DataNodeProcessor) DataTransformTool(ctx context.Context, req *mcp.CallToolRequest, args map[string]interface{}) (*mcp.CallToolResult, interface{}, error) {
	transformed := map[string]interface{}{
		"original":       args,
		"transformed_at": time.Now().Unix(),
		"status":         "transformed",
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("数据已转换: %+v", transformed)},
		},
	}, transformed, nil
}

// DataValidateTool 数据验证工具
func (d *DataNodeProcessor) DataValidateTool(ctx context.Context, req *mcp.CallToolRequest, args map[string]interface{}) (*mcp.CallToolResult, interface{}, error) {
	isValid := len(args) > 0

	result := map[string]interface{}{
		"valid":        isValid,
		"validated_at": time.Now().Unix(),
		"data":         args,
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("数据验证结果: %t", isValid)},
		},
	}, result, nil
}
