package models

import (
	"time"
)

const TableNameAeMcpServicesRequestLogs = "ae_mcp_services_request_logs"

// AeMcpServicesRequestLogs mcp服务工具调用日志
type AeMcpServicesRequestLogs struct {
	Id           int32     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`          // 自增主键
	ServerId     string    `gorm:"column:server_id;not null" json:"server_id"`            // 服务id
	ToolName     string    `gorm:"column:tool_name;not null" json:"tool_name"`            // 工具名称
	RequestTime  time.Time `gorm:"column:request_time;not null" json:"request_time"`      // 请求时间
	ReturnTime   time.Time `gorm:"column:return_time;not null" json:"return_time"`        // 返回时间
	ResponseTime int32     `gorm:"column:response_time;not null" json:"response_time"`    // 响应时间(毫秒)
	Status       int16     `gorm:"column:status;default:0" json:"status"`                 // 请求状态：-1 失败 0 位置 1 成功
	CreateTime   time.Time `gorm:"column:create_time;default:'now()'" json:"create_time"` // 创建时间
	UpdateTime   time.Time `gorm:"column:update_time;default:'now()'" json:"update_time"` // 更新时间
}

func (l *AeMcpServicesRequestLogs) TableName() string {
	return TableNameAeMcpServicesRequestLogs
}

// Create 创建MCP服务请求日志
func (l *AeMcpServicesRequestLogs) Create() error {
	return GetDB().Create(l).Error
}

// Update 更新MCP服务请求日志
func (l *AeMcpServicesRequestLogs) Update() error {
	return GetDB().Model(l).Where("id = ?", l.Id).Updates(l).Error
}

// GetLogsByServerId 根据服务id获取日志
func GetLogsByServerId(serverId string) ([]AeMcpServicesRequestLogs, error) {
	var logs []AeMcpServicesRequestLogs
	err := GetDB().Where("server_id = ?", serverId).Find(&logs).Error
	return logs, err
}

func (l *AeMcpServicesRequestLogs) GetOne() error {
	return GetDB().Where("id = ?", l.Id).First(l).Error
}
