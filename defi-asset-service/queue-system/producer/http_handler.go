package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/mux"

	"defi-asset-service/queue-system/models"
)

// HTTPServer HTTP服务器
type HTTPServer struct {
	producer *Producer
	router   *mux.Router
	server   *http.Server
	logger   *log.Logger
}

// NewHTTPServer 创建新的HTTP服务器
func NewHTTPServer(producer *Producer, port string, logger *log.Logger) *HTTPServer {
	router := mux.NewRouter()
	
	server := &HTTPServer{
		producer: producer,
		router:   router,
		server: &http.Server{
			Addr:         ":" + port,
			Handler:      router,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		logger: logger,
	}
	
	server.setupRoutes()
	return server
}

// setupRoutes 设置路由
func (s *HTTPServer) setupRoutes() {
	s.router.HandleFunc("/health", s.healthHandler).Methods("GET")
	s.router.HandleFunc("/stats", s.statsHandler).Methods("GET")
	s.router.HandleFunc("/publish", s.publishHandler).Methods("POST")
	s.router.HandleFunc("/publish/batch", s.publishBatchHandler).Methods("POST")
	s.router.HandleFunc("/metrics", s.metricsHandler).Methods("GET")
}

// healthHandler 健康检查接口
func (s *HTTPServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// 检查Redis连接
	if err := s.producer.HealthCheck(ctx); err != nil {
		s.logger.Printf("Health check failed: %v", err)
		http.Error(w, fmt.Sprintf("Health check failed: %v", err), http.StatusServiceUnavailable)
		return
	}
	
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"service":   "redis-queue-producer",
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// statsHandler 获取队列统计信息
func (s *HTTPServer) statsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	stats, err := s.producer.GetQueueStats(ctx)
	if err != nil {
		s.logger.Printf("Failed to get queue stats: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get queue stats: %v", err), http.StatusInternalServerError)
		return
	}
	
	response := map[string]interface{}{
		"stream_name":    stats.StreamName,
		"message_count":  stats.MessageCount,
		"consumer_count": stats.ConsumerCount,
		"created_at":     stats.CreatedAt.Format(time.RFC3339),
		"timestamp":      time.Now().Unix(),
	}
	
	// 添加第一条和最后一条消息的信息
	if stats.FirstMessage != nil {
		response["first_message_id"] = stats.FirstMessage.ID
		response["first_message_time"] = extractTimestamp(stats.FirstMessage)
	}
	if stats.LastMessage != nil {
		response["last_message_id"] = stats.LastMessage.ID
		response["last_message_time"] = extractTimestamp(stats.LastMessage)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// publishHandler 发布单个消息
func (s *HTTPServer) publishHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// 解析请求体
	var request struct {
		UserAddress string                 `json:"user_address"`
		ProtocolID  string                 `json:"protocol_id"`
		Position    models.PositionData    `json:"position_data"`
		ChainID     int                    `json:"chain_id,omitempty"`
		Source      string                 `json:"source,omitempty"`
		Metadata    map[string]interface{} `json:"metadata,omitempty"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.logger.Printf("Failed to decode request: %v", err)
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	
	// 创建消息
	msg := &models.PositionUpdateMessage{
		EventID:     fmt.Sprintf("evt_%d", time.Now().UnixNano()),
		EventType:   "position_update",
		UserAddress: request.UserAddress,
		ProtocolID:  request.ProtocolID,
		ChainID:     request.ChainID,
		Position:    request.Position,
		Timestamp:   time.Now().Unix(),
		Source:      request.Source,
		Version:     "1.0",
	}
	
	if request.ChainID == 0 {
		msg.ChainID = 1 // 默认以太坊主网
	}
	if request.Source == "" {
		msg.Source = "service_b"
	}
	
	// 发布消息
	messageID, err := s.producer.PublishPositionUpdate(ctx, msg)
	if err != nil {
		s.logger.Printf("Failed to publish message: %v", err)
		http.Error(w, fmt.Sprintf("Failed to publish message: %v", err), http.StatusInternalServerError)
		return
	}
	
	response := map[string]interface{}{
		"message_id":   messageID,
		"event_id":     msg.EventID,
		"user_address": msg.UserAddress,
		"protocol_id":  msg.ProtocolID,
		"timestamp":    msg.Timestamp,
		"status":       "published",
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// publishBatchHandler 批量发布消息
func (s *HTTPServer) publishBatchHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// 解析请求体
	var requests []struct {
		UserAddress string                 `json:"user_address"`
		ProtocolID  string                 `json:"protocol_id"`
		Position    models.PositionData    `json:"position_data"`
		ChainID     int                    `json:"chain_id,omitempty"`
		Source      string                 `json:"source,omitempty"`
		Metadata    map[string]interface{} `json:"metadata,omitempty"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
		s.logger.Printf("Failed to decode batch request: %v", err)
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}
	
	// 创建消息列表
	messages := make([]*models.PositionUpdateMessage, 0, len(requests))
	for _, req := range requests {
		msg := &models.PositionUpdateMessage{
			EventID:     fmt.Sprintf("evt_%d", time.Now().UnixNano()),
			EventType:   "position_update",
			UserAddress: req.UserAddress,
			ProtocolID:  req.ProtocolID,
			ChainID:     req.ChainID,
			Position:    req.Position,
			Timestamp:   time.Now().Unix(),
			Source:      req.Source,
			Version:     "1.0",
		}
		
		if req.ChainID == 0 {
			msg.ChainID = 1 // 默认以太坊主网
		}
		if req.Source == "" {
			msg.Source = "service_b"
		}
		
		messages = append(messages, msg)
	}
	
	// 批量发布消息
	messageIDs, err := s.producer.PublishBatch(ctx, messages)
	if err != nil {
		s.logger.Printf("Failed to publish batch messages: %v", err)
		http.Error(w, fmt.Sprintf("Failed to publish batch messages: %v", err), http.StatusInternalServerError)
		return
	}
	
	response := map[string]interface{}{
		"total_messages": len(requests),
		"published_count": len(messageIDs),
		"message_ids":    messageIDs,
		"timestamp":      time.Now().Unix(),
		"status":         "published",
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// metricsHandler 获取监控指标
func (s *HTTPServer) metricsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// 获取队列统计
	stats, err := s.producer.GetQueueStats(ctx)
	if err != nil {
		s.logger.Printf("Failed to get metrics: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get metrics: %v", err), http.StatusInternalServerError)
		return
	}
	
	// 获取Redis信息
	redisInfo, err := s.producer.redisClient.Info(ctx).Result()
	if err != nil {
		s.logger.Printf("Failed to get Redis info: %v", err)
	}
	
	response := map[string]interface{}{
		"queue_stats": map[string]interface{}{
			"stream_name":    stats.StreamName,
			"message_count":  stats.MessageCount,
			"consumer_count": stats.ConsumerCount,
		},
		"redis_info": map[string]interface{}{
			"connected": err == nil,
			"info":      redisInfo,
		},
		"timestamp": time.Now().Unix(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// extractTimestamp 从消息中提取时间戳
func extractTimestamp(msg *redis.XMessage) int64 {
	if timestampStr, ok := msg.Values["timestamp"].(string); ok {
		var timestamp int64
		fmt.Sscanf(timestampStr, "%d", &timestamp)
		return timestamp
	}
	return 0
}

// Start 启动HTTP服务器
func (s *HTTPServer) Start() error {
	s.logger.Printf("Starting HTTP server on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown 优雅关闭HTTP服务器
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	s.logger.Println("Shutting down HTTP server")
	return s.server.Shutdown(ctx)
}