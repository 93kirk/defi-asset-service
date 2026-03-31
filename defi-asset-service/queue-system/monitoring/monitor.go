package monitoring

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"defi-asset-service/queue-system/config"
)

// QueueMonitor 队列监控器
type QueueMonitor struct {
	redisClient *redis.Client
	config      *config.QueueConfig
	logger      *log.Logger
	
	// Prometheus指标
	metrics     *QueueMetrics
	registry    *prometheus.Registry
	
	// 监控状态
	mu          sync.RWMutex
	isRunning   bool
	stopChan    chan struct{}
}

// QueueMetrics 队列监控指标
type QueueMetrics struct {
	// 队列指标
	QueueLength        prometheus.Gauge
	QueueConsumerCount prometheus.Gauge
	QueuePendingCount  prometheus.Gauge
	
	// 处理指标
	MessagesProcessed  prometheus.Counter
	MessagesFailed     prometheus.Counter
	MessagesRetried    prometheus.Counter
	MessagesDLQ        prometheus.Counter
	
	// 延迟指标
	ProcessingLatency  prometheus.Histogram
	QueueLatency       prometheus.Histogram
	
	// 消费者指标
	ActiveConsumers    prometheus.Gauge
	ConsumerLag        prometheus.Gauge
	
	// 系统指标
	RedisConnections   prometheus.Gauge
	RedisMemoryUsage   prometheus.Gauge
	RedisCommands      prometheus.Counter
}

// NewQueueMonitor 创建新的队列监控器
func NewQueueMonitor(redisClient *redis.Client, cfg *config.QueueConfig, logger *log.Logger) *QueueMonitor {
	registry := prometheus.NewRegistry()
	
	metrics := &QueueMetrics{
		QueueLength: promauto.With(registry).NewGauge(prometheus.GaugeOpts{
			Name: "defi_queue_length",
			Help: "Number of messages in the queue",
		}),
		
		QueueConsumerCount: promauto.With(registry).NewGauge(prometheus.GaugeOpts{
			Name: "defi_queue_consumer_count",
			Help: "Number of consumers in the consumer group",
		}),
		
		QueuePendingCount: promauto.With(registry).NewGauge(prometheus.GaugeOpts{
			Name: "defi_queue_pending_count",
			Help: "Number of pending messages",
		}),
		
		MessagesProcessed: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "defi_messages_processed_total",
			Help: "Total number of messages processed",
		}),
		
		MessagesFailed: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "defi_messages_failed_total",
			Help: "Total number of messages failed",
		}),
		
		MessagesRetried: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "defi_messages_retried_total",
			Help: "Total number of messages retried",
		}),
		
		MessagesDLQ: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "defi_messages_dlq_total",
			Help: "Total number of messages moved to DLQ",
		}),
		
		ProcessingLatency: promauto.With(registry).NewHistogram(prometheus.HistogramOpts{
			Name:    "defi_processing_latency_seconds",
			Help:    "Message processing latency in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16), // 1ms to 32s
		}),
		
		QueueLatency: promauto.With(registry).NewHistogram(prometheus.HistogramOpts{
			Name:    "defi_queue_latency_seconds",
			Help:    "Message queue latency in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 16),
		}),
		
		ActiveConsumers: promauto.With(registry).NewGauge(prometheus.GaugeOpts{
			Name: "defi_active_consumers",
			Help: "Number of active consumers",
		}),
		
		ConsumerLag: promauto.With(registry).NewGauge(prometheus.GaugeOpts{
			Name: "defi_consumer_lag_messages",
			Help: "Consumer lag in number of messages",
		}),
		
		RedisConnections: promauto.With(registry).NewGauge(prometheus.GaugeOpts{
			Name: "defi_redis_connections",
			Help: "Number of Redis connections",
		}),
		
		RedisMemoryUsage: promauto.With(registry).NewGauge(prometheus.GaugeOpts{
			Name: "defi_redis_memory_usage_bytes",
			Help: "Redis memory usage in bytes",
		}),
		
		RedisCommands: promauto.With(registry).NewCounter(prometheus.CounterOpts{
			Name: "defi_redis_commands_total",
			Help: "Total number of Redis commands executed",
		}),
	}
	
	return &QueueMonitor{
		redisClient: redisClient,
		config:      cfg,
		logger:      logger,
		metrics:     metrics,
		registry:    registry,
		stopChan:    make(chan struct{}),
	}
}

