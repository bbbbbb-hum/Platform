package users

import (
	"AgentEarth_AgentPlatform/models"
	"time"

	"gorm.io/datatypes"
)

const TableNameMcpUserKeys = "mcp_user_keys"

// 用户密钥表
type McpUserKeys struct {
	Id          int32             `gorm:"column:id;primaryKey;autoIncrement" json:"id"`     // 自增id,主键
	UserId      string            `gorm:"column:user_id;not null" json:"user_id"`           // 用户id
	KeyName     string            `gorm:"column:key_name;not null" json:"key_name"`         // 密钥名称
	KeyValue    string            `gorm:"column:key_value;not null" json:"key_value"`       // 密钥值，全局唯一
	KeyType     string            `gorm:"column:key_type;default:'api'" json:"key_type"`    // 密钥类型：api-api密钥, access_token-访问令牌, refresh_token-刷新令牌, secret-密钥
	Status      string            `gorm:"column:status;default:'active'" json:"status"`     // 密钥状态：active-有效, inactive-禁用, revoked-已撤销
	Permissions datatypes.JSONMap `gorm:"column:permissions;default:{}" json:"permissions"` // 钥权限范围json数组
	ExpiresAt   time.Time         `gorm:"column:expires_at" json:"expires_at"`              // 密钥过期时间
	LastUsedAt  time.Time         `gorm:"column:last_used_at" json:"last_used_at"`          // 最后使用时间
	UsageCount  int32             `gorm:"column:usage_count;default:0" json:"usage_count"`  // 密钥使用次数统计
	CreateTime  time.Time         `gorm:"column:create_time" json:"create_time"`            // 创建时间
	UpdateTime  time.Time         `gorm:"column:update_time" json:"update_time"`            // 更新时间
}

func (k *McpUserKeys) TableName() string {
	return TableNameMcpUserKeys
}

func (k *McpUserKeys) GetOneByKeyValue(value string) error {
	return models.GetDB().Where("key_value = ?", value).First(k).Error
}
