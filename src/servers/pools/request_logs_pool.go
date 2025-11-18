package pools

import (
	"AgentEarth_AgentPlatform/src/helpers/logger"
	"AgentEarth_AgentPlatform/src/models"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RequestLogsPool 请求日志批量插入池
type RequestLogsPool struct {
	logs         []*models.AeMcpServicesRequestLogs // 待插入的日志列表
	mutex        sync.Mutex                         // 保护 logs 和 stopped 的并发访问
	stopped      bool                               // 是否已停止，停止后拒绝新的 Add() 调用
	flushStop    chan struct{}                      // 停止信号
	flushWG      sync.WaitGroup                     // 等待 flush 协程退出
	maxBatchSize int                                // 最大批量大小，避免单次插入过多
}

var (
	globalRequestLogsPool *RequestLogsPool
	onceRequestLogsPool   sync.Once
)

// GetRequestLogsPool 获取请求日志池单例
func GetRequestLogsPool() *RequestLogsPool {
	onceRequestLogsPool.Do(func() {
		globalRequestLogsPool = &RequestLogsPool{
			logs:         make([]*models.AeMcpServicesRequestLogs, 0),
			maxBatchSize: 1000, // 默认最大批量大小为1000
		}
	})
	return globalRequestLogsPool
}

// Add 添加日志到待插入队列
func (p *RequestLogsPool) Add(log *models.AeMcpServicesRequestLogs) {
	if log == nil {
		return
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// 如果已停止，拒绝添加新的日志,新日志写入 文件，如果需要，手动处理
	if p.stopped {
		logger.Error("请求日志池已停止，拒绝添加新日志", zap.Any("miss_request_log", log))
		return
	}

	p.logs = append(p.logs, log)
}

// Start 启动定时批量插入协程
func (p *RequestLogsPool) Start(interval time.Duration) {
	p.mutex.Lock()
	// 防重复启动
	if p.flushStop != nil {
		p.mutex.Unlock()
		return
	}
	stop := make(chan struct{})
	p.flushStop = stop
	p.stopped = false // 重置停止标志
	// 在启动 goroutine 前登记
	p.flushWG.Add(1)
	p.mutex.Unlock()

	logger.Info("启动请求日志批量插入协程", zap.Duration("interval", interval))

	go func() {
		defer p.flushWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				// 停止前执行最后一次同步
				p.flush()
				return
			case <-ticker.C:
				p.flush()
			}
		}
	}()
}

// Stop 停止批量插入协程
func (p *RequestLogsPool) Stop() {
	// 先设置停止标志，阻止新的 Add() 调用
	p.mutex.Lock()
	p.stopped = true
	if p.flushStop != nil {
		close(p.flushStop)
		p.flushStop = nil
	}
	p.mutex.Unlock()

	// 等待 flush 协程退出，确保最后一次同步完成
	p.flushWG.Wait()

	// 协程退出后，处理剩余数据（在设置 stopped 之前可能已经加入的数据）
	// 循环处理直到没有剩余数据
	for {
		p.mutex.Lock()
		if len(p.logs) == 0 {
			p.mutex.Unlock()
			break
		}
		// 取出剩余数据
		batch := make([]*models.AeMcpServicesRequestLogs, len(p.logs))
		copy(batch, p.logs)
		p.logs = p.logs[:0]
		p.mutex.Unlock()

		// 插入剩余数据
		if len(batch) > p.maxBatchSize {
			p.flushInBatches(batch)
		} else {
			p.insertBatch(batch)
		}
		logger.Info("停止时同步剩余日志", zap.Int("count", len(batch)))
	}

	logger.Info("请求日志批量插入协程已停止")
}

// flush 执行批量插入
func (p *RequestLogsPool) flush() {
	// 取出待插入的日志（使用双缓冲，避免阻塞写入）
	p.mutex.Lock()
	if len(p.logs) == 0 {
		p.mutex.Unlock()
		return
	}
	// 创建临时切片，避免长时间持有锁
	batch := make([]*models.AeMcpServicesRequestLogs, len(p.logs))
	copy(batch, p.logs)
	// 清空原切片
	p.logs = p.logs[:0]
	p.mutex.Unlock()

	// 如果批量大小超过限制，分批插入
	if len(batch) > p.maxBatchSize {
		p.flushInBatches(batch)
	} else {
		p.insertBatch(batch)
	}
}

// flushInBatches 分批插入（当批量过大时）
func (p *RequestLogsPool) flushInBatches(batch []*models.AeMcpServicesRequestLogs) {
	total := len(batch)
	for i := 0; i < total; i += p.maxBatchSize {
		end := i + p.maxBatchSize
		if end > total {
			end = total
		}
		p.insertBatch(batch[i:end])
	}
}

// insertBatch 执行单次批量插入
func (p *RequestLogsPool) insertBatch(batch []*models.AeMcpServicesRequestLogs) {
	if len(batch) == 0 {
		return
	}

	err := models.BatchInsert(batch)
	if err != nil {
		logger.Error("批量插入请求日志失败",
			zap.Int("count", len(batch)),
			zap.Error(err))
		// 插入失败时，将日志写入文件，后续手动处理
		logger.Error("批量插入请求日志失败，已写入文件", zap.Any("miss_request_log", batch))
	} else {
		logger.Debug("批量插入请求日志成功", zap.Int("count", len(batch)))
	}
}

// GetPendingCount 获取待插入的日志数量（用于监控）
func (p *RequestLogsPool) GetPendingCount() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return len(p.logs)
}
