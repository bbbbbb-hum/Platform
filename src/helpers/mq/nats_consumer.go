package mq

import (
	"AgentEarth_AgentPlatform/src/boot"
	"AgentEarth_AgentPlatform/src/helpers/config"
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/models"
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// RequestLogsConsumer 请求日志消费者
type RequestLogsConsumer struct {
	sub      *nats.Subscription
	stopChan chan struct{}
	wg       sync.WaitGroup
	handler  RequestLogsBatchHandler
}

// RequestLogsBatchHandler 批量日志处理函数类型
type RequestLogsBatchHandler func(logs []*models.AeMcpServicesRequestLogs) error

var (
	requestLogsConsumer     *RequestLogsConsumer
	onceRequestLogsConsumer sync.Once
)

// GetRequestLogsConsumer 获取消费者单例
func GetRequestLogsConsumer() *RequestLogsConsumer {
	onceRequestLogsConsumer.Do(func() {
		requestLogsConsumer = &RequestLogsConsumer{
			stopChan: make(chan struct{}),
		}
	})
	return requestLogsConsumer
}

// Start 启动消费者
// handler: 处理批量日志的回调函数，如果为 nil 则使用默认处理（打印日志）
func (c *RequestLogsConsumer) Start(handler RequestLogsBatchHandler) error {
	if !boot.IsNatsEnabled() {
		return ErrNatsNotEnabled
	}

	js := boot.GetJetStream()
	if js == nil {
		return ErrJetStreamNotAvailable
	}

	if handler == nil {
		handler = defaultBatchHandler
	}
	c.handler = handler

	streamName := config.GetString("nats.stream_name")
	subject := boot.GetNatsSubject()
	consumerName := "request-logs-consumer"

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
		zap.String("consumer", consumerName))

	return nil
}

// consumeLoop 消费循环
func (c *RequestLogsConsumer) consumeLoop() {
	defer c.wg.Done()

	for {
		select {
		case <-c.stopChan:
			logger.Info("NATS 消费者收到停止信号")
			return
		default:
			// 拉取消息，每次最多10条，超时1秒
			msgs, err := c.sub.Fetch(10, nats.MaxWait(1*time.Second))
			if err != nil {
				if err == nats.ErrTimeout {
					// 超时是正常的，继续循环
					continue
				}
				if err == nats.ErrConnectionClosed {
					logger.Warn("NATS 连接已关闭，消费者退出")
					return
				}
				logger.Error("拉取消息失败", zap.Error(err))
				time.Sleep(time.Second) // 出错时稍微等待
				continue
			}

			for _, msg := range msgs {
				c.processMessage(msg)
			}
		}
	}
}

// processMessage 处理单条消息（消息内容是批量日志）
func (c *RequestLogsConsumer) processMessage(msg *nats.Msg) {
	var batch RequestLogsBatch
	if err := json.Unmarshal(msg.Data, &batch); err != nil {
		logger.Error("解析消息失败",
			zap.String("data", string(msg.Data)),
			zap.Error(err))
		// 解析失败，确认消息避免重复投递
		msg.Ack()
		return
	}

	if len(batch.Logs) == 0 {
		msg.Ack()
		return
	}

	// 调用处理函数
	if err := c.handler(batch.Logs); err != nil {
		logger.Error("处理消息失败",
			zap.Int("count", batch.Count),
			zap.Error(err))
		// 处理失败，使用 Nak 让消息重新投递
		msg.Nak()
		return
	}

	// 处理成功，确认消息
	msg.Ack()
	logger.Debug("消费请求日志成功", zap.Int("count", batch.Count))
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

// defaultBatchHandler 默认处理函数（打印日志用于测试）
func defaultBatchHandler(logs []*models.AeMcpServicesRequestLogs) error {
	logger.Info("收到请求日志批次", zap.Int("count", len(logs)))
	for i, log := range logs {
		logger.Info("日志详情",
			zap.Int("index", i),
			zap.String("server_id", log.ServerId),
			zap.String("tool_name", log.ToolName),
			zap.String("user_id", log.UserId),
			zap.Int32("response_time_ms", log.ResponseTime),
			zap.Int16("status", log.Status),
			zap.Float64("xlcredit_amount", log.XlcreditAmount))
	}
	return nil
}

// StartWithContext 带 context 的启动方式
func (c *RequestLogsConsumer) StartWithContext(ctx context.Context, handler RequestLogsBatchHandler) error {
	if err := c.Start(handler); err != nil {
		return err
	}

	// 监听 context 取消
	go func() {
		<-ctx.Done()
		c.Stop()
	}()

	return nil
}

// BatchInsertHandler 批量插入数据库的处理函数（用于生产环境）
func BatchInsertHandler(logs []*models.AeMcpServicesRequestLogs) error {
	logger.Debug("消费者收到请求日志批次，开始批量插入数据库...", zap.Int("count", len(logs)))
	return models.BatchInsert(logs)
}
