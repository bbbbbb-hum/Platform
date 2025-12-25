package users

import (
	"AgentEarth_AgentPlatform/src/models"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const TableNameAeUserRequestServicesLogs = "ae_user_request_services_logs"

// 用户请求mcp服务日志表
type AeUserRequestServicesLogs struct {
	Id         int32     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`   // 自增主键
	UserId     string    `gorm:"column:user_id;not null" json:"user_id"`         // 用户uuid
	KeyId      int32     `gorm:"column:key_id;not null" json:"key_id"`           // 密钥id
	ServerId   string    `gorm:"column:server_id;not null" json:"server_id"`     // 服务id
	Calls      int32     `gorm:"column:calls;not null" json:"calls"`             // 调用次数
	Tokens     int32     `gorm:"column:tokens;not null" json:"tokens"`           // tokens使用量
	Year       int16     `gorm:"column:year;not null" json:"year"`               // 年
	Month      int16     `gorm:"column:month;not null" json:"month"`             // 月
	Day        int16     `gorm:"column:day;not null" json:"day"`                 // 日
	CreateTime time.Time `gorm:"column:create_time;not null" json:"create_time"` // 创建时间
	UpdateTime time.Time `gorm:"column:update_time;not null" json:"update_time"` // 更新时间
}

func (l *AeUserRequestServicesLogs) TableName() string {
	return TableNameAeUserRequestServicesLogs
}

func (l *AeUserRequestServicesLogs) GetOneByUKSDate(userID string, keyID int32, serverID string, year, month, day int16) error {
	return models.GetDB().
		Where("user_id = ? AND key_id = ? AND server_id = ? AND year = ? AND month = ? AND day = ?",
			userID, keyID, serverID, year, month, day).
		First(l).Error
}

// GetUserCallsSumByUSRange returns total calls for a user+server within a time range (summed across all keys).
// We use create_time as each (user_id,key_id,server_id,day) row is created on that day.
func GetUserCallsSumByUSRange(userID, serverID string, startInclusive, endExclusive time.Time) (int64, error) {
	var sum int64
	err := models.GetDB().
		Model(&AeUserRequestServicesLogs{}).
		Select("COALESCE(SUM(calls), 0)").
		Where("user_id = ? AND server_id = ? AND create_time >= ? AND create_time < ?",
			userID, serverID, startInclusive, endExclusive).
		Scan(&sum).Error
	return sum, err
}

// AddUsageUpsert 按 (user_id, key_id, server_id, year, month, day) 维度 upsert + 累加 calls/tokens.
// 依赖数据库上存在对应的唯一键/唯一索引，否则 on conflict 不会生效。
func AddUsageUpsert(userID string, keyID int32, serverID string, year, month, day int16, callsDelta, tokensDelta int32) error {
	if callsDelta == 0 && tokensDelta == 0 {
		return nil
	}
	now := time.Now()
	row := &AeUserRequestServicesLogs{
		UserId:     userID,
		KeyId:      keyID,
		ServerId:   serverID,
		Calls:      callsDelta,
		Tokens:     tokensDelta,
		Year:       year,
		Month:      month,
		Day:        day,
		CreateTime: now,
		UpdateTime: now,
	}

	db := models.GetDB()
	err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "key_id"},
			{Name: "server_id"},
			{Name: "year"},
			{Name: "month"},
			{Name: "day"},
		},
		DoUpdates: clause.Assignments(map[string]any{
			"calls":       clause.Expr{SQL: "calls + ?", Vars: []any{callsDelta}},
			"tokens":      clause.Expr{SQL: "tokens + ?", Vars: []any{tokensDelta}},
			"update_time": now,
		}),
	}).Create(row).Error

	return err
}

// IsNotFound 判断 gorm 的 not found。
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
