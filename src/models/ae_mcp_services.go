package models

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

const TableNameAeMcpServices = "ae_mcp_services"

// 对外服务表
type AeMcpServices struct {
	Id              int32          `gorm:"column:id;autoIncrement;not null" json:"id"`
	ServerId        string         `gorm:"column:server_id;primaryKey" json:"server_id"`                         // 服务id
	ServerName      string         `gorm:"column:server_name;not null" json:"server_name"`                       // 服务名称
	Logo            string         `gorm:"column:logo;not null" json:"logo"`                                     // logo
	ProtocolVersion string         `gorm:"column:protocol_version;default:'2024-11-05'" json:"protocol_version"` // 协议版本号
	Enabled         bool           `gorm:"column:enabled;default:true" json:"enabled"`                           // 是否开启
	Tags            pq.StringArray `gorm:"column:tags;type:text[];default:'{}'" json:"tags"`                     // 标签名称（多个）
	Description     string         `gorm:"column:description;not null" json:"description"`                       // 描述
	TaskChainId     int32          `gorm:"column:task_chain_id;not null" json:"task_chain_id"`                   // 任务链id
	CreateTime      time.Time      `gorm:"column:create_time" json:"create_time"`                                // 创建时间
	UpdateTime      time.Time      `gorm:"column:update_time" json:"update_time"`                                // 更新时间
	XNetServiceId   string         `gorm:"column:x_net_service_id" json:"x_net_service_id"`                      // xnetserviceid
	CallNum         int32          `gorm:"column:call_num" json:"call_num"`                                      // 调用次数
	CallSuccessNum  int32          `gorm:"column:call_success_num" json:"call_success_num"`                      // 成功次数
	ResponseTime    float32        `gorm:"column:response_time" json:"response_time"`                            // 响应时间 ms 毫秒
	IsCreated       bool           `gorm:"column:is_created;default:true" json:"is_created"`                     // 是否启动时创建服务
}

func (m *AeMcpServices) TableName() string {
	return TableNameAeMcpServices
}

func (m *AeMcpServices) GetList() (err error, list []*AeMcpServices) {
	db := GetDB()
	db = db.Where("is_created = ?", true)
	err = db.Find(&list).Error
	return
}

// 更新调用次数
func (m *AeMcpServices) UpdateCallNum(serverId string) (err error) {
	err = GetDB().Model(&AeMcpServices{}).Where("server_id = ?", serverId).Update("call_num", gorm.Expr("call_num + 1")).Error
	return
}

// 更新成功次数
func (m *AeMcpServices) UpdateCallSuccess(serverId string) (err error) {
	err = GetDB().Model(&AeMcpServices{}).Where("server_id = ?", serverId).Update("call_success_num", gorm.Expr("call_success_num + 1")).Error
	return
}

// 更新响应时间
func (m *AeMcpServices) UpdateResponseTime(serverId string, responseTime float32) (err error) {
	err = GetDB().Model(&AeMcpServices{}).Where("server_id = ?", serverId).Update("response_time", responseTime).Error
	return
}
func (m *AeMcpServices) UpdateEnabled(enabled bool) (err error) {
	err = GetDB().Model(&AeMcpServices{}).Where("id = ?", m.Id).Update("enabled", enabled).Error
	return
}
