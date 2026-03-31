package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"defi-asset-service/internal/config"
	"defi-asset-service/internal/model"
	"defi-asset-service/internal/repository"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

// QueueWorker 队列处理Worker接口
type QueueWorker interface {
	Start(ctx context.Context) error
	Stop() error
	ProcessPositionUpdates(ctx context.Context) error
	ProcessDelayedTasks(ctx context.Context) error
	AddPositionUpdate(ctx context.Context, message *model.PositionUpdateMessage) error
	AddDelayedTask(ctx context.Context, taskType string, data interface{}, delay time.Duration) error
	GetQueueStats(ctx context.Context) (*model.MetricsResponse, error)
}

// queueWorker 队列处理Worker实现
type queueWorker struct {
	config        *config.QueueConfig
	redisRepo     repository.RedisRepository
	positionSvc   ServiceBService
	logger        *logrus.Logger
	stopChan      chan struct{}
	running       bool
	workerID      string
}

// NewQueueWorker 创建队列处理Worker
func NewQueueWorker(
	config *config.QueueConfig,
	redisRepo repository.RedisRepository,
	positionSvc ServiceBService,
	logger *logrus.Logger,
) QueueWorker {
	workerID := fmt.Sprintf("worker_%d", time.Now().Unix())
	
	return &queueWorker{
		config:      config,
		redisRepo:   redisRepo,
		positionSvc: positionSvc,
		logger:      logger,
		stopChan:    make(chan struct{}),
		running:     false,
		workerID:    workerID,
	}
}

// Start 启动Worker
func (w *queueWorker) Start(ctx context.Context) error {
	if w.running {
		return fmt.Errorf("worker is already running")
	}
	
	w.running = true
	w.logger.Info("Starting queue worker")
	
	// 启动仓位更新处理协程
	go w.processPositionUpdatesLoop(ctx)
	
	// 启动延迟任务处理协程
	go w.processDelayedTasksLoop(ctx)
	
	// 启动监控协程
	go w.monitorLoop(ctx)
	
	return nil
}

// Stop 停止Worker
func (w *queueWorker) Stop() error {
	if !w.running {
		return fmt.Errorf("worker is not running")
	}
	
	w.logger.Info("Stopping queue worker")
	close(w.stopChan)
	w.running = false
	
	return nil
}

// ProcessPositionUpdates 处理仓位更新消息
func (w *queueWorker) ProcessPositionUpdates(ctx context.Context) error {
	streams, err := w.redisRepo.ReadFromStream(
		ctx,
		w.config.PositionUpdates.StreamName,
		w.config.PositionUpdates.ConsumerGroup,
		w.workerID,
		w.config.PositionUpdates.BatchSize,
		time.Duration(w.config.PositionUpdates.BlockTime)*time.Millisecond,
	)
	
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to read from stream: %w", err)
	}
	
	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil
	}
	
	var processedIDs []string
	var failedMessages []redis.XMessage
	
	for _, message := range streams[0].Messages {
		// 解析消息
		positionUpdate, err := w.parsePositionUpdateMessage(message)
		if err != nil {
			w.logger.WithError(err).Error("Failed to parse position update message")
			failedMessages = append(failedMessages, message)
			continue
		}
		
		// 处理仓位更新
		if err := w.positionSvc.ProcessPositionUpdate(ctx, positionUpdate); err != nil {
			w.logger.WithError(err).Error("Failed to process position update")
			failedMessages = append(failedMessages, message)
			continue
		}
		
		processedIDs = append(processedIDs, message.ID)
		w.logger.Infof("Processed position update: %s for user %s", positionUpdate.EventID, positionUpdate.UserAddress)
	}
	
	// 确认已处理的消息
	if len(processedIDs) > 0 {
		if err := w.redisRepo.AckMessage(ctx, w.config.PositionUpdates.StreamName, w.config.PositionUpdates.ConsumerGroup, processedIDs...); err != nil {
			w.logger.WithError(err).Error("Failed to ack messages")
		}
	}
	
	// 处理失败的消息
	for _, message := range failedMessages {
		if err := w.handleFailedMessage(ctx, message); err != nil {
			w.logger.WithError(err).Error("Failed to handle failed message")
		}
	}
	
	return nil
}

