package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"

	"defi-asset-service/queue-system/config"
	"defi-asset-service/queue-system/models"
)

// Consumer Redis队列消费者
type Consumer struct {
	redisClient *redis.Client
	db          *sql.DB
	config      *config.QueueConfig
	logger      *log.Logger
	
	// 消费者状态
	mu            sync.RWMutex
	isRunning     bool
	workers       []*Worker
	workerWg      sync.WaitGroup
	shutdownChan  chan struct{}
	
	// 监控指标
	metrics       *ConsumerMetrics
}

// Worker 工作协程
type Worker struct {
	id        int
	consumer  *Consumer
	ctx       context.Context
	cancel    context.CancelFunc
	wg        *sync.WaitGroup
}

// ConsumerMetrics 消费者监控指标
type ConsumerMetrics struct {
	mu                 sync.RWMutex
	MessagesProcessed  int64
	MessagesFailed     int64
	MessagesRetried    int64
	MessagesDLQ        int64
	LastProcessedTime  time.Time
	LastErrorTime      time.Time
	ProcessingLatency  time.Duration
	WorkerCount        int
}

// NewConsumer 创建新的消费者
func NewConsumer(redisClient *redis.Client, db *sql.DB, cfg *config.QueueConfig, logger *log.Logger) (*Consumer, error) {
	consumer := &Consumer{
		redisClient:  redisClient,
		db:           db,
		config:       cfg,
		logger:       logger,
		shutdownChan: make(chan struct{}),
		metrics: &ConsumerMetrics{
			WorkerCount: cfg.Workers,
		},
	}
	
	// 确保消费者组存在
	if err := consumer.ensureConsumerGroup(); err != nil {
		return nil, fmt.Errorf("failed to ensure consumer group: %w", err)
	}
	
	return consumer, nil
}

// ensureConsumerGroup 确保消费者组存在
func (c *Consumer) ensureConsumerGroup() error {
	ctx := context.Background()
	
	// 检查消费者组是否存在
	groups, err := c.redisClient.XInfoGroups(ctx, c.config.StreamName).Result()
	if err != nil && err != redis.Nil {
		// Stream可能不存在，尝试创建
		c.logger.Printf("Stream %s may not exist, attempting to create with consumer group", c.config.StreamName)
	}
	
	groupExists := false
	for _, group := range groups {
		if group.Name == c.config.ConsumerGroup {
			groupExists = true
			break
		}
	}
	
	if !groupExists {
		// 创建消费者组
		err := c.redisClient.XGroupCreate(ctx, c.config.StreamName, c.config.ConsumerGroup, "0").Err()
		if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
			return fmt.Errorf("failed to create consumer group: %w", err)
		}
		c.logger.Printf("Created consumer group %s for stream %s", c.config.ConsumerGroup, c.config.StreamName)
	}
	
	return nil
}

// Start 启动消费者
func (c *Consumer) Start(ctx context.Context) error {
	c.mu.Lock()
	if c.isRunning {
		c.mu.Unlock()
		return fmt.Errorf("consumer is already running")
	}
	
	c.isRunning = true
	c.mu.Unlock()
	
	c.logger.Printf("Starting consumer with %d workers", c.config.Workers)
	
	// 创建工作协程
	for i := 0; i < c.config.Workers; i++ {
		workerCtx, cancel := context.WithCancel(ctx)
		worker := &Worker{
			id:       i + 1,
			consumer: c,
			ctx:      workerCtx,
			cancel:   cancel,
			wg:       &c.workerWg,
		}
		
		c.workers = append(c.workers, worker)
		c.workerWg.Add(1)
		go worker.run()
	}
	
	c.logger.Println("Consumer started successfully")
	return nil
}

// Stop 停止消费者
func (c *Consumer) Stop() error {
	c.mu.Lock()
	if !c.isRunning {
		c.mu.Unlock()
		return fmt.Errorf("consumer is not running")
	}
	
	c.isRunning = false
	c.mu.Unlock()
	
	c.logger.Println("Stopping consumer...")
	
	// 取消所有工作协程
	for _, worker := range c.workers {
		worker.cancel()
	}
	
	// 等待所有工作协程结束
	done := make(chan struct{})
	go func() {
		c.workerWg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		c.logger.Println("Consumer stopped successfully")
		return nil
	case <-time.After(30 * time.Second):
		c.logger.Println("Timeout waiting for workers to stop")
		return fmt.Errorf("timeout waiting for workers to stop")
	}
}

// run 工作协程主循环
func (w *Worker) run() {
	defer w.wg.Done()
	defer w.cancel()
	
	w.consumer.logger.Printf("Worker %d started", w.id)
	
	for {
		select {
		case <-w.ctx.Done():
			w.consumer.logger.Printf("Worker %d stopped", w.id)
			return
		default:
			w.processMessages()
		}
	}
}

