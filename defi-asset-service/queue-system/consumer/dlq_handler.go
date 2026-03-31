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

// DLQHandler 死信队列处理器
type DLQHandler struct {
	redisClient *redis.Client
	streamName  string
	logger      *log.Logger
}

// NewDLQHandler 创建新的死信队列处理器
func NewDLQHandler(redisClient *redis.Client, streamName string, logger *log.Logger) *DLQHandler {
	return &DLQHandler{
		redisClient: redisClient,
		streamName:  streamName,
		logger:      logger,
	}
}

// DLQStats 死信队列统计
type DLQStats struct {
	TotalMessages   int64                  `json:"total_messages"`
	ErrorStats      map[string]int         `json:"error_stats"`
	RecentMessages  []DLQMessageSummary    `json:"recent_messages"`
	OldestMessage   *time.Time             `json:"oldest_message,omitempty"`
	NewestMessage   *time.Time             `json:"newest_message,omitempty"`
	LastAnalyzed    time.Time              `json:"last_analyzed"`
}

// DLQMessageSummary 死信队列消息摘要
type DLQMessageSummary struct {
	MessageID      string    `json:"message_id"`
	OriginalID     string    `json:"original_id,omitempty"`
	ErrorReason    string    `json:"error_reason"`
	FailedAt       time.Time `json:"failed_at"`
	RetryCount     int       `json:"retry_count,omitempty"`
	UserAddress    string    `json:"user_address,omitempty"`
	ProtocolID     string    `json:"protocol_id,omitempty"`
}

// GetStats 获取死信队列统计
func (h *DLQHandler) GetStats(ctx context.Context) (*DLQStats, error) {
	// 获取队列长度
	messageCount, err := h.redisClient.XLen(ctx, h.streamName).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get DLQ length: %w", err)
	}
	
	if messageCount == 0 {
		return &DLQStats{
			TotalMessages:  0,
			ErrorStats:     make(map[string]int),
			RecentMessages: []DLQMessageSummary{},
			LastAnalyzed:   time.Now(),
		}, nil
	}
	
	// 获取所有消息
	messages, err := h.redisClient.XRange(ctx, h.streamName, "-", "+").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get DLQ messages: %w", err)
	}
	
	// 分析消息
	stats := h.analyzeMessages(messages)
	stats.TotalMessages = messageCount
	stats.LastAnalyzed = time.Now()
	
	return stats, nil
}

// analyzeMessages 分析消息
func (h *DLQHandler) analyzeMessages(messages []redis.XMessage) *DLQStats {
	stats := &DLQStats{
		ErrorStats:     make(map[string]int),
		RecentMessages: make([]DLQMessageSummary, 0, len(messages)),
	}
	
	var oldestTime, newestTime *time.Time
	
	for _, msg := range messages {
		// 提取错误原因
		errorReason := "unknown"
		if reason, ok := msg.Values["error_reason"].(string); ok {
			errorReason = reason
		}
		
		// 统计错误原因
		stats.ErrorStats[errorReason]++
		
		// 提取失败时间
		var failedAt time.Time
		if failedAtStr, ok := msg.Values["failed_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, failedAtStr); err == nil {
				failedAt = t
				
				// 更新最早和最晚时间
				if oldestTime == nil || t.Before(*oldestTime) {
					oldestTime = &t
				}
				if newestTime == nil || t.After(*newestTime) {
					newestTime = &t
				}
			}
		}
		
		if failedAt.IsZero() {
			failedAt = time.Now()
		}
		
		// 提取重试次数
		retryCount := 0
		if countStr, ok := msg.Values["retry_count"].(string); ok {
			fmt.Sscanf(countStr, "%d", &retryCount)
		}
		
		// 提取用户地址和协议ID
		userAddress := ""
		protocolID := ""
		
		// 尝试从failed_message中提取
		if failedData, ok := msg.Values["failed_message"].(string); ok {
			var failedMsg models.FailedMessage
			if err := json.Unmarshal([]byte(failedData), &failedMsg); err == nil {
				userAddress = failedMsg.OriginalData.UserAddress
				protocolID = failedMsg.OriginalData.ProtocolID
			}
		}
		
		// 创建消息摘要
		summary := DLQMessageSummary{
			MessageID:   msg.ID,
			OriginalID:  extractString(msg.Values, "original_id"),
			ErrorReason: errorReason,
			FailedAt:    failedAt,
			RetryCount:  retryCount,
			UserAddress: userAddress,
			ProtocolID:  protocolID,
		}
		
		stats.RecentMessages = append(stats.RecentMessages, summary)
	}
	
	stats.OldestMessage = oldestTime
	stats.NewestMessage = newestTime
	
	return stats
}

// extractString 从map中提取字符串
func extractString(values map[string]interface{}, key string) string {
	if val, ok := values[key].(string); ok {
		return val
	}
	return ""
}

