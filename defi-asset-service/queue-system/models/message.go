package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// PositionData 仓位数据
type PositionData struct {
	TokenAddress string          `json:"token_address"`
	TokenSymbol  string          `json:"token_symbol"`
	Amount       string          `json:"amount"`       // 原始数量（字符串格式）
	AmountUSD    string          `json:"amount_usd"`   // USD价值（字符串格式）
	APY          string          `json:"apy,omitempty"` // 年化收益率
	RiskLevel    int             `json:"risk_level,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"` // 扩展元数据
}

// PositionUpdateMessage 仓位更新消息
type PositionUpdateMessage struct {
	EventID     string        `json:"event_id"`     // 事件ID（UUID）
	EventType   string        `json:"event_type"`   // 事件类型：position_update
	UserAddress string        `json:"user_address"` // 用户钱包地址
	ProtocolID  string        `json:"protocol_id"`  // 协议ID（如aave、compound）
	ChainID     int           `json:"chain_id"`     // 链ID（1=以太坊主网）
	Position    PositionData  `json:"position_data"` // 仓位数据
	Timestamp   int64         `json:"timestamp"`    // 时间戳（Unix秒）
	Source      string        `json:"source"`       // 数据来源（service_b）
	Version     string        `json:"version"`      // 消息版本
}

// Validate 验证消息有效性
func (m *PositionUpdateMessage) Validate() error {
	if m.EventID == "" {
		return fmt.Errorf("event_id is required")
	}
	if m.EventType == "" {
		return fmt.Errorf("event_type is required")
	}
	if m.UserAddress == "" {
		return fmt.Errorf("user_address is required")
	}
	if m.ProtocolID == "" {
		return fmt.Errorf("protocol_id is required")
	}
	if m.Timestamp == 0 {
		return fmt.Errorf("timestamp is required")
	}
	
	// 验证用户地址格式（以太坊地址）
	if len(m.UserAddress) != 42 || m.UserAddress[:2] != "0x" {
		return fmt.Errorf("invalid user_address format")
	}
	
	return nil
}

// ToJSON 转换为JSON字符串
func (m *PositionUpdateMessage) ToJSON() (string, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}
	return string(data), nil
}

// FromJSON 从JSON字符串解析
func FromJSON(data string) (*PositionUpdateMessage, error) {
	var msg PositionUpdateMessage
	if err := json.Unmarshal([]byte(data), &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	return &msg, nil
}

// FailedMessage 失败消息（用于死信队列）
type FailedMessage struct {
	OriginalMessageID string                 `json:"original_message_id"` // 原始消息ID
	OriginalStream    string                 `json:"original_stream"`     // 原始Stream名称
	OriginalData      PositionUpdateMessage  `json:"original_data"`       // 原始消息数据
	ErrorReason       string                 `json:"error_reason"`        // 错误原因
	ErrorMessage      string                 `json:"error_message"`       // 错误信息
	RetryCount        int                    `json:"retry_count"`         // 重试次数
	FailedAt          time.Time              `json:"failed_at"`           // 失败时间
	NextRetryAt       *time.Time             `json:"next_retry_at,omitempty"` // 下次重试时间
	Metadata          map[string]interface{} `json:"metadata,omitempty"`  // 扩展元数据
}

// NewPositionUpdateMessage 创建新的仓位更新消息
func NewPositionUpdateMessage(userAddress, protocolID string, position PositionData) *PositionUpdateMessage {
	return &PositionUpdateMessage{
		EventID:     generateEventID(),
		EventType:   "position_update",
		UserAddress: userAddress,
		ProtocolID:  protocolID,
		ChainID:     1, // 默认以太坊主网
		Position:    position,
		Timestamp:   time.Now().Unix(),
		Source:      "service_b",
		Version:     "1.0",
	}
}

// generateEventID 生成事件ID（简化版，实际应使用UUID）
func generateEventID() string {
	return fmt.Sprintf("evt_%d_%d", time.Now().UnixNano(), time.Now().Nanosecond())
}