// Start 启动监控
func (m *QueueMonitor) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.isRunning {
		m.mu.Unlock()
		return fmt.Errorf("monitor is already running")
	}
	
	m.isRunning = true
	m.mu.Unlock()
	
	m.logger.Println("Starting queue monitor")
	
	// 启动监控循环
	go m.monitoringLoop(ctx)
	
	// 启动Prometheus HTTP服务器（如果启用）
	if m.config.MetricsEnabled {
		go m.startMetricsServer()
	}
	
	return nil
}

// Stop 停止监控
func (m *QueueMonitor) Stop() {
	m.mu.Lock()
	if !m.isRunning {
		m.mu.Unlock()
		return
	}
	
	m.isRunning = false
	m.mu.Unlock()
	
	close(m.stopChan)
	m.logger.Println("Queue monitor stopped")
}

// monitoringLoop 监控循环
func (m *QueueMonitor) monitoringLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.HealthCheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.collectMetrics(ctx)
		}
	}
}

// collectMetrics 收集指标
func (m *QueueMonitor) collectMetrics(ctx context.Context) {
	// 收集队列指标
	if err := m.collectQueueMetrics(ctx); err != nil {
		m.logger.Printf("Failed to collect queue metrics: %v", err)
	}
	
	// 收集Redis指标
	if err := m.collectRedisMetrics(ctx); err != nil {
		m.logger.Printf("Failed to collect Redis metrics: %v", err)
	}
	
	// 收集消费者指标
	if err := m.collectConsumerMetrics(ctx); err != nil {
		m.logger.Printf("Failed to collect consumer metrics: %v", err)
	}
}

// collectQueueMetrics 收集队列指标
func (m *QueueMonitor) collectQueueMetrics(ctx context.Context) error {
	// 获取队列长度
	queueLen, err := m.redisClient.XLen(ctx, m.config.StreamName).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to get queue length: %w", err)
	}
	
	m.metrics.QueueLength.Set(float64(queueLen))
	
	// 获取死信队列长度
	dlqLen, err := m.redisClient.XLen(ctx, m.config.DLQStreamName).Result()
	if err != nil && err != redis.Nil {
		// DLQ可能不存在，忽略错误
	} else {
		// 可以添加DLQ指标
	}
	
	// 获取消费者组信息
	groups, err := m.redisClient.XInfoGroups(ctx, m.config.StreamName).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to get consumer groups: %w", err)
	}
	
	// 查找目标消费者组
	var targetGroup *redis.XInfoGroup
	for _, group := range groups {
		if group.Name == m.config.ConsumerGroup {
			targetGroup = &group
			break
		}
	}
	
	if targetGroup != nil {
		m.metrics.QueueConsumerCount.Set(float64(len(groups)))
		m.metrics.QueuePendingCount.Set(float64(targetGroup.Pending))
		
		// 计算消费者延迟
		if err := m.calculateConsumerLag(ctx, targetGroup); err != nil {
			m.logger.Printf("Failed to calculate consumer lag: %v", err)
		}
	}
	
	return nil
}

