package consumer

import (
	"context"
	"fmt"
	"log"
	"time"
)

// GetMetrics 获取监控指标（续）
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
		LastErrorTime:     c.metrics.LastErrorTime,
		ProcessingLatency: c.metrics.ProcessingLatency,
		WorkerCount:       c.metrics.WorkerCount,
	}
}

// HealthCheck 健康检查
func (c *Consumer) HealthCheck(ctx context.Context) error {
	// 检查Redis连接
	if err := c.redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis connection failed: %w", err)
	}
	
	// 检查数据库连接
	if err := c.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	
	// 检查消费者组状态
	groups, err := c.redisClient.XInfoGroups(ctx, c.config.StreamName).Result()
	if err != nil {
		return fmt.Errorf("failed to get consumer groups: %w", err)
	}
	
	groupFound := false
	for _, group := range groups {
		if group.Name == c.config.ConsumerGroup {
			groupFound = true
			
			// 检查pending消息数量
			if group.Pending > 1000 {
				c.logger.Printf("Warning: consumer group %s has %d pending messages", group.Name, group.Pending)
			}
			break
		}
	}
	
	if !groupFound {
		return fmt.Errorf("consumer group %s not found", c.config.ConsumerGroup)
	}
	
	return nil
}

// GetPendingMessages 获取待处理消息
func (c *Consumer) GetPendingMessages(ctx context.Context) ([]redis.XPendingExt, error) {
	// 获取pending消息
	pending, err := c.redisClient.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream:   c.config.StreamName,
		Group:    c.config.ConsumerGroup,
		Start:    "-",
		End:      "+",
		Count:    100,
		Consumer: "",
	}).Result()
	
	if err != nil {
		return nil, fmt.Errorf("failed to get pending messages: %w", err)
	}
	
	return pending, nil
}

// ClaimPendingMessages 认领pending消息
func (c *Consumer) ClaimPendingMessages(ctx context.Context, minIdleTime time.Duration, count int64) ([]redis.XMessage, error) {
	// 认领长时间未处理的消息
	messages, err := c.redisClient.XClaim(ctx, &redis.XClaimArgs{
		Stream:   c.config.StreamName,
		Group:    c.config.ConsumerGroup,
		Consumer: c.config.ConsumerName,
		MinIdle:  minIdleTime,
		Messages: []string{"0-0"}, // 从最早的消息开始
	}).Result()
	
	if err != nil {
		return nil, fmt.Errorf("failed to claim pending messages: %w", err)
	}
	
	return messages, nil
}

// CleanupStaleMessages 清理陈旧消息
func (c *Consumer) CleanupStaleMessages(ctx context.Context, maxAge time.Duration) (int64, error) {
	// 获取Stream信息
	streamInfo, err := c.redisClient.XInfoStream(ctx, c.config.StreamName).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get stream info: %w", err)
	}
	
	// 计算最早允许的时间戳
	minTime := time.Now().Add(-maxAge).Unix() * 1000
	
	// 删除旧消息
	deleted, err := c.redisClient.XTrimMinID(ctx, c.config.StreamName, fmt.Sprintf("%d-0", minTime)).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to trim stream: %w", err)
	}
	
	c.logger.Printf("Cleaned up %d stale messages older than %v", deleted, maxAge)
	return deleted, nil
}

// GetDLQStats 获取死信队列统计
func (c *Consumer) GetDLQStats(ctx context.Context) (*DLQStats, error) {
	// 获取死信队列长度
	dlqLen, err := c.redisClient.XLen(ctx, c.config.DLQStreamName).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get DLQ length: %w", err)
	}
	
	// 获取死信队列消息
	var dlqMessages []redis.XMessage
	if dlqLen > 0 {
		messages, err := c.redisClient.XRange(ctx, c.config.DLQStreamName, "-", "+").Result()
		if err != nil {
			return nil, fmt.Errorf("failed to get DLQ messages: %w", err)
		}
		dlqMessages = messages
	}
	
	// 按错误原因统计
	errorStats := make(map[string]int)
	for _, msg := range dlqMessages {
		if reason, ok := msg.Values["error_reason"].(string); ok {
			errorStats[reason]++
		}
	}
	
	return &DLQStats{
		MessageCount: dlqLen,
		Messages:     dlqMessages,
		ErrorStats:   errorStats,
		LastUpdated:  time.Now(),
	}, nil
}

// DLQStats 死信队列统计
type DLQStats struct {
	MessageCount int64
	Messages     []redis.XMessage
	ErrorStats   map[string]int
	LastUpdated  time.Time
}

// RetryDLQMessages 重试死信队列消息
func (c *Consumer) RetryDLQMessages(ctx context.Context, count int64) (int64, error) {
	// 从死信队列读取消息
	messages, err := c.redisClient.XRange(ctx, c.config.DLQStreamName, "-", "+").Result()
	if err != nil && err != redis.Nil {
		return 0, fmt.Errorf("failed to read DLQ messages: %w", err)
	}
	
	if len(messages) == 0 {
		return 0, nil
	}
	
	// 限制重试数量
	if int64(len(messages)) > count {
		messages = messages[:count]
	}
	
	retried := int64(0)
	
	// 重新发布到主队列
	for _, msg := range messages {
		// 从失败消息中提取原始数据
		if failedData, ok := msg.Values["failed_message"].(string); ok {
			// 重新发布到主队列
			_, err := c.redisClient.XAdd(ctx, &redis.XAddArgs{
				Stream: c.config.StreamName,
				Values: map[string]interface{}{
					"retry_from_dlq": true,
					"original_dlq_id": msg.ID,
					"failed_message": failedData,
					"retry_count":    1,
					"retry_time":     time.Now().Format(time.RFC3339),
				},
			}).Result()
			
			if err != nil {
				c.logger.Printf("Failed to retry DLQ message %s: %v", msg.ID, err)
				continue
			}
			
			// 从死信队列删除
			if err := c.redisClient.XDel(ctx, c.config.DLQStreamName, msg.ID).Err(); err != nil {
				c.logger.Printf("Failed to delete DLQ message %s: %v", msg.ID, err)
			}
			
			retried++
		}
	}
	
	c.logger.Printf("Retried %d messages from DLQ", retried)
	return retried, nil
}

// ExampleUsage 示例使用
func ExampleUsage() {
	// 创建Redis客户端
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       3,
	})
	
	// 创建数据库连接
	db, err := sql.Open("mysql", "root:@tcp(localhost:3306)/defi_asset_service?charset=utf8mb4&parseTime=true")
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	
	// 创建配置
	cfg := &config.QueueConfig{
		StreamName:    "defi:stream:position_updates",
		ConsumerGroup: "position_workers",
		ConsumerName:  "worker",
		DLQStreamName: "defi:stream:dlq:position_updates",
		MaxRetries:    3,
		RetryDelay:    30 * time.Second,
		BatchSize:     10,
		BlockTimeout:  5 * time.Second,
		AutoAck:       false,
		Workers:       5,
	}
	
	// 创建消费者
	consumer, err := NewConsumer(redisClient, db, cfg, log.Default())
	if err != nil {
		log.Fatalf("Failed to create consumer: %v", err)
	}
	
	// 启动消费者
	ctx := context.Background()
	if err := consumer.Start(ctx); err != nil {
		log.Fatalf("Failed to start consumer: %v", err)
	}
	
	// 等待信号
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	
	<-signalChan
	
	// 优雅关闭
	log.Println("Shutting down consumer...")
	if err := consumer.Stop(); err != nil {
		log.Printf("Error stopping consumer: %v", err)
	}
	
	log.Println("Consumer stopped")
}