// GetMessageDetail 获取消息详情
func (h *DLQHandler) GetMessageDetail(ctx context.Context, messageID string) (*DLQMessageDetail, error) {
	// 获取消息
	messages, err := h.redisClient.XRange(ctx, h.streamName, messageID, messageID).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get message: %w", err)
	}
	
	if len(messages) == 0 {
		return nil, fmt.Errorf("message not found: %s", messageID)
	}
	
	msg := messages[0]
	return h.parseMessageDetail(msg), nil
}

// parseMessageDetail 解析消息详情
func (h *DLQHandler) parseMessageDetail(msg redis.XMessage) *DLQMessageDetail {
	detail := &DLQMessageDetail{
		MessageID:   msg.ID,
		Values:      make(map[string]interface{}),
	}
	
	// 复制原始值
	for k, v := range msg.Values {
		detail.Values[k] = v
	}
	
	// 解析失败消息
	if failedData, ok := msg.Values["failed_message"].(string); ok {
		var failedMsg models.FailedMessage
		if err := json.Unmarshal([]byte(failedData), &failedMsg); err == nil {
			detail.FailedMessage = &failedMsg
		}
	}
	
	// 提取时间信息
	if failedAtStr, ok := msg.Values["failed_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, failedAtStr); err == nil {
			detail.FailedAt = t
		}
	}
	
	// 提取错误原因
	if reason, ok := msg.Values["error_reason"].(string); ok {
		detail.ErrorReason = reason
	}
	
	// 提取重试次数
	if countStr, ok := msg.Values["retry_count"].(string); ok {
		fmt.Sscanf(countStr, "%d", &detail.RetryCount)
	}
	
	return detail
}

// DLQMessageDetail 死信队列消息详情
type DLQMessageDetail struct {
	MessageID     string                     `json:"message_id"`
	FailedAt      time.Time                  `json:"failed_at,omitempty"`
	ErrorReason   string                     `json:"error_reason"`
	RetryCount    int                        `json:"retry_count"`
	FailedMessage *models.FailedMessage      `json:"failed_message,omitempty"`
	Values        map[string]interface{}     `json:"values"`
}

// RetryMessage 重试死信队列消息
func (h *DLQHandler) RetryMessage(ctx context.Context, messageID string) error {
	// 获取消息
	messages, err := h.redisClient.XRange(ctx, h.streamName, messageID, messageID).Result()
	if err != nil {
		return fmt.Errorf("failed to get message: %w", err)
	}
	
	if len(messages) == 0 {
		return fmt.Errorf("message not found: %s", messageID)
	}
	
	msg := messages[0]
	
	// 从失败消息中提取原始数据
	var originalData *models.PositionUpdateMessage
	var failedMsg models.FailedMessage
	
	if failedData, ok := msg.Values["failed_message"].(string); ok {
		if err := json.Unmarshal([]byte(failedData), &failedMsg); err == nil {
			originalData = &failedMsg.OriginalData
		}
	}
	
	if originalData == nil {
		return fmt.Errorf("failed to extract original data from message %s", messageID)
	}
	
	// 重新发布到主队列（需要主队列名称，这里假设为配置的一部分）
	mainStream := "defi:stream:position_updates" // 应从配置获取
	
	// 创建重试消息
	retryMsg := map[string]interface{}{
		"event_id":       fmt.Sprintf("retry_%s", originalData.EventID),
		"event_type":     "position_update",
		"user_address":   originalData.UserAddress,
		"protocol_id":    originalData.ProtocolID,
		"chain_id":       fmt.Sprintf("%d", originalData.ChainID),
		"position_data":  originalData.ToJSON(),
		"timestamp":      fmt.Sprintf("%d", time.Now().Unix()),
		"source":         "dlq_retry",
		"version":        originalData.Version,
		"dlq_original_id": messageID,
		"retry_count":    failedMsg.RetryCount + 1,
	}
	
	// 发布到主队列
	_, err = h.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: mainStream,
		Values: retryMsg,
	}).Result()
	
	if err != nil {
		return fmt.Errorf("failed to republish message: %w", err)
	}
	
	// 从死信队列删除
	if err := h.redisClient.XDel(ctx, h.streamName, messageID).Err(); err != nil {
		return fmt.Errorf("failed to delete from DLQ: %w", err)
	}
	
	h.logger.Printf("Retried DLQ message %s for user %s", messageID, originalData.UserAddress)
	return nil
}

// BatchRetry 批量重试
func (h *DLQHandler) BatchRetry(ctx context.Context, filter *DLQFilter) (int, error) {
	// 获取符合条件的消息
	messages, err := h.getFilteredMessages(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to get filtered messages: %w", err)
	}
	
	retried := 0
	errors := make([]string, 0)
	
	// 重试每个消息
	for _, msg := range messages {
		if err := h.RetryMessage(ctx, msg.ID); err != nil {
			h.logger.Printf("Failed to retry message %s: %v", msg.ID, err)
			errors = append(errors, fmt.Sprintf("%s: %v", msg.ID, err))
			continue
		}
		retried++
	}
	
	if len(errors) > 0 {
		h.logger.Printf("Batch retry completed with %d errors", len(errors))
	}
	
	return retried, nil
}