// calculateConsumerLag 计算消费者延迟
func (m *QueueMonitor) calculateConsumerLag(ctx context.Context, group *redis.XInfoGroup) error {
	// 获取Stream信息
	streamInfo, err := m.redisClient.XInfoStream(ctx, m.config.StreamName).Result()
	if err != nil {
		return fmt.Errorf("failed to get stream info: %w", err)
	}
	
	// 获取最后一条消息的ID
	if len(streamInfo.LastEntry) == 0 {
		m.metrics.ConsumerLag.Set(0)
		return nil
	}
	
	lastID := streamInfo.LastEntry[0]
	
	// 获取消费者组的最后交付ID
	consumers, err := m.redisClient.XInfoConsumers(ctx, m.config.StreamName, m.config.ConsumerGroup).Result()
	if err != nil {
		return fmt.Errorf("failed to get consumer info: %w", err)
	}
	
	// 计算最慢的消费者延迟
	var maxLag int64
	activeConsumers := 0
	
	for _, consumer := range consumers {
		if consumer.Pending > 0 || consumer.Idle < m.config.HealthCheckInterval {
			activeConsumers++
		}
		
		// 这里简化计算，实际应根据消息ID计算延迟
		// 可以使用XPENDING命令获取更精确的延迟
	}
	
	m.metrics.ActiveConsumers.Set(float64(activeConsumers))
	
	// 使用pending消息数量作为延迟的近似值
	m.metrics.ConsumerLag.Set(float64(group.Pending))
	
	return nil
}

// collectRedisMetrics 收集Redis指标
func (m *QueueMonitor) collectRedisMetrics(ctx context.Context) error {
	// 获取Redis信息
	info, err := m.redisClient.Info(ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to get Redis info: %w", err)
	}
	
	// 解析信息（简化版，实际应使用完整的解析）
	// 这里只提取关键指标
	
	// 连接数
	// 在实际实现中，应解析info字符串获取具体值
	// 这里使用固定值示例
	m.metrics.RedisConnections.Set(10)
	
	// 内存使用
	m.metrics.RedisMemoryUsage.Set(1024 * 1024 * 100) // 100MB示例
	
	return nil
}

// collectConsumerMetrics 收集消费者指标
func (m *QueueMonitor) collectConsumerMetrics(ctx context.Context) error {
	// 在实际实现中，这里应从消费者实例获取指标
	// 这里使用固定值示例
	
	return nil
}

// startMetricsServer 启动指标服务器
func (m *QueueMonitor) startMetricsServer() {
	http.Handle("/metrics", promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		Registry: m.registry,
	}))
	
	port := ":9090"
	m.logger.Printf("Starting metrics server on %s", port)
	
	if err := http.ListenAndServe(port, nil); err != nil {
		m.logger.Printf("Metrics server error: %v", err)
	}
}

// RecordMessageProcessed 记录消息处理
func (m *QueueMonitor) RecordMessageProcessed(latency time.Duration) {
	m.metrics.MessagesProcessed.Inc()
	m.metrics.ProcessingLatency.Observe(latency.Seconds())
}

// RecordMessageFailed 记录消息失败
func (m *QueueMonitor) RecordMessageFailed() {
	m.metrics.MessagesFailed.Inc()
}

// RecordMessageRetried 记录消息重试
func (m *QueueMonitor) RecordMessageRetried() {
	m.metrics.MessagesRetried.Inc()
}

// RecordMessageDLQ 记录消息进入死信队列
func (m *QueueMonitor) RecordMessageDLQ() {
	m.metrics.MessagesDLQ.Inc()
}

// RecordQueueLatency 记录队列延迟
func (m *QueueMonitor) RecordQueueLatency(latency time.Duration) {
	m.metrics.QueueLatency.Observe(latency.Seconds())
}

// RecordRedisCommand 记录Redis命令
func (m *QueueMonitor) RecordRedisCommand() {
	m.metrics.RedisCommands.Inc()
}

