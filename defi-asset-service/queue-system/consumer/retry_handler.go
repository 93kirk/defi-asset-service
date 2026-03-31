package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"

	"defi-asset-service/queue-system/models"
)

// RetryHandler 重试处理器
type RetryHandler struct {
	redisClient *redis.Client
	config      *QueueConfig
	logger      *log.Logger
}

// QueueConfig 队列配置（简化版）
type QueueConfig struct {
	StreamName    string
	DLQStreamName string
	MaxRetries    int
	RetryDelay    time.Duration
}

// NewRetryHandler 创建新的重试处理器
func NewRetryHandler(redisClient *redis.Client, config *QueueConfig, logger *log.Logger) *RetryHandler {
	return &RetryHandler{
		redisClient: redisClient,
		config:      config,
		logger:      logger,
	}
}

// HandleRetry 处理消息重试
func (h *RetryHandler) HandleRetry(ctx context.Context, message redis.XMessage, msg *models.PositionUpdateMessage, err error) error {
	// 获取当前重试次数
	retryCount := h.getRetryCount(ctx, message.ID)
	
	if retryCount >= h.config.MaxRetries {
		// 达到最大重试次数，移动到死信队列
		return h.moveToDLQ(ctx, message, msg, err, retryCount)
	}
	
	// 执行重试
	return h.retryMessage(ctx, message, msg, err, retryCount)
}

// getRetryCount 获取消息重试次数
func (h *RetryHandler) getRetryCount(ctx context.Context, messageID string) int {
	// 从Redis中获取重试计数
	retryKey := fmt.Sprintf("defi:retry:%s", messageID)
	
	countStr, err := h.redisClient.Get(ctx, retryKey).Result()
	if err != nil {
		if err == redis.Nil {
			return 0 // 第一次重试
		}
		h.logger.Printf("Failed to get retry count for message %s: %v", messageID, err)
		return 0
	}
	
	var count int
	fmt.Sscanf(countStr, "%d", &count)
	return count
}

// incrementRetryCount 增加重试计数
func (h *RetryHandler) incrementRetryCount(ctx context.Context, messageID string) (int, error) {
	retryKey := fmt.Sprintf("defi:retry:%s", messageID)
	
	// 使用INCR命令原子增加计数
	count, err := h.redisClient.Incr(ctx, retryKey).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment retry count: %w", err)
	}
	
	// 设置过期时间（24小时）
	if err := h.redisClient.Expire(ctx, retryKey, 24*time.Hour).Err(); err != nil {
		h.logger.Printf("Failed to set retry key expiration: %v", err)
	}
	
	return int(count), nil
}

// retryMessage 重试消息
func (h *RetryHandler) retryMessage(ctx context.Context, message redis.XMessage, msg *models.PositionUpdateMessage, err error, retryCount int) error {
	// 增加重试计数
	newRetryCount, err := h.incrementRetryCount(ctx, message.ID)
	if err != nil {
		return fmt.Errorf("failed to increment retry count: %w", err)
	}
	
	// 记录重试日志
	h.logger.Printf("Retrying message %s (attempt %d/%d): %v", message.ID, newRetryCount, h.config.MaxRetries, err)
	
	// 创建重试消息
	retryMsg := h.createRetryMessage(message, msg, err, newRetryCount)
	
	// 延迟重试
	delay := h.calculateRetryDelay(newRetryCount)
	
	// 使用延迟队列或定时任务
	if err := h.scheduleRetry(ctx, retryMsg, delay); err != nil {
		return fmt.Errorf("failed to schedule retry: %w", err)
	}
	
	return nil
}

// createRetryMessage 创建重试消息
func (h *RetryHandler) createRetryMessage(message redis.XMessage, msg *models.PositionUpdateMessage, err error, retryCount int) *RetryMessage {
	return &RetryMessage{
		OriginalMessageID: message.ID,
		OriginalData:      *msg,
		ErrorReason:       getErrorReason(err),
		ErrorMessage:      err.Error(),
		RetryCount:        retryCount,
		ScheduledAt:       time.Now(),
		NextRetryAt:       time.Now().Add(h.calculateRetryDelay(retryCount)),
		Metadata: map[string]interface{}{
			"original_stream": h.config.StreamName,
			"error_type":      getErrorType(err),
		},
	}
}

// calculateRetryDelay 计算重试延迟
func (h *RetryHandler) calculateRetryDelay(retryCount int) time.Duration {
	// 指数退避算法
	baseDelay := h.config.RetryDelay
	maxDelay := 10 * time.Minute
	
	delay := baseDelay * time.Duration(1<<uint(retryCount-1)) // 2^(retryCount-1) * baseDelay
	
	// 添加随机抖动避免惊群效应
	jitter := time.Duration(float64(delay) * 0.1) // 10%抖动
	delay += time.Duration(float64(jitter) * (2*rand.Float64() - 1))
	
	// 限制最大延迟
	if delay > maxDelay {
		delay = maxDelay
	}
	
	return delay
}

// scheduleRetry 调度重试
func (h *RetryHandler) scheduleRetry(ctx context.Context, retryMsg *RetryMessage, delay time.Duration) error {
	// 使用Redis Sorted Set实现延迟队列
	retryKey := "defi:zset:retry_queue"
	
	// 计算执行时间
	executeAt := time.Now().Add(delay)
	score := float64(executeAt.Unix())
	
	// 序列化重试消息
	retryData, err := json.Marshal(retryMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal retry message: %w", err)
	}
	
	// 添加到Sorted Set
	member := fmt.Sprintf("%s:%s", retryMsg.OriginalMessageID, string(retryData))
	if err := h.redisClient.ZAdd(ctx, retryKey, &redis.Z{
		Score:  score,
		Member: member,
	}).Err(); err != nil {
		return fmt.Errorf("failed to add to retry queue: %w", err)
	}
	
	return nil
}