// getFilteredMessages 获取过滤后的消息
func (h *DLQHandler) getFilteredMessages(ctx context.Context, filter *DLQFilter) ([]redis.XMessage, error) {
	// 获取所有消息
	messages, err := h.redisClient.XRange(ctx, h.streamName, "-", "+").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	
	// 应用过滤条件
	filtered := make([]redis.XMessage, 0, len(messages))
	
	for _, msg := range messages {
		if h.matchesFilter(msg, filter) {
			filtered = append(filtered, msg)
		}
	}
	
	return filtered, nil
}

// matchesFilter 检查消息是否匹配过滤条件
func (h *DLQHandler) matchesFilter(msg redis.XMessage, filter *DLQFilter) bool {
	if filter == nil {
		return true
	}
	
	// 检查错误原因
	if filter.ErrorReason != "" {
		if reason, ok := msg.Values["error_reason"].(string); !ok || reason != filter.ErrorReason {
			return false
		}
	}
	
	// 检查时间范围
	if !filter.StartTime.IsZero() || !filter.EndTime.IsZero() {
		var failedAt time.Time
		if failedAtStr, ok := msg.Values["failed_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, failedAtStr); err == nil {
				failedAt = t
			}
		}
		
		if !filter.StartTime.IsZero() && failedAt.Before(filter.StartTime) {
			return false
		}
		if !filter.EndTime.IsZero() && failedAt.After(filter.EndTime) {
			return false
		}
	}
	
	// 检查用户地址
	if filter.UserAddress != "" {
		// 尝试从failed_message中提取
		userAddress := ""
		if failedData, ok := msg.Values["failed_message"].(string); ok {
			var failedMsg models.FailedMessage
			if err := json.Unmarshal([]byte(failedData), &failedMsg); err == nil {
				userAddress = failedMsg.OriginalData.UserAddress
			}
		}
		
		if userAddress != filter.UserAddress {
			return false
		}
	}
	
	return true
}

// DLQFilter 死信队列过滤条件
type DLQFilter struct {
	ErrorReason string    `json:"error_reason,omitempty"`
	StartTime   time.Time `json:"start_time,omitempty"`
	EndTime     time.Time `json:"end_time,omitempty"`
	UserAddress string    `json:"user_address,omitempty"`
	Limit       int       `json:"limit,omitempty"`
}

// CleanupOldMessages 清理旧消息
func (h *DLQHandler) CleanupOldMessages(ctx context.Context, maxAge time.Duration) (int64, error) {
	// 获取所有消息
	messages, err := h.redisClient.XRange(ctx, h.streamName, "-", "+").Result()
	if err != nil {
		return 0, fmt.Errorf("failed to get messages: %w", err)
	}
	
	cutoffTime := time.Now().Add(-maxAge)
	deleted := int64(0)
	
	// 删除旧消息
	for _, msg := range messages {
		var failedAt time.Time
		if failedAtStr, ok := msg.Values["failed_at"].(string); ok {
			if t, err := time.Parse(time.RFC3339, failedAtStr); err == nil {
				failedAt = t
			}
		}
		
		if failedAt.IsZero() || failedAt.Before(cutoffTime) {
			if err := h.redisClient.XDel(ctx, h.streamName, msg.ID).Err(); err != nil {
				h.logger.Printf("Failed to delete old message %s: %v", msg.ID, err)
				continue
			}
			deleted++
		}
	}
	
	if deleted > 0 {
		h.logger.Printf("Cleaned up %d old messages from DLQ older than %v", deleted, maxAge)
	}
	
	return deleted, nil
}

// ExportMessages 导出消息
func (h *DLQHandler) ExportMessages(ctx context.Context, format string) ([]byte, error) {
	// 获取所有消息
	messages, err := h.redisClient.XRange(ctx, h.streamName, "-", "+").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	
	// 转换为导出格式
	var exportData interface{}
	
	switch format {
	case "json":
		exportData = h.exportAsJSON(messages)
	case "csv":
		exportData = h.exportAsCSV(messages)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
	
	// 序列化
	data, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal export data: %w", err)
	}
	
	return data, nil
}

// exportAsJSON 导出为JSON格式
func (h *DLQHandler) exportAsJSON(messages []redis.XMessage) []DLQMessageDetail {
	details := make([]DLQMessageDetail, 0, len(messages))
	
	for _, msg := range messages {
		detail := h.parseMessageDetail(msg)
		details = append(details, *detail)
	}
	
	return details
}

// exportAsCSV 导出为CSV格式（简化版，返回JSON数组）
func (h *DLQHandler) exportAsCSV(messages []redis.XMessage) []map[string]string {
	records := make([]map[string]string, 0, len(messages))
	
	for _, msg := range messages {
		record := make(map[string]string