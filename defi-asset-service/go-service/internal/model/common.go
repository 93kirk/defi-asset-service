package model

import (
	"time"
)

// SyncRecord 同步记录模型
type SyncRecord struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	SyncType     string    `gorm:"size:50;not null;index:idx_sync_type" json:"sync_type"`
	SyncSource   string    `gorm:"size:50;not null;index:idx_sync_source" json:"sync_source"`
	TargetID     string    `gorm:"size:100;index:idx_target_id" json:"target_id,omitempty"`
	Status       string    `gorm:"size:20;not null;index:idx_status" json:"status"`
	TotalCount   int       `gorm:"default:0" json:"total_count"`
	SuccessCount int       `gorm:"default:0" json:"success_count"`
	FailedCount  int       `gorm:"default:0" json:"failed_count"`
	ErrorMessage string    `gorm:"type:text" json:"error_message,omitempty"`
	StartedAt    time.Time `gorm:"index:idx_started_at" json:"started_at"`
	FinishedAt   time.Time `json:"finished_at,omitempty"`
	DurationMs   int       `gorm:"default:0" json:"duration_ms"`
	Metadata     JSON      `gorm:"type:json" json:"metadata,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// TableName 返回表名
func (SyncRecord) TableName() string {
	return "sync_records"
}

// CacheStatus 缓存状态模型
type CacheStatus struct {
	ID            uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	CacheKey      string    `gorm:"size:500;not null;uniqueIndex:uk_cache_key" json:"cache_key"`
	CacheType     string    `gorm:"size:50;not null;index:idx_cache_type" json:"cache_type"`
	EntityID      string    `gorm:"size:100;not null;index:idx_entity_id" json:"entity_id"`
	TtlSeconds    int       `gorm:"not null;default:600" json:"ttl_seconds"`
	LastCachedAt  time.Time `gorm:"index:idx_last_cached" json:"last_cached_at"`
	ExpiresAt     time.Time `gorm:"index:idx_expires_at" json:"expires_at"`
	HitCount      int       `gorm:"default:0" json:"hit_count"`
	MissCount     int       `gorm:"default:0" json:"miss_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// TableName 返回表名
func (CacheStatus) TableName() string {
	return "cache_status"
}

// QueueMessage 队列消息模型
type QueueMessage struct {
	ID           uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	MessageID    string    `gorm:"size:100;not null;uniqueIndex:uk_message_id" json:"message_id"`
	QueueName    string    `gorm:"size:100;not null;index:idx_queue_name" json:"queue_name"`
	MessageType  string    `gorm:"size:50;not null;index:idx_message_type" json:"message_type"`
	Payload      JSON      `gorm:"type:json;not null" json:"payload"`
	Status       string    `gorm:"size:20;not null;default:'pending';index:idx_status" json:"status"`
	RetryCount   int       `gorm:"default:0" json:"retry_count"`
	MaxRetries   int       `gorm:"default:3" json:"max_retries"`
	ErrorMessage string    `gorm:"type:text" json:"error_message,omitempty"`
	ProcessedAt  time.Time `json:"processed_at,omitempty"`
	ScheduledAt  time.Time `gorm:"index:idx_scheduled_at" json:"scheduled_at"`
	CreatedAt    time.Time `gorm:"index:idx_created_at" json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 返回表名
func (QueueMessage) TableName() string {
	return "queue_messages"
}

// APIRequestLog API请求日志模型
type APIRequestLog struct {
	ID             uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	RequestID      string    `gorm:"size:100;not null;index:idx_request_id" json:"request_id"`
	APIPath        string    `gorm:"size:500;not null;index:idx_api_path" json:"api_path"`
	Method         string    `gorm:"size:10;not null" json:"method"`
	UserID         uint64    `gorm:"index:idx_user_id" json:"user_id,omitempty"`
	UserAddress    string    `gorm:"size:42;index:idx_user_address" json:"user_address,omitempty"`
	IPAddress      string    `gorm:"size:45" json:"ip_address,omitempty"`
	UserAgent      string    `gorm:"type:text" json:"user_agent,omitempty"`
	RequestParams  JSON      `gorm:"type:json" json:"request_params,omitempty"`
	RequestBody    JSON      `gorm:"type:json" json:"request_body,omitempty"`
	ResponseStatus int       `gorm:"not null;index:idx_response_status" json:"response_status"`
	ResponseTimeMs int       `gorm:"not null;index:idx_response_time" json:"response_time_ms"`
	ErrorCode      string    `gorm:"size:50" json:"error_code,omitempty"`
	ErrorMessage   string    `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt      time.Time `gorm:"index:idx_created_at" json:"created_at"`
}

// TableName 返回表名
func (APIRequestLog) TableName() string {
	return "api_request_logs"
}

// PositionUpdateMessage 仓位更新消息
type PositionUpdateMessage struct {
	EventID      string                 `json:"event_id"`
	EventType    string                 `json:"event_type"`
	UserAddress  string                 `json:"user_address"`
	ProtocolID   string                 `json:"protocol_id"`
	PositionData map[string]interface{} `json:"position_data"`
	Timestamp    int64                  `json:"timestamp"`
}

// CacheData 缓存数据
type CacheData struct {
	Data      interface{} `json:"data"`
	CachedAt  int64       `json:"cached_at"`
	TTL       int         `json:"ttl"`
	Version   int         `json:"version,omitempty"`
	ExpiresAt int64       `json:"expires_at"`
}

// PriceData 价格数据
type PriceData struct {
	Price      float64 `json:"price"`
	Source     string  `json:"source"`
	UpdatedAt  int64   `json:"updated_at"`
	TTL        int     `json:"ttl"`
}

// ApyData APY数据
type ApyData struct {
	SupplyApy  float64 `json:"supply_apy"`
	BorrowApy  float64 `json:"borrow_apy"`
	UpdatedAt  int64   `json:"updated_at"`
	TTL        int     `json:"ttl"`
}

// APIResponse API响应格式
type APIResponse struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// HealthCheckResponse 健康检查响应
type HealthCheckResponse struct {
	Status    string                 `json:"status"`
	Timestamp string                 `json:"timestamp"`
	Services  map[string]ServiceStatus `json:"services"`
}

// ServiceStatus 服务状态
type ServiceStatus struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Latency int64  `json:"latency,omitempty"`
}

// MetricsResponse 指标响应
type MetricsResponse struct {
	Requests struct {
		Total     int64 `json:"total"`
		Success   int64 `json:"success"`
		Failed    int64 `json:"failed"`
		AvgTimeMs int64 `json:"avg_time_ms"`
	} `json:"requests"`
	Cache struct {
		HitRate   float64 `json:"hit_rate"`
		TotalKeys int64   `json:"total_keys"`
		MemoryMB  float64 `json:"memory_mb"`
	} `json:"cache"`
	Queue struct {
		Pending   int64 `json:"pending"`
		Processed int64 `json:"processed"`
		Failed    int64 `json:"failed"`
	} `json:"queue"`
	Database struct {
		Connections int64 `json:"connections"`
		Queries     int64 `json:"queries"`
		SlowQueries int64 `json:"slow_queries"`
	} `json:"database"`
}

// 错误码定义
const (
	// 成功
	CodeSuccess = 0

	// 系统错误 (1-999)
	CodeInternalError      = 1
	CodeDatabaseError      = 2
	CodeRedisError         = 3
	CodeExternalServiceError = 4
	CodeQueueError         = 5

	// 认证错误 (1000-1999)
	CodeUnauthorized     = 1000
	CodeInvalidToken     = 1001
	CodeTokenExpired     = 1002
	CodeInvalidAPIKey    = 1003
	CodeRateLimitExceeded = 1004
	CodePermissionDenied = 1005

	// 参数错误 (2000-2999)
	CodeInvalidParameter = 2000
	CodeMissingParameter = 2001
	CodeInvalidAddress   = 2002
	CodeInvalidChainID   = 2003
	CodeInvalidProtocol  = 2004
	CodeBatchLimitExceeded = 2005

	// 业务错误 (3000-3999)
	CodeUserNotFound      = 3000
	CodeProtocolNotFound  = 3001
	CodePositionNotFound  = 3002
	CodeAssetNotFound     = 3003
	CodeSyncInProgress    = 3004
	CodeSyncFailed        = 3005
	CodeExternalServiceUnavailable = 3006

	// 外部服务错误 (4000-4999)
	CodeServiceAError = 4000
	CodeServiceBError = 4001
	CodeDebankError   = 4002
)

// 错误消息映射
var ErrorMessages = map[int]string{
	CodeSuccess: "success",
	
	CodeInternalError:      "internal server error",
	CodeDatabaseError:      "database error",
	CodeRedisError:         "redis error",
	CodeExternalServiceError: "external service error",
	CodeQueueError:         "queue processing error",
	
	CodeUnauthorized:     "unauthorized",
	CodeInvalidToken:     "invalid token",
	CodeTokenExpired:     "token expired",
	CodeInvalidAPIKey:    "invalid api key",
	CodeRateLimitExceeded: "rate limit exceeded",
	CodePermissionDenied: "permission denied",
	
	CodeInvalidParameter: "invalid parameter",
	CodeMissingParameter: "missing required parameter",
	CodeInvalidAddress:   "invalid address format",
	CodeInvalidChainID:   "invalid chain id",
	CodeInvalidProtocol:  "invalid protocol",
	CodeBatchLimitExceeded: "batch limit exceeded",
	
	CodeUserNotFound:      "user not found",
	CodeProtocolNotFound:  "protocol not found",
	CodePositionNotFound:  "position not found",
	CodeAssetNotFound:     "asset not found",
	CodeSyncInProgress:    "sync in progress",
	CodeSyncFailed:        "sync failed",
	CodeExternalServiceUnavailable: "external service unavailable",
	
	CodeServiceAError: "service a error",
	CodeServiceBError: "service b error",
	CodeDebankError:   "debank error",
}

// GetErrorMessage 获取错误消息
func GetErrorMessage(code int) string {
	if msg, ok := ErrorMessages[code]; ok {
		return msg
	}
	return "unknown error"
}

// NewAPIResponse 创建API响应
func NewAPIResponse(code int, data interface{}) APIResponse {
	return APIResponse{
		Code:      code,
		Message:   GetErrorMessage(code),
		Data:      data,
		Timestamp: time.Now().Unix(),
	}
}

// NewErrorResponse 创建错误响应
func NewErrorResponse(code int) ErrorResponse {
	return ErrorResponse{
		Code:    code,
		Message: GetErrorMessage(code),
	}
}