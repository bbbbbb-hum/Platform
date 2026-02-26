package users

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/models"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const TableNameAeUserBalanceStatisticDaily = "ae_user_balance_statistic_daily"

// 用户日余额表
type AeUserBalanceStatisticDaily struct {
	Id         int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`          // 用户账户表
	UserId     string    `gorm:"column:user_id;not null" json:"user_id"`                // 用户id
	Day        time.Time `gorm:"column:day;not null" json:"day"`                        // 日期
	Balance    float64   `gorm:"column:balance;default:0" json:"balance"`               // 余额
	CreateTime time.Time `gorm:"column:create_time;default:'now()'" json:"create_time"` // 创建时间
	UpdateTime time.Time `gorm:"column:update_time;default:'now()'" json:"update_time"` // 更新时间
}

func (l *AeUserBalanceStatisticDaily) TableName() string {
	return TableNameAeUserBalanceStatisticDaily
}

// GetTodayBalance 获取今日余额
func (l *AeUserBalanceStatisticDaily) GetTodayBalance() (float64, error) {
	if l.UserId == "" {
		return 0, errors.New("userId is empty")
	}
	var balanceDaily AeUserBalanceStatisticDaily
	today := time.Now().Format("2006-01-02")
	err := models.GetDB().
		Where("day=?", today).
		Where("user_id=?", l.UserId).Find(&balanceDaily).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("获取今日余额失败", zap.String("userId", l.UserId), zap.Error(err))
		return 0, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	return balanceDaily.Balance, nil
}

// AddTodayBalance 添加今日余额
func (l *AeUserBalanceStatisticDaily) AddTodayBalance(userId string) (float64, error) {
	db := models.GetDB()
	// 获取最新日期余额
	var balanceDaily AeUserBalanceStatisticDaily
	err := db.Where("user_id=?", userId).Order("day desc").First(&balanceDaily).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.Error("获取最新日期余额失败", zap.Error(err))
		return 0, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		balanceDaily = AeUserBalanceStatisticDaily{

			UserId:  userId,
			Balance: 0,
		}
	}
	balanceDaily.Id = 0
	balanceDaily.Day = time.Now()
	balanceDaily.CreateTime = time.Now()
	balanceDaily.UpdateTime = time.Now()
	err = db.Create(&balanceDaily).Error
	return balanceDaily.Balance, err
}
