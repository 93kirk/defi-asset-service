package producer

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"

	"defi-asset-service/queue-system/config"
	"defi-asset-service/queue-system/models"
)

// Producer Redis队列生产者
type Producer struct {
	redisClient *redis.Client
	streamName  string
	config      *config.QueueConfig
	logger      *log.Logger
}

// NewProducer 创建新的生产者
func NewProducer(redisClient *redis.Client, cfg *config.QueueConfig, logger *log.Logger) *Producer {
	return &Producer{
		redisClient: redisClient,
		streamName:  cfg.StreamName,
		config:      cfg,
		logger:      logger,
	}
}

// PublishPositionUpdate 发布仓位更新消息
func (p *Producer) PublishPositionUpdate(ctx context.Context, msg *models.PositionUpdateMessage) (string, error) {
	// 验证消息
	if err := msg.Validate(); err != nil {
		return "", fmt.Errorf("invalid message: %w", err)
	}

	// 转换为JSON
	jsonData, err := msg.ToJSON()
	if err != nil {
		return "", fmt.Errorf("failed to serialize message: %w", err)
	}

	// 准备消息字段
	fields := map[string]interface{}{
		"event_id":     msg.EventID,
		"event_type":   msg.EventType,
		"user_address": msg.UserAddress,
		"protocol_id":  msg.ProtocolID,
		"chain_id":     fmt.Sprintf("%d", msg.ChainID),
		"position_data": jsonData,
		"timestamp":    fmt.Sprintf("%d", msg.Timestamp),
		"source":       msg.Source,
		"version":      msg.Version,
	}

	// 发布到Redis Stream
	messageID, err := p.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: p.streamName,
		Values: fields,
	}).Result()

	if err != nil {
		p.logger.Printf("Failed to publish message to stream %s: %v", p.streamName, err)
		return "", fmt.Errorf("failed to publish to stream: %w", err)
	}

	p.logger.Printf("Published message %s to stream %s", messageID, p.streamName)
	return messageID, nil
}

// PublishBatch 批量发布消息
func (p *Producer) PublishBatch(ctx context.Context, messages []*models.PositionUpdateMessage) ([]string, error) {
	if len(messages) == 0 {
		return nil, nil
	}

	// 使用Pipeline提高性能
	pipe := p.redisClient.Pipeline()
	messageIDs := make([]string, 0, len(messages))

	for _, msg := range messages {
		// 验证消息
		if err := msg.Validate(); err != nil {
			p.logger.Printf("Skipping invalid message: %v", err)
			continue
		}

		// 转换为JSON
		jsonData, err := msg.ToJSON()
		if err != nil {
			p.logger.Printf("Failed to serialize message: %v", err)
			continue
		}

		// 准备消息字段
		fields := map[string]interface{}{
			"event_id":     msg.EventID,
			"event_type":   msg.EventType,
			"user_address": msg.UserAddress,
			"protocol_id":  msg.ProtocolID,
			"chain_id":     fmt.Sprintf("%d", msg.ChainID),
			"position_data": jsonData,
			"timestamp":    fmt.Sprintf("%d", msg.Timestamp),
			"source":       msg.Source,
			"version":      msg.Version,
		}

		// 添加到Pipeline
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: p.streamName,
			Values: fields,
		})
	}

	// 执行Pipeline
	cmds, err := pipe.Exec(ctx)
	if err != nil {
		p.logger.Printf("Failed to execute pipeline: %v", err)
		return nil, fmt.Errorf("failed to execute pipeline: %w", err)
	}

	// 收集消息ID
	for _, cmd := range cmds {
		if xaddCmd, ok := cmd.(*redis.StringCmd); ok {
			if id, err := xaddCmd.Result(); err == nil {
				messageIDs = append(messageIDs, id)
			}
		}
	}

	p.logger.Printf("Published %d messages to stream %s", len(messageIDs), p.streamName)
	return messageIDs, nil
}

// GetQueueStats 获取队列统计信息
func (p *Producer) GetQueueStats(ctx context.Context) (*QueueStats, error) {
	// 获取Stream长度
	streamLen, err := p.redisClient.XLen(ctx, p.streamName).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get stream length: %w", err)
	}

	// 获取消费者组信息
	groups, err := p.redisClient.XInfoGroups(ctx, p.streamName).Result()
	if err != nil {
		// Stream可能不存在或没有消费者组
		p.logger.Printf("Failed to get consumer groups: %v", err)
	}

	// 获取Stream信息
	streamInfo, err := p.redisClient.XInfoStream(ctx, p.streamName).Result()
	if err != nil {
		p.logger.Printf("Failed to get stream info: %v", err)
	}

	stats := &QueueStats{
		StreamName:    p.streamName,
		MessageCount:  streamLen,
		FirstMessage:  nil,
		LastMessage:   nil,
		ConsumerCount: len(groups),
		CreatedAt:     time.Now(),
	}

	// 获取第一条和最后一条消息
	if streamLen > 0 {
		// 获取第一条消息
		messages, err := p.redisClient.XRange(ctx, p.streamName, "-", "+").Result()
		if err == nil && len(messages) > 0 {
			stats.FirstMessage = &messages[0]
			stats.LastMessage = &messages[len(messages)-1]
		}
	}

	return stats, nil
}

// HealthCheck 健康检查
func (p *Producer) HealthCheck(ctx context.Context) error {
	// 检查Redis连接
	if err := p.redisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis connection failed: %w", err)
	}

	// 检查Stream是否存在或可以创建
	_, err := p.redisClient.XLen(ctx, p.streamName).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("stream check failed: %w", err)
	}

	return nil
}

// QueueStats 队列统计信息
type QueueStats struct {
	StreamName    string
	MessageCount  int64
	FirstMessage  *redis.XMessage
	LastMessage   *redis.XMessage
	ConsumerCount int
	CreatedAt     time.Time
}

// ExampleUsage 示例使用
func ExampleUsage() {
	// 创建Redis客户端
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       3,
	})

	// 创建配置
	cfg := &config.QueueConfig{
		StreamName:    "defi:stream:position_updates",
		ConsumerGroup: "position_workers",
		ConsumerName:  "worker",
	}

	// 创建生产者
	producer := NewProducer(redisClient, cfg, log.Default())

	// 创建仓位更新消息
	position := models.PositionData{
		TokenAddress: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		TokenSymbol:  "USDC",
		Amount:       "10000.0",
		AmountUSD:    "10000.0",
		APY:          "2.15",
		RiskLevel:    2,
	}

	msg := &models.PositionUpdateMessage{
		EventID:     uuid.New().String(),
		EventType:   "position_update",
		UserAddress: "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
		ProtocolID:  "aave",
		ChainID:     1,
		Position:    position,
		Timestamp:   time.Now().Unix(),
		Source:      "service_b",
		Version:     "1.0",
	}

	// 发布消息
	ctx := context.Background()
	messageID, err := producer.PublishPositionUpdate(ctx, msg)
	if err != nil {
		log.Printf("Failed to publish message: %v", err)
		return
	}

	log.Printf("Message published with ID: %s", messageID)
}