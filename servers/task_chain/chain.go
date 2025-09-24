package task_chain

import (
	"AgentEarth_AgentPlatform/models"
	"AgentEarth_AgentPlatform/servers/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Chain interface {
	Init(config InitConfig) error
	Process(rc *types.RunningContext, userCmd string, userParamMap map[string]interface{}) (currentResp map[string]*types.CallToolResult, err error)
	GetTools(rc *types.RunningContext) []*mcp.Tool
	GetChainInfo() *ChainInfo
}

type (
	// InitConfig 链初始化参数
	InitConfig struct {
		ServiceId  string
		ChainModel *models.AeMcpTaskChain
	}
	// ChainInfo 链信息
	ChainInfo struct {
		ChainID   int32  `json:"chain_id"`
		ServiceID string `json:"service_id"`
	}
)
