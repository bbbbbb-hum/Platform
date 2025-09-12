package models

import (
	"fmt"
	"github.com/lib/pq"
	"time"

	"github.com/wcs1010270451/helpers/logger"
	"go.uber.org/zap"
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
}

func (m *AeMcpServices) TableName() string {
	return TableNameAeMcpServices
}

func (m *AeMcpServices) GetList() (err error, list []*AeMcpServices) {
	// 获取数据库连接
	db := getDB()

	// 检查数据库连接是否有效
	if db == nil {
		err = fmt.Errorf("数据库连接未初始化")
		return
	}

	// 初始化切片，避免nil指针
	list = make([]*AeMcpServices, 0)

	// PostgreSQL 调试信息
	logger.Info("mcp_services", zap.String("db_type", fmt.Sprintf("%T", db.Dialector)))

	// 检查表是否存在
	if !db.Migrator().HasTable("ae_mcp_services") {
		logger.Warn("mcp_services", zap.String("error", "table ae_mcp_services not exists"))
		// 尝试检查其他可能的表名
		tables := []string{"ae_mcp_services", "AeMcpServices", "\"ae_mcp_services\""}
		for _, tableName := range tables {
			if db.Migrator().HasTable(tableName) {
				logger.Info("mcp_services", zap.String("found_table", tableName))
				break
			}
		}
		return fmt.Errorf("表不存在"), list
	}

	logger.Info("mcp_services", zap.String("action", "start querying table"))

	// 先尝试简单的原生SQL查询测试连接
	var count int64
	err = db.Raw("SELECT COUNT(*) FROM ae_mcp_services").Scan(&count).Error
	if err != nil {
		logger.Error("mcp_services", zap.Error(err), zap.String("action", "raw sql count failed"))
		return
	}
	logger.Info("mcp_services", zap.Int64("record_count", count))

	// 执行查询 - 直接指定表名避免模型识别问题
	err = db.Table("ae_mcp_services").Find(&list).Error
	if err != nil {
		// 记录详细的错误信息
		logger.Error("mcp_services", zap.Error(err), zap.String("action", "query failed"))
		return
	}

	logger.Info("mcp_services", zap.Int("found_records", len(list)), zap.String("status", "query success"))
	return
}