// processMessages 处理消息
func (w *Worker) processMessages() {
	ctx := w.ctx
	
	// 从Stream读取消息
	messages, err := w.consumer.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    w.consumer.config.ConsumerGroup,
		Consumer: fmt.Sprintf("%s-%d", w.consumer.config.ConsumerName, w.id),
		Streams:  []string{w.consumer.config.StreamName, ">"},
		Count:    w.consumer.config.BatchSize,
		Block:    w.consumer.config.BlockTimeout,
	}).Result()
	
	if err != nil {
		if err == context.Canceled || err == redis.Nil {
			return
		}
		w.consumer.logger.Printf("Worker %d failed to read messages: %v", w.id, err)
		time.Sleep(1 * time.Second)
		return
	}
	
	// 处理消息
	for _, stream := range messages {
		for _, message := range stream.Messages {
			startTime := time.Now()
			
			// 处理消息
			processed, err := w.processMessage(message)
			
			// 更新监控指标
			w.consumer.metrics.mu.Lock()
			w.consumer.metrics.ProcessingLatency = time.Since(startTime)
			w.consumer.metrics.LastProcessedTime = time.Now()
			
			if processed {
				w.consumer.metrics.MessagesProcessed++
			} else if err != nil {
				w.consumer.metrics.MessagesFailed++
				w.consumer.metrics.LastErrorTime = time.Now()
			}
			w.consumer.metrics.mu.Unlock()
			
			// 确认消息
			if processed || w.consumer.config.AutoAck {
				if err := w.ackMessage(message); err != nil {
					w.consumer.logger.Printf("Worker %d failed to ack message %s: %v", w.id, message.ID, err)
				}
			}
		}
	}
}

// processMessage 处理单个消息
func (w *Worker) processMessage(message redis.XMessage) (bool, error) {
	// 解析消息
	positionData, ok := message.Values["position_data"].(string)
	if !ok {
		w.consumer.logger.Printf("Worker %d: invalid position_data in message %s", w.id, message.ID)
		return false, fmt.Errorf("invalid position_data")
	}
	
	// 解析JSON消息
	msg, err := models.FromJSON(positionData)
	if err != nil {
		w.consumer.logger.Printf("Worker %d: failed to parse message %s: %v", w.id, message.ID, err)
		return false, fmt.Errorf("failed to parse message: %w", err)
	}
	
	// 验证消息
	if err := msg.Validate(); err != nil {
		w.consumer.logger.Printf("Worker %d: invalid message %s: %v", w.id, message.ID, err)
		return false, fmt.Errorf("invalid message: %w", err)
	}
	
	// 处理消息（更新MySQL和Redis）
	if err := w.updatePosition(msg); err != nil {
		w.consumer.logger.Printf("Worker %d: failed to update position for message %s: %v", w.id, message.ID, err)
		
		// 检查重试次数
		retryCount := w.getRetryCount(message)
		if retryCount < w.consumer.config.MaxRetries {
			// 重试消息
			if err := w.retryMessage(message, msg, err, retryCount); err != nil {
				w.consumer.logger.Printf("Worker %d: failed to retry message %s: %v", w.id, message.ID, err)
			}
			return false, err
		} else {
			// 达到最大重试次数，移动到死信队列
			if err := w.moveToDLQ(message, msg, err, retryCount); err != nil {
				w.consumer.logger.Printf("Worker %d: failed to move message %s to DLQ: %v", w.id, message.ID, err)
			}
			return false, err
		}
	}
	
	w.consumer.logger.Printf("Worker %d: successfully processed message %s for user %s", w.id, message.ID, msg.UserAddress)
	return true, nil
}

// updatePosition 更新仓位数据到MySQL和Redis
func (w *Worker) updatePosition(msg *models.PositionUpdateMessage) error {
	ctx := context.Background()
	
	// 开始数据库事务
	tx, err := w.consumer.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	
	// 1. 更新MySQL中的用户仓位数据
	if err := w.updateMySQLPosition(tx, msg); err != nil {
		return fmt.Errorf("failed to update MySQL: %w", err)
	}
	
	// 2. 更新Redis缓存
	if err := w.updateRedisCache(ctx, msg); err != nil {
		return fmt.Errorf("failed to update Redis cache: %w", err)
	}
	
	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	return nil
}