// ProcessDelayedTasks 处理延迟任务
func (w *queueWorker) ProcessDelayedTasks(ctx context.Context) error {
	now := time.Now().Unix()
	
	// 获取到期的任务
	tasks, err := w.redisRepo.GetFromDelayedQueue(
		ctx,
		w.config.DelayedTasks.ZSetName,
		now,
		10, // 每次处理10个任务
	)
	
	if err != nil {
		return fmt.Errorf("failed to get delayed tasks: %w", err)
	}
	
	if len(tasks) == 0 {
		return nil
	}
	
	var processedTasks []interface{}
	
	for _, task := range tasks {
		// 解析任务数据
		taskData, err := w.parseDelayedTask(task)
		if err != nil {
			w.logger.WithError(err).Error("Failed to parse delayed task")
			continue
		}
		
		// 处理任务
		if err := w.processDelayedTask(ctx, taskData); err != nil {
			w.logger.WithError(err).Error("Failed to process delayed task")
			continue
		}
		
		processedTasks = append(processedTasks, task.Member)
		w.logger.Infof("Processed delayed task: %v", taskData)
	}
	
	// 从队列中移除已处理的任务
	if len(processedTasks) > 0 {
		if err := w.redisRepo.RemoveFromDelayedQueue(ctx, w.config.DelayedTasks.ZSetName, processedTasks...); err != nil {
			w.logger.WithError(err).Error("Failed to remove processed tasks")
		}
	}
	
	return nil
}

// AddPositionUpdate 添加仓位更新消息到队列
func (w *queueWorker) AddPositionUpdate(ctx context.Context, message *model.PositionUpdateMessage) error {
	// 设置事件ID（如果未设置）
	if message.EventID == "" {
		message.EventID = fmt.Sprintf("evt_%d", time.Now().UnixNano())
	}
	
	// 设置时间戳（如果未设置）
	if message.Timestamp == 0 {
		message.Timestamp = time.Now().Unix()
	}
	
	// 转换为Redis消息格式
	values := map[string]interface{}{
		"event_id":      message.EventID,
		"event_type":    message.EventType,
		"user_address":  message.UserAddress,
		"protocol_id":   message.ProtocolID,
		"timestamp":     fmt.Sprintf("%d", message.Timestamp),
	}
	
	// 添加仓位数据
	positionDataBytes, err := json.Marshal(message.PositionData)
	if err != nil {
		return fmt.Errorf("failed to marshal position data: %w", err)
	}
	values["position_data"] = string(positionDataBytes)
	
	// 添加到Stream
	_, err = w.redisRepo.AddToStream(ctx, w.config.PositionUpdates.StreamName, values)
	if err != nil {
		return fmt.Errorf("failed to add to stream: %w", err)
	}
	
	w.logger.Infof("Added position update to queue: %s for user %s", message.EventID, message.UserAddress)
	return nil
}

// AddDelayedTask 添加延迟任务
func (w *queueWorker) AddDelayedTask(ctx context.Context, taskType string, data interface{}, delay time.Duration) error {
	task := map[string]interface{}{
		"task_id":   fmt.Sprintf("task_%d", time.Now().UnixNano()),
		"task_type": taskType,
		"data":      data,
		"created_at": time.Now().Unix(),
		"execute_at": time.Now().Add(delay).Unix(),
	}
	
	// 添加到延迟队列
	score := time.Now().Add(delay).Unix()
	if err := w.redisRepo.AddToDelayedQueue(ctx, w.config.DelayedTasks.ZSetName, score, task); err != nil {
		return fmt.Errorf("failed to add delayed task: %w", err)
	}
	
	w.logger.Infof("Added delayed task: %s (execute at: %v)", taskType, time.Unix(score, 0))
	return nil
}