// GetHealthStatus 获取健康状态
func (m *QueueMonitor) GetHealthStatus(ctx context.Context) (*HealthStatus, error) {
	status := &HealthStatus{
		Timestamp:   time.Now(),
		Components:  make(map[string]ComponentStatus),
	}
	
	// 检查Redis连接
	if err := m.redisClient.Ping(ctx).Err(); err != nil {
		status.Components["redis"] = ComponentStatus{
			Healthy: false,
			Message: fmt.Sprintf("Redis connection failed: %v", err),
		}
		status.OverallHealthy = false
	} else {
		status.Components["redis"] = ComponentStatus{
			Healthy: true,
			Message: "Connected",
		}
	}
	
	// 检查队列状态
	queueLen, err := m.redisClient.XLen(ctx, m.config.StreamName).Result()
	if err != nil && err != redis.Nil {
		status.Components["queue"] = ComponentStatus{
			Healthy: false,
			Message: fmt.Sprintf("Failed to check queue: %v", err),
		}
		status.OverallHealthy = false
	} else {
		queueStatus := "Healthy"
		if queueLen > 1000 {
			queueStatus = fmt.Sprintf("Warning: %d messages in queue", queueLen)
		}
		
		status.Components["queue"] = ComponentStatus{
			Healthy: queueLen <= 5000, // 阈值
			Message: queueStatus,
			Metrics: map[string]interface{}{
				"queue_length": queueLen,
			},
		}
		
		if queueLen > 5000 {
			status.OverallHealthy = false
		}
	}
	
	// 检查消费者组
	groups, err := m.redisClient.XInfoGroups(ctx, m.config.StreamName).Result()
	if err != nil && err != redis.Nil {
		status.Components["consumer_group"] = ComponentStatus{
			Healthy: false,
			Message: fmt.Sprintf("Failed to check consumer group: %v", err),
		}
		status.OverallHealthy = false
	} else {
		groupFound := false
		for _, group := range groups {
			if group.Name == m.config.ConsumerGroup {
				groupFound = true
				
				consumerStatus := "Healthy"
				if group.Pending > 100 {
					consumerStatus = fmt.Sprintf("Warning: %d pending messages", group.Pending)
				}
				
				status.Components["consumer_group"] = ComponentStatus{
					Healthy: group.Pending <= 500, // 阈值
					Message: consumerStatus,
					Metrics: map[string]interface{}{
						"pending_messages": group.Pending,
						"consumers":        group.Consumers,
					},
				}
				
				if group.Pending > 500 {
					status.OverallHealthy = false
				}
				break
			}
		}
		
		if !groupFound {
			status.Components["consumer_group"] = ComponentStatus{
				Healthy: false,
				Message: "Consumer group not found",
			}
			status.OverallHealthy = false
		}
	}
	
	// 检查死信队列
	dlqLen, err := m.redisClient.XLen(ctx, m.config.DLQStreamName).Result()
	if err != nil && err != redis.Nil {
		// DLQ可能不存在，忽略错误
	} else {
		dlqStatus := "Healthy"
		if dlqLen > 0 {
			dlqStatus = fmt.Sprintf("Warning: %d messages in DLQ", dlqLen)
		}
		
		status.Components["dlq"] = ComponentStatus{
			Healthy: dlqLen == 0,
			Message: dlqStatus,
			Metrics: map[string]interface{}{
				"dlq_length": dlqLen,
			},
		}
		
		if dlqLen > 100 {
			status.OverallHealthy = false
		}
	}
	
	return status, nil
}

// HealthStatus 健康状态
type HealthStatus struct {
	Timestamp      time.Time                  `json:"timestamp"`
	OverallHealthy bool                       `json:"overall_healthy"`
	Components     map[string]ComponentStatus `json:"components"`
}

// ComponentStatus 组件状态
type ComponentStatus struct {
	Healthy bool                   `json:"healthy"`
	Message string                 `json:"message"`
	Metrics map[string]interface{} `json:"metrics,omitempty"`
}

// GenerateReport 生成监控报告
func (m *QueueMonitor) GenerateReport(ctx context.Context) (*MonitorReport, error) {
	report := &MonitorReport{
		Timestamp: time.Now(),
		Metrics: