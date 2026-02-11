package mq

import (
	"AgentEarth_AgentPlatform/src/boot"
	"AgentEarth_AgentPlatform/src/helpers/config"
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/models"
	"encoding/json"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

const (
	// 批量处理阈值
	batchSize     = 100             // 累计100条触发插入
	batchInterval = 5 * time.Minute // 或5分钟触发插入
)

// RequestLogsConsumer 请求日志消费者
type RequestLogsConsumer struct {
	sub      *nats.Subscription
	stopChan chan struct{}
	wg       sync.WaitGroup
}

var (
	requestLogsConsumer     *RequestLogsConsumer
	onceRequestLogsConsumer sync.Once
)

// GetRequestLogsConsumer 获取消费者单例
func GetRequestLogsConsumer() *RequestLogsConsumer {
	onceRequestLogsConsumer.Do(func() {
		requestLogsConsumer = &RequestLogsConsumer{}
	})
	return requestLogsConsumer
}

// Start 启动消费者
func (c *RequestLogsConsumer) Start() error {
	if !boot.IsNatsEnabled() {
		return ErrNatsNotEnabled
	}

	js := boot.GetJetStream()
	if js == nil {
		return ErrJetStreamNotAvailable
	}

	streamName := boot.GetJetStreamName()
	subject := boot.GetNatsSubject()
	consumerName := config.GetString("server.namespace") + "_consumer"

	// 创建持久化消费者
	sub, err := js.PullSubscribe(
		subject,
		consumerName,
		nats.ManualAck(),
		nats.AckWait(30*time.Second),
		nats.MaxDeliver(3), // 最多重试3次
		nats.BindStream(streamName),
	)
	if err != nil {
		logger.Error("创建 NATS 消费者失败",
			zap.String("stream", streamName),
			zap.String("subject", subject),
			zap.Error(err))
		return err
	}

	c.sub = sub
	c.stopChan = make(chan struct{})

	// 启动消费协程
	c.wg.Add(1)
	go c.consumeLoop()

	logger.Info("NATS 消费者启动成功",
		zap.String("stream", streamName),
		zap.String("subject", subject),
		zap.String("consumer", consumerName),
		zap.Int("batch_size", batchSize),
		zap.Duration("batch_interval", batchInterval))

	return nil
}

// consumeLoop 消费循环（批量处理）
func (c *RequestLogsConsumer) consumeLoop() {
	defer c.wg.Done()

	// 批量缓冲
	var pendingMsgs []*nats.Msg
	var pendingLogs []*models.AeMcpServicesRequestLogs
	lastFlushTime := time.Now()

	// 定时器，检查是否需要按时间触发刷新
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			// 停止前处理剩余数据
			if len(pendingLogs) > 0 {
				c.flushBatch(pendingMsgs, pendingLogs)
			}
			logger.Info("NATS 消费者收到停止信号")
			return

		case <-ticker.C:
			// 检查是否超过时间阈值
			if len(pendingLogs) > 0 && time.Since(lastFlushTime) >= batchInterval {
				c.flushBatch(pendingMsgs, pendingLogs)
				pendingMsgs = nil
				pendingLogs = nil
				lastFlushTime = time.Now()
			}

		default:
			// 拉取消息
			msgs, err := c.sub.Fetch(10, nats.MaxWait(1*time.Second))
			if err != nil {
				if err == nats.ErrTimeout {
					continue
				}
				if err == nats.ErrConnectionClosed {
					// 停止前处理剩余数据
					if len(pendingLogs) > 0 {
						c.flushBatch(pendingMsgs, pendingLogs)
					}
					logger.Warn("NATS 连接已关闭，消费者退出")
					return
				}
				logger.Error("拉取消息失败", zap.Error(err))
				time.Sleep(time.Second)
				continue
			}

			// 解析消息并加入缓冲
			for _, msg := range msgs {
				log, err := c.parseMessage(msg)
				if err != nil {
					// 解析失败，直接 Ack 避免重复投递
					msg.Ack()
					continue
				}
				pendingMsgs = append(pendingMsgs, msg)
				pendingLogs = append(pendingLogs, log)
			}

			// 检查是否达到数量阈值
			if len(pendingLogs) >= batchSize {
				c.flushBatch(pendingMsgs, pendingLogs)
				pendingMsgs = nil
				pendingLogs = nil
				lastFlushTime = time.Now()
			}
		}
	}
}

// parseMessage 解析消息
func (c *RequestLogsConsumer) parseMessage(msg *nats.Msg) (*models.AeMcpServicesRequestLogs, error) {
	// 先尝试解析批量格式
	var batch RequestLogsBatch
	if err := json.Unmarshal(msg.Data, &batch); err == nil && len(batch.Logs) > 0 {
		// 批量格式，返回第一条（实际上现在生产者每次只发一条）
		return batch.Logs[0], nil
	}

	// 尝试解析单条格式
	var log models.AeMcpServicesRequestLogs
	if err := json.Unmarshal(msg.Data, &log); err != nil {
		logger.Error("解析消息失败",
			zap.String("data", string(msg.Data)),
			zap.Error(err))
		return nil, err
	}
	return &log, nil
}

// flushBatch 批量插入数据库
func (c *RequestLogsConsumer) flushBatch(msgs []*nats.Msg, logs []*models.AeMcpServicesRequestLogs) {
	if len(logs) == 0 {
		return
	}

	logger.Info("开始批量插入数据库", zap.Int("count", len(logs)))

	err := models.BatchInsert(logs)
	if err != nil {
		logger.Error("批量插入数据库失败，消息将重新投递",
			zap.Int("count", len(logs)),
			zap.Error(err))
		// 失败，Nak 所有消息让其重新投递
		for _, msg := range msgs {
			msg.Nak()
		}
		return
	}

	// 成功，Ack 所有消息
	for _, msg := range msgs {
		msg.Ack()
	}
	logger.Info("批量插入数据库成功", zap.Int("count", len(logs)))
}

// Stop 停止消费者
func (c *RequestLogsConsumer) Stop() {
	if c.stopChan != nil {
		close(c.stopChan)
	}
	c.wg.Wait()

	if c.sub != nil {
		c.sub.Unsubscribe()
	}

	logger.Info("NATS 消费者已停止")
}