// GetQueueStats 获取队列统计信息
func (w *queueWorker) GetQueueStats(ctx context.Context) (*model.MetricsResponse, error) {
	stats := &model.MetricsResponse{}
	
	// 获取Stream长度（待处理消息数）
	streamKey := fmt.Sprintf("%s:stream:%s", w.redisRepo.(*redisRepository).prefix, w.config.PositionUpdates.StreamName)
	streamLen, err := w.redisRepo.(*redisRepository).client.XLen(ctx, streamKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get stream length: %w", err)
	}
	
	// 获取消费者组信息
	groupInfo, err := w.redisRepo.(*redisRepository).client.XInfoGroups(ctx, streamKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get group info: %w", err)
	}
	
	var pendingCount int64
	var consumerCount int64
	
	for _, group := range groupInfo {
		if group.Name == w.config.PositionUpdates.ConsumerGroup {
			pendingCount = group.Pending
			consumerCount = group.Consumers
			break
		}
	}
	
	// 获取延迟队列长度
	zsetKey := fmt.Sprintf("%s:zset:%s", w.redisRepo.(*redisRepository).prefix, w.config.DelayedTasks.ZSetName)
	delayedCount, err := w.redisRepo.(*redisRepository).client.ZCard(ctx, zsetKey).Result()
	if err != nil {
		delayedCount = 0
	}
	
	stats.Queue.Pending = pendingCount
	stats.Queue.Processed = streamLen - pendingCount
	stats.Queue.Failed = 0 // 需要从其他来源获取失败计数
	
	return stats, nil
}

// processPositionUpdatesLoop 仓位更新处理循环
func (w *queueWorker) processPositionUpdatesLoop(ctx context.Context) {
	w.logger.Info("Starting position updates processing loop")
	
	for {
		select {
		case <-w.stopChan:
			w.logger.Info("Stopping position updates processing loop")
			return
		default:
			if err := w.ProcessPositionUpdates(ctx); err != nil {
				w.logger.WithError(err).Error("Failed to process position updates")
			}
			
			// 短暂休眠避免CPU占用过高
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// processDelayedTasksLoop 延迟任务处理循环
func (w *queueWorker) processDelayedTasksLoop(ctx context.Context) {
	w.logger.Info("Starting delayed tasks processing loop")
	
	ticker := time.NewTicker(time.Duration(w.config.DelayedTasks.PollInterval) * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-w.stopChan:
			w.logger.Info("Stopping delayed tasks processing loop")
			return
		case <-ticker.C:
			if err := w.ProcessDelayedTasks(ctx); err != nil {
				w.logger.WithError(err).Error("Failed to process delayed tasks")
			}
		}
	}
}

// monitorLoop 监控循环
func (w *queueWorker) monitorLoop(ctx context.Context) {
	w.logger.Info("Starting monitor loop")
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-w.stopChan:
			w.logger.Info("Stopping monitor loop")
			return
		case <-ticker.C:
			// 获取队列统计
			stats, err := w.GetQueueStats(ctx)
			if err != nil {
				w.logger.WithError(err).Error("Failed to get queue stats")
				continue
			}
			
			// 记录监控日志
			if stats.Queue.Pending > 100 {
				w.logger.Warnf("High queue backlog: %d pending messages", stats.Queue.Pending)
			}
			
			// 检查消费者健康状态
			w.checkConsumerHealth(ctx)
		}
	}
}