// updateMySQLPosition 更新MySQL中的仓位数据
func (w *Worker) updateMySQLPosition(tx *sql.Tx, msg *models.PositionUpdateMessage) error {
	// 这里实现具体的MySQL更新逻辑
	// 根据数据库schema更新相应的表
	
	// 示例：更新用户仓位表
	query := `
		INSERT INTO user_positions 
		(user_address, protocol_id, chain_id, token_address, amount, amount_usd, apy, risk_level, metadata, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
		ON DUPLICATE KEY UPDATE
		amount = VALUES(amount),
		amount_usd = VALUES(amount_usd),
		apy = VALUES(apy),
		risk_level = VALUES(risk_level),
		metadata = VALUES(metadata),
		updated_at = NOW()
	`
	
	// 转换元数据为JSON
	metadataJSON, err := json.Marshal(msg.Position.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	_, err = tx.Exec(query,
		msg.UserAddress,
		msg.ProtocolID,
		msg.ChainID,
		msg.Position.TokenAddress,
		msg.Position.Amount,
		msg.Position.AmountUSD,
		msg.Position.APY,
		msg.Position.RiskLevel,
		metadataJSON,
	)
	
	if err != nil {
		return fmt.Errorf("failed to execute MySQL query: %w", err)
	}
	
	return nil
}

// updateRedisCache 更新Redis缓存
func (w *Worker) updateRedisCache(ctx context.Context, msg *models.PositionUpdateMessage) error {
	// 更新仓位缓存
	cacheKey := fmt.Sprintf("defi:position:%s:%s", msg.UserAddress, msg.ProtocolID)
	
	cacheData := map[string]interface{}{
		"data":       positionDataToJSON(msg.Position),
		"cached_at":  fmt.Sprintf("%d", time.Now().Unix()),
		"ttl":        "600",
	}
	
	// 使用Hash存储
	if err := w.consumer.redisClient.HSet(ctx, cacheKey, cacheData).Err(); err != nil {
		return fmt.Errorf("failed to set Redis cache: %w", err)
	}
	
	// 设置过期时间
	if err := w.consumer.redisClient.Expire(ctx, cacheKey, 600*time.Second).Err(); err != nil {
		return fmt.Errorf("failed to set cache expiration: %w", err)
	}
	
	// 清除用户所有仓位缓存（下次查询时会重新生成）
	userPositionsKey := fmt.Sprintf("defi:positions:%s", msg.UserAddress)
	w.consumer.redisClient.Del(ctx, userPositionsKey)
	
	return nil
}

// positionDataToJSON 将仓位数据转换为JSON字符串
func positionDataToJSON(position models.PositionData) string {
	data, _ := json.Marshal(position)
	return string(data)
}

// getRetryCount 获取消息重试次数
func (w *Worker) getRetryCount(message redis.XMessage) int {
	// 从消息的pending状态或自定义字段获取重试次数
	// 这里简化实现，实际应从Redis中存储的重试计数获取
	return 0
}

// retryMessage 重试消息
func (w *Worker) retryMessage(message redis.XMessage, msg *models.PositionUpdateMessage, err error, retryCount int) error {
	// 记录重试日志
	w.consumer.logger.Printf("Retrying message %s (attempt %d/%d)", message.ID, retryCount+1, w.consumer.config.MaxRetries)
	
	// 更新重试计数（在实际实现中，应在Redis中存储重试计数）
	// 这里简化实现，直接返回
	
	// 延迟重试
	time.Sleep(w.consumer.config.RetryDelay)
	
	return nil
}

// moveToDLQ 将消息移动到死信队列
func (w *Worker) moveToDLQ(message redis.XMessage, msg *models.PositionUpdateMessage, err error, retryCount int) error {
	ctx := context.Background()
	
	// 创建失败消息
	failedMsg := models.FailedMessage{
		OriginalMessageID: message.ID,
		OriginalStream:    w.consumer.config.StreamName,
		OriginalData:      *msg,
		ErrorReason:       "max_retries_exceeded",
		ErrorMessage:      err.Error(),
		RetryCount:        retryCount,
		FailedAt:          time.Now(),
		Metadata: map[string]interface{}{
			"worker_id": w.id,
		},
	}
	
	// 转换为JSON
	failedData, err := json.Marshal(failedMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal failed message: %w", err)
	}
	
	// 添加到死信队列
	_, err = w.consumer.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: w.consumer.config.DLQStreamName,
		Values: map[string]interface{}{
			"failed_message": string(failedData),
			"original_id":    message.ID,
			"error_reason":   "max_retries_exceeded",
			"failed_at":      time.Now().Format(time.RFC3339),
			"retry_count":    retryCount,
		},
	}).Result()
	
	if err != nil {
		return fmt.Errorf("failed to add to DLQ: %w", err)
	}
	
	w.consumer.metrics.mu.Lock()
	w.consumer.metrics.MessagesDLQ++
	w.consumer.metrics.mu.Unlock()
	
	w.consumer.logger.Printf("Moved message %s to DLQ after %d retries", message.ID, retryCount)
	return nil
}

// ackMessage 确认消息
func (w *Worker) ackMessage(message redis.XMessage) error {
	ctx := context.Background()
	
	err := w.consumer.redisClient.XAck(ctx, w.consumer.config.StreamName, w.consumer.config.ConsumerGroup, message.ID).Err()
	if err != nil {
		return fmt.Errorf("failed to ack message: %w", err)
	}
	
	return nil
}

// GetMetrics 获取监控指标
func (c *Consumer) GetMetrics() *ConsumerMetrics {
	c.metrics.mu.RLock()
	defer c.metrics.mu.RUnlock()
	
	// 返回副本
	return &ConsumerMetrics{
		MessagesProcessed: c.metrics.MessagesProcessed,
		MessagesFailed:    c.metrics.MessagesFailed,
		MessagesRetried:   c.metrics.MessagesRetried,
		MessagesDLQ:       c.metrics.MessagesDLQ,
		LastProcessedTime: c.metrics.LastProcessedTime,
		LastErrorTime:     c.metrics.Last