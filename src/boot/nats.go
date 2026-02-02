package boot

import (
	"AgentEarth_AgentPlatform/src/helpers/config"
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

var (
	natsConn   *nats.Conn
	jetStream  nats.JetStreamContext
	natsOnce   sync.Once
	natsMutex  sync.RWMutex
	natsEnable bool
)

// SetupNats 初始化 NATS 连接和 JetStream
func SetupNats() error {
	if !config.GetBool("nats.enable") {
		logger.Info("NATS disabled", zap.Bool("enable", false))
		return nil
	}

	var setupErr error
	natsOnce.Do(func() {
		setupErr = initNatsConnection()
	})

	return setupErr
}

// initNatsConnection 初始化 NATS 连接
func initNatsConnection() error {
	url := config.GetString("nats.url")
	maxReconnects := config.GetInt("nats.max_reconnects")
	reconnectWait := time.Duration(config.GetInt("nats.reconnect_wait_seconds")) * time.Second
	connectTimeout := time.Duration(config.GetInt("nats.connect_timeout_seconds")) * time.Second

	// 配置连接选项
	opts := []nats.Option{
		nats.MaxReconnects(maxReconnects),
		nats.ReconnectWait(reconnectWait),
		nats.Timeout(connectTimeout),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				logger.Warn("NATS disconnected", zap.Error(err))
			}
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected", zap.String("url", nc.ConnectedUrl()))
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			logger.Info("NATS connection closed")
		}),
		nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
			logger.Error("NATS error", zap.Error(err))
		}),
	}

	// 连接 NATS
	nc, err := nats.Connect(url, opts...)
	if err != nil {
		logger.Error("NATS 连接失败", zap.String("url", url), zap.Error(err))
		return fmt.Errorf("NATS connection failed: %w", err)
	}

	// 创建 JetStream 上下文
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		logger.Error("JetStream 上下文创建失败", zap.Error(err))
		return fmt.Errorf("JetStream context creation failed: %w", err)
	}

	// 确保 Stream 存在
	if err := ensureStream(js); err != nil {
		nc.Close()
		return err
	}

	natsMutex.Lock()
	natsConn = nc
	jetStream = js
	natsEnable = true
	natsMutex.Unlock()

	logger.Info("NATS 连接成功",
		zap.String("url", url),
		zap.String("stream", config.GetString("nats.stream_name")),
		zap.String("subject", config.GetString("nats.subject")),
	)

	return nil
}

// ensureStream 确保 JetStream Stream 存在
func ensureStream(js nats.JetStreamContext) error {
	streamName := config.GetString("nats.stream_name")
	subject := config.GetString("nats.subject")
	maxMsgs := config.GetInt64("nats.stream_max_msgs")
	maxBytes := config.GetInt64("nats.stream_max_bytes")
	maxAgeHours := config.GetInt("nats.stream_max_age_hours")
	replicas := config.GetInt("nats.stream_replicas")

	// 检查 Stream 是否已存在
	stream, err := js.StreamInfo(streamName)
	if err == nil && stream != nil {
		logger.Info("JetStream Stream 已存在", zap.String("stream", streamName))
		return nil
	}

	// 创建 Stream
	streamCfg := &nats.StreamConfig{
		Name:      streamName,
		Subjects:  []string{subject},
		Retention: nats.LimitsPolicy,
		MaxMsgs:   maxMsgs,
		MaxBytes:  maxBytes,
		MaxAge:    time.Duration(maxAgeHours) * time.Hour,
		Storage:   nats.FileStorage,
		Replicas:  replicas,
	}

	_, err = js.AddStream(streamCfg)
	if err != nil {
		// 如果是 stream 已存在的错误，忽略它（可能是并发创建）
		if err.Error() == "stream name already in use" {
			logger.Info("JetStream Stream 已被其他实例创建", zap.String("stream", streamName))
			return nil
		}
		logger.Error("JetStream Stream 创建失败",
			zap.String("stream", streamName),
			zap.Error(err))
		return fmt.Errorf("JetStream stream creation failed: %w", err)
	}

	logger.Info("JetStream Stream 创建成功",
		zap.String("stream", streamName),
		zap.String("subject", subject),
		zap.Int64("max_msgs", maxMsgs),
		zap.Int64("max_bytes", maxBytes),
		zap.Int("max_age_hours", maxAgeHours),
	)

	return nil
}

// GetJetStream 获取 JetStream 上下文
func GetJetStream() nats.JetStreamContext {
	natsMutex.RLock()
	defer natsMutex.RUnlock()
	return jetStream
}

// GetNatsConn 获取 NATS 连接
func GetNatsConn() *nats.Conn {
	natsMutex.RLock()
	defer natsMutex.RUnlock()
	return natsConn
}

// IsNatsEnabled 检查 NATS 是否启用且可用
func IsNatsEnabled() bool {
	natsMutex.RLock()
	defer natsMutex.RUnlock()
	return natsEnable && natsConn != nil && natsConn.IsConnected()
}

// GetNatsSubject 获取配置的发布主题
func GetNatsSubject() string {
	return config.GetString("nats.subject")
}

// CloseNats 关闭 NATS 连接
func CloseNats() {
	natsMutex.Lock()
	defer natsMutex.Unlock()

	if natsConn != nil {
		// 等待所有挂起的发布完成
		if err := natsConn.Drain(); err != nil {
			logger.Warn("NATS drain failed", zap.Error(err))
		}
		natsConn.Close()
		natsConn = nil
		jetStream = nil
		natsEnable = false
		logger.Info("NATS 连接已关闭")
	}
}