// moveToDLQ 移动到死信队列
func (h *RetryHandler) moveToDLQ(ctx context.Context, message redis.XMessage, msg *models.PositionUpdateMessage, err error, retryCount int) error {
	// 创建失败消息
	failedMsg := models.FailedMessage{
		OriginalMessageID: message.ID,
		OriginalStream:    h.config.StreamName,
		OriginalData:      *msg,
		ErrorReason:       "max_retries_exceeded",
		ErrorMessage:      err.Error(),
		RetryCount:        retryCount,
		FailedAt:          time.Now(),
		Metadata: map[string]interface{}{
			"max_retries": h.config.MaxRetries,
			"final_error": err.Error(),
		},
	}
	
	// 序列化失败消息
	failedData, err := json.Marshal(failedMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal failed message: %w", err)
	}
	
	// 添加到死信队列
	_, err = h.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: h.config.DLQStreamName,
		Values: map[string]interface{}{
			"failed_message":   string(failedData),
			"original_id":      message.ID,
			"error_reason":     "max_retries_exceeded",
			"failed_at":        time.Now().Format(time.RFC3339),
			"retry_count":      retryCount,
			"max_retries":      h.config.MaxRetries,
			"original_stream":  h.config.StreamName,
		},
	}).Result()
	
	if err != nil {
		return fmt.Errorf("failed to add to DLQ: %w", err)
	}
	
	h.logger.Printf("Moved message %s to DLQ after %d retries", message.ID, retryCount)
	
	// 清理重试计数
	retryKey := fmt.Sprintf("defi:retry:%s", message.ID)
	h.redisClient.Del(ctx, retryKey)
	
	return nil
}

// ProcessRetryQueue 处理重试队列
func (h *RetryHandler) ProcessRetryQueue(ctx context.Context) error {
	retryKey := "defi:zset:retry_queue"
	
	// 获取到期的重试任务
	now := float64(time.Now().Unix())
	members, err := h.redisClient.ZRangeByScore(ctx, retryKey, &redis.ZRangeBy{
		Min: "0",
		Max: fmt.Sprintf("%f", now),
	}).Result()
	
	if err != nil {
		return fmt.Errorf("failed to get retry tasks: %w", err)
	}
	
	if len(members) == 0 {
		return nil
	}
	
	// 处理每个重试任务
	for _, member := range members {
		if err := h.processRetryTask(ctx, member); err != nil {
			h.logger.Printf("Failed to process retry task %s: %v", member, err)
			continue
		}
		
		// 从重试队列删除
		if err := h.redisClient.ZRem(ctx, retryKey, member).Err(); err != nil {
			h.logger.Printf("Failed to remove retry task %s: %v", member, err)
		}
	}
	
	return nil
}

// processRetryTask 处理单个重试任务
func (h *RetryHandler) processRetryTask(ctx context.Context, member string) error {
	// 解析重试任务
	parts := splitRetryMember(member)
	if len(parts) < 2 {
		return fmt.Errorf("invalid retry task format: %s", member)
	}
	
	messageID := parts[0]
	retryData := parts[1]
	
	// 解析重试消息
	var retryMsg RetryMessage
	if err := json.Unmarshal([]byte(retryData), &retryMsg); err != nil {
		return fmt.Errorf("failed to unmarshal retry message: %w", err)
	}
	
	// 重新发布到主队列
	_, err := h.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: h.config.StreamName,
		Values: map[string]interface{}{
			"retry":           true,
			"original_id":     messageID,
			"retry_count":     retryMsg.RetryCount,
			"position_data":   retryMsg.OriginalData.ToJSON(),
			"retry_timestamp": time.Now().Unix(),
		},
	}).Result()
	
	if err != nil {
		return fmt.Errorf("failed to republish retry message: %w", err)
	}
	
	h.logger.Printf("Republished retry message %s (attempt %d)", messageID, retryMsg.RetryCount)
	return nil
}

// splitRetryMember 分割重试任务成员
func splitRetryMember(member string) []string {
	// 格式: messageID:jsonData
	for i := 0; i < len(member); i++ {
		if member[i] == ':' {
			return []string{member[:i], member[i+1:]}
		}
	}
	return []string{member}
}

// getErrorReason 获取错误原因
func getErrorReason(err error) string {
	// 根据错误类型返回原因
	// 这里简化实现，实际应根据具体错误判断
	return "processing_failed"
}

// getErrorType 获取错误类型
func getErrorType(err error) string {
	// 根据错误类型返回分类
	// 这里简化实现
	return "unknown"
}

// RetryMessage 重试消息
type RetryMessage struct {
	OriginalMessageID string                     `json:"original_message_id"`
	OriginalData      models.PositionUpdateMessage `json:"original_data"`
	ErrorReason       string                     `json:"error_reason"`
	ErrorMessage      string                     `json:"error_message"`
	RetryCount        int                        `json:"retry_count"`
	ScheduledAt       time.Time                  `json:"scheduled_at"`
	NextRetryAt       time.Time                  `json:"next_retry_at"`
	Metadata          map[string]interface{}     `json:"metadata,omitempty"`
}

// StartRetryProcessor 启动重试处理器
func (h *RetryHandler) StartRetryProcessor(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			h.logger.Println("Retry processor stopped")
			return
		case <-ticker.C:
			if err := h.ProcessRetryQueue(ctx); err != nil {
				h.logger.Printf("Failed to process retry queue: %v", err)
			}
		}
	}
}