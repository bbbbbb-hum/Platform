package users

import "time"

const TableNameMcpUser = "ae_user"

// 用户表
type McpUser struct {
	Id           int32     `gorm:"column:id;autoIncrement;not null" json:"id"`         // 自增id
	UserId       string    `gorm:"column:user_id;primaryKey" json:"user_id"`           // 唯一标识
	Username     string    `gorm:"column:username;not null" json:"username"`           // 用户名
	PasswordHash string    `gorm:"column:password_hash;not null" json:"password_hash"` // 用户密码
	Phone        string    `gorm:"column:phone;not null" json:"phone"`                 // 手机号码
	Email        string    `gorm:"column:email;not null" json:"email"`                 // 邮箱地址
	AvatarUrl    string    `gorm:"column:avatar_url;not null" json:"avatar_url"`       // 用户头像url
	Status       string    `gorm:"column:status;default:'active'" json:"status"`       // 账号状态：active-正常, inactive-未激活
	LastLoginAt  time.Time `gorm:"column:last_login_at" json:"last_login_at"`          // 最后登录时间
	CreateTime   time.Time `gorm:"column:create_time" json:"create_time"`              // 创建时间
	UpdateTime   time.Time `gorm:"column:update_time" json:"update_time"`              // 更新时间
}

func (u *McpUser) TableName() string {
	return TableNameMcpUser
}
