package pools

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/models/config"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
)

// 加载服务配置
func (c *ConnectionPool) loadServiceConfigs(externalServiceId string) (service *ExternalService, err error) {
	logger.Info("加载外部服务配置...", zap.String("external_service_id", externalServiceId))
	externalServiceConfigModel := config.AeMcpExternalServicesConfig{}
	err = externalServiceConfigModel.GetOneByExternalServiceId(externalServiceId)
	if err != nil {
		logger.Error("获取外部服务配置失败", zap.String("external_service_id", externalServiceId), zap.Error(err))
	}
	// 解析JSONB启动配置信息
	var launchInfo LaunchInfo
	launchInfoByte, err := json.Marshal(externalServiceConfigModel.LaunchInfo)
	if err != nil {
		err = fmt.Errorf("解析连接配置失败: %v", err)
		return
	}
	err = json.Unmarshal(launchInfoByte, &launchInfo)
	if err != nil {
		err = fmt.Errorf("解析连接配置失败: %v", err)
		return
	}
	// 解析JSONB连接配置信息
	var connectInfo ConnectInfo
	connectInfoByte, err := json.Marshal(externalServiceConfigModel.ConnectInfo)
	if err != nil {
		err = fmt.Errorf("解析连接配置失败: %v", err)
		return
	}
	err = json.Unmarshal(connectInfoByte, &connectInfo)
	if err != nil {
		err = fmt.Errorf("解析连接配置失败: %v", err)
		return
	}
	service = &ExternalService{
		Id:                externalServiceConfigModel.Id,
		ExternalServiceId: externalServiceConfigModel.ExternalServiceId,
		Type:              externalServiceConfigModel.Type,
		ServiceName:       externalServiceConfigModel.Name,
		MaxInstance:       externalServiceConfigModel.MaxInstance,
		Tools:             nil,
		LaunchInfo:        &launchInfo,
		ConnectInfo:       &connectInfo,
		Accounts:          nil,
		InstanceMap:       make(map[string]*ServiceInstance),
	}
	return
}

// 加载账号信息
func (c *ConnectionPool) loadAccountConfigs(service *ExternalService) (*ExternalService, error) {
	// 从数据库加载账号配置：
	// - 有账号数据：按账号鉴权/替换（URL/Header/ENV）
	// - 无账号数据：视为无需鉴权，直接跳过（不报错）
	var accountModel = config.AeMcpExternalServicesAccount{}
	accounts, err := accountModel.GetListByConfigId(service.Id)
	if err != nil {
		return service, fmt.Errorf("加载账号配置失败: %v", err)
	}
	if len(accounts) == 0 {
		service.Accounts = nil
		return service, nil
	}

	service.Accounts = make([]*ExternalAccount, len(accounts))
	for i, account := range accounts {
		// account.AuthInfo 是 gorm datatypes.JSONMap（map[string]any），这里直接转换为 map[string]string
		authInfo := make(map[string]string, len(account.AuthInfo))
		for k, v := range account.AuthInfo {
			authInfo[k] = fmt.Sprint(v)
		}
		service.Accounts[i] = &ExternalAccount{
			AccountID: account.ConfigId,
			AuthInfo:  authInfo,
		}
	}
	return service, nil
}
