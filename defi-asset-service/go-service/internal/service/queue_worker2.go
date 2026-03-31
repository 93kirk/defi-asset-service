package service

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// moveToDeadLetterQueue 移动到死信队列（续）
func (w *queueWorker) moveToDeadLetterQueue(ctx context.Context, message redis.XMessage) error {
	deadLetterStream := fmt.Sprintf("dlq:%s", w.config.PositionUpdates.StreamName)
	
	values := map[string]interface{}{
		"original_message_id": message.ID,
		"original_values":     message.Values,
		"failed_at":           time.Now().Unix(),
		"reason":             "max_retries_exceeded",
	}
	
	_, err := w.redisRepo.AddToStream(ctx, deadLetterStream, values)
	if err != nil {
		return fmt.Errorf("failed to add to dead letter queue: %w", err)
	}
	
	w.logger.Warnf("Moved message %s to dead letter queue (max retries exceeded)", message.ID)
	return nil
}

// retryMessage 重试消息
func (w *queueWorker) retryMessage(ctx context.Context, message redis.XMessage, delay time.Duration) error {
	// 创建延迟任务来重试消息
	task := map[string]interface{}{
		"task_type": "retry_position_update",
		"message":   message.Values,
		"retry_at":  time.Now().Add(delay).Unix(),
	}
	
	// 添加到延迟队列
	score := time.Now().Add(delay).Unix()
	if err := w.redisRepo.AddToDelayedQueue(ctx, w.config.DelayedTasks.ZSetName, score, task); err != nil {
		return fmt.Errorf("failed to add retry task: %w", err)
	}
	
	w.logger.Infof("Scheduled retry for message %s after %v (retry count: %s)", 
		message.ID, delay, message.Values["retry_count"])
	
	return nil
}

// checkConsumerHealth 检查消费者健康状态
func (w *queueWorker) checkConsumerHealth(ctx context.Context) {
	streamKey := fmt.Sprintf("%s:stream:%s", w.redisRepo.(*redisRepository).prefix, w.config.PositionUpdates.StreamName)
	
	// 获取消费者信息
	consumers, err := w.redisRepo.(*redisRepository).client.XInfoConsumers(ctx, streamKey, w.config.PositionUpdates.ConsumerGroup).Result()
	if err != nil {
		w.logger.WithError(err).Error("Failed to get consumer info")
		return
	}
	
	// 检查每个消费者的空闲时间
	now := time.Now()
	for _, consumer := range consumers {
		idleTime := time.Duration(consumer.Idle) * time.Millisecond
		
		// 如果消费者空闲时间过长，可能已经死亡
		if idleTime > 5*time.Minute {
			w.logger.Warnf("Consumer %s has been idle for %v", consumer.Name, idleTime)
			
			// 尝试恢复：删除死亡消费者并重新分配待处理消息
			if err := w.recoverDeadConsumer(ctx, consumer.Name); err != nil {
				w.logger.WithError(err).Errorf("Failed to recover dead consumer %s", consumer.Name)
			}
		}
	}
}

// recoverDeadConsumer 恢复死亡消费者
func (w *queueWorker) recoverDeadConsumer(ctx context.Context, consumerName string) error {
	streamKey := fmt.Sprintf("%s:stream:%s", w.redisRepo.(*redisRepository).prefix, w.config.PositionUpdates.StreamName)
	
	// 获取消费者的待处理消息
	pending, err := w.redisRepo.(*redisRepository).client.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: streamKey,
		Group:  w.config.PositionUpdates.ConsumerGroup,
		Start:  "-",
		End:    "+",
		Count:  100,
		Consumer: consumerName,
	}).Result()
	
	if err != nil {
		return fmt.Errorf("failed to get pending messages: %w", err)
	}
	
	// 将待处理消息重新分配给其他消费者
	for _, msg := range pending {
		// 认领消息
		claimed, err := w.redisRepo.(*redisRepository).client.XClaim(ctx, &redis.XClaimArgs{
			Stream:   streamKey,
			Group:    w.config.PositionUpdates.ConsumerGroup,
			Consumer: w.workerID,
			MinIdle:  5 * time.Minute,
			Messages: []string{msg.ID},
		}).Result()
		
		if err != nil {
			w.logger.WithError(err).Errorf("Failed to claim message %s", msg.ID)
			continue
		}
		
		if len(claimed) > 0 {
			w.logger.Infof("Claimed message %s from dead consumer %s", msg.ID, consumerName)
			
			// 处理认领的消息
			positionUpdate, err := w.parsePositionUpdateMessage(claimed[0])
			if err != nil {
				w.logger.WithError(err).Errorf("Failed to parse claimed message %s", msg.ID)
				continue
			}
			
			// 处理仓位更新
			if err := w.positionSvc.ProcessPositionUpdate(ctx, positionUpdate); err != nil {
				w.logger.WithError(err).Errorf("Failed to process claimed message %s", msg.ID)
				continue
			}
			
			// 确认消息
			if err := w.redisRepo.AckMessage(ctx, w.config.PositionUpdates.StreamName, w.config.PositionUpdates.ConsumerGroup, msg.ID); err != nil {
				w.logger.WithError(err).Errorf("Failed to ack claimed message %s", msg.ID)
			}
		}
	}
	
	// 删除死亡消费者
	if err := w.redisRepo.(*redisRepository).client.XGroupDelConsumer(ctx, streamKey, w.config.PositionUpdates.ConsumerGroup, consumerName).Err(); err != nil {
		return fmt.Errorf("failed to delete dead consumer: %w", err)
	}
	
	w.logger.Infof("Recovered dead consumer %s and reassigned %d messages", consumerName, len(pending))
	return nil
}