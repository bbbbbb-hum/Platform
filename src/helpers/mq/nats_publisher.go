package mq

import (
	"AgentEarth_AgentPlatform/src/boot"
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/models"
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// RequestLogsBatch 请求日志批次消息格式
type RequestLogsBatch struct {
	Count int                                 `json:"count"` // 日志数量
	Logs  []*models.AeMcpServicesRequestLogs `json:"logs"`  // 日志列表
}

// PublishRequestLogs 发布批量请求日志到 NATS
// 返回 error，调用方可以根据错误决定是否降级处理
func PublishRequestLogs(logs []*models.AeMcpServicesRequestLogs) error {
	if len(logs) == 0 {
		return nil
	}

	if !boot.IsNatsEnabled() {
		return ErrNatsNotEnabled
	}

	js := boot.GetJetStream()
	if js == nil {
		return ErrJetStreamNotAvailable
	}

	// 构造批次消息
	batch := RequestLogsBatch{
		Count: len(logs),
		Logs:  logs,
	}

	// 序列化为 JSON
	data, err := json.Marshal(batch)
	if err != nil {
		logger.Error("序列化请求日志批次失败",
			zap.Int("count", len(logs)),
			zap.Error(err))
		return err
	}

	// 发布到 JetStream
	subject := boot.GetNatsSubject()
	ack, err := js.Publish(subject, data, nats.AckWait(5*time.Second))
	if err != nil {
		logger.Error("发布请求日志到 NATS 失败",
			zap.String("subject", subject),
			zap.Int("count", len(logs)),
			zap.Error(err))
		return err
	}

	logger.Debug("发布请求日志到 NATS 成功",
		zap.String("subject", subject),
		zap.Int("count", len(logs)),
		zap.Uint64("sequence", ack.Sequence))

	return nil
}

// PublishRequestLogsAsync 异步发布批量请求日志到 NATS（不阻塞调用方）
func PublishRequestLogsAsync(logs []*models.AeMcpServicesRequestLogs) {
	if len(logs) == 0 {
		return
	}

	go func() {
		if err := PublishRequestLogs(logs); err != nil {
			// 发布失败，记录到错误日志便于后续补录
			logger.Error("异步发布请求日志失败",
				zap.Int("count", len(logs)),
				zap.Error(err),
				zap.Any("miss_request_logs", logs))
		}
	}()
}

// IsNatsAvailable 检查 NATS 是否可用
func IsNatsAvailable() bool {
	return boot.IsNatsEnabled()
}