// parsePositionUpdateMessage 解析仓位更新消息
func (w *queueWorker) parsePositionUpdateMessage(message redis.XMessage) (*model.PositionUpdateMessage, error) {
	var positionUpdate model.PositionUpdateMessage
	
	// 解析基本字段
	if eventID, ok := message.Values["event_id"].(string); ok {
		positionUpdate.EventID = eventID
	}
	
	if eventType, ok := message.Values["event_type"].(string); ok {
		positionUpdate.EventType = eventType
	}
	
	if userAddress, ok := message.Values["user_address"].(string); ok {
		positionUpdate.UserAddress = userAddress
	}
	
	if protocolID, ok := message.Values["protocol_id"].(string); ok {
		positionUpdate.ProtocolID = protocolID
	}
	
	if timestampStr, ok := message.Values["timestamp"].(string); ok {
		if timestamp, err := time.ParseInt(timestampStr, 10, 64); err == nil {
			positionUpdate.Timestamp = timestamp
		}
	}
	
	// 解析仓位数据
	if positionDataStr, ok := message.Values["position_data"].(string); ok {
		var positionData map[string]interface{}
		if err := json.Unmarshal([]byte(positionDataStr), &positionData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal position data: %w", err)
		}
		positionUpdate.PositionData = positionData
	}
	
	// 验证必要字段
	if positionUpdate.EventID == "" || positionUpdate.UserAddress == "" || positionUpdate.ProtocolID == "" {
		return nil, fmt.Errorf("missing required fields in position update message")
	}
	
	return &positionUpdate, nil
}

// parseDelayedTask 解析延迟任务
func (w *queueWorker) parseDelayedTask(task redis.Z) (map[string]interface{}, error) {
	taskData := make(map[string]interface{})
	
	// 解析任务数据
	taskBytes, ok := task.Member.(string)
	if !ok {
		return nil, fmt.Errorf("invalid task data type")
	}
	
	if err := json.Unmarshal([]byte(taskBytes), &taskData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal task data: %w", err)
	}
	
	return taskData, nil
}

// processDelayedTask 处理延迟任务
func (w *queueWorker) processDelayedTask(ctx context.Context, taskData map[string]interface{}) error {
	taskType, ok := taskData["task_type"].(string)
	if !ok {
		return fmt.Errorf("missing task type")
	}
	
	switch taskType {
	case "cache_refresh":
		return w.processCacheRefreshTask(ctx, taskData)
	case "price_update":
		return w.processPriceUpdateTask(ctx, taskData)
	case "protocol_sync":
		return w.processProtocolSyncTask(ctx, taskData)
	default:
		return fmt.Errorf("unknown task type: %s", taskType)
	}
}

// processCacheRefreshTask 处理缓存刷新任务
func (w *queueWorker) processCacheRefreshTask(ctx context.Context, taskData map[string]interface{}) error {
	// 实现缓存刷新逻辑
	w.logger.Infof("Processing cache refresh task: %v", taskData)
	return nil
}

// processPriceUpdateTask 处理价格更新任务
func (w *queueWorker) processPriceUpdateTask(ctx context.Context, taskData map[string]interface{}) error {
	// 实现价格更新逻辑
	w.logger.Infof("Processing price update task: %v", taskData)
	return nil
}

// processProtocolSyncTask 处理协议同步任务
func (w *queueWorker) processProtocolSyncTask(ctx context.Context, taskData map[string]interface{}) error {
	// 实现协议同步逻辑
	w.logger.Infof("Processing protocol sync task: %v", taskData)
	return nil
}

// handleFailedMessage 处理失败的消息
func (w *queueWorker) handleFailedMessage(ctx context.Context, message redis.XMessage) error {
	// 获取重试次数
	retryCount := 0
	if retryCountStr, ok := message.Values["retry_count"].(string); ok {
		fmt.Sscanf(retryCountStr, "%d", &retryCount)
	}
	
	// 检查是否超过最大重试次数
	if retryCount >= w.config.PositionUpdates.MaxRetries {
		// 移动到死信队列
		return w.moveToDeadLetterQueue(ctx, message)
	}
	
	// 增加重试次数并重新入队
	retryCount++
	message.Values["retry_count"] = fmt.Sprintf("%d", retryCount)
	
	// 添加延迟后重新入队
	delay := time.Duration(w.config.PositionUpdates.RetryDelay) * time.Second
	return w.retryMessage(ctx, message, delay)
}

// moveToDeadLetterQueue 移动到死信队列
func (w *queueWorker) moveToDeadLetterQueue(ctx context.Context, message redis.XMessage) error {
	deadLetterStream := fmt.Sprintf("dlq:%s", w.config.PositionUpdates.StreamName)
	
	values := map[string]interface{}{
		"original_message_id": message.ID