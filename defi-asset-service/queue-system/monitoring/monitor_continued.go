package monitoring

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// GenerateReport 生成监控报告（续）
func (m *QueueMonitor) GenerateReport(ctx context.Context) (*MonitorReport, error) {
	report := &MonitorReport{
		Timestamp: time.Now(),
		Metrics:   make(map[string]interface{}),
		Alerts:    make([]Alert, 0),
	}
	
	// 收集队列指标
	queueLen, err := m.redisClient.XLen(ctx, m.config.StreamName).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get queue length: %w", err)
	}
	
	report.Metrics["queue_length"] = queueLen
	
	// 收集消费者组信息
	groups, err := m.redisClient.XInfoGroups(ctx, m.config.StreamName).Result()
	if err != nil && err != redis.Nil {
		// 忽略错误，继续生成报告
	} else {
		for _, group := range groups {
			if group.Name == m.config.ConsumerGroup {
				report.Metrics["pending_messages"] = group.Pending
				report.Metrics["consumer_count"] = group.Consumers
				
				// 检查pending消息告警
				if group.Pending > 1000 {
					report.Alerts = append(report.Alerts, Alert{
						Level:     "warning",
						Component: "consumer_group",
						Message:   fmt.Sprintf("High pending messages: %d", group.Pending),
						Timestamp: time.Now(),
					})
				}
				break
			}
		}
	}
	
	// 收集死信队列指标
	dlqLen, err := m.redisClient.XLen(ctx, m.config.DLQStreamName).Result()
	if err != nil && err != redis.Nil {
		// DLQ可能不存在
	} else {
		report.Metrics["dlq_length"] = dlqLen
		
		// 检查DLQ告警
		if dlqLen > 0 {
			report.Alerts = append(report.Alerts, Alert{
				Level:     "warning",
				Component: "dlq",
				Message:   fmt.Sprintf("Messages in DLQ: %d", dlqLen),
				Timestamp: time.Now(),
			})
		}
	}
	
	// 收集Redis指标
	info, err := m.redisClient.Info(ctx).Result()
	if err != nil {
		report.Alerts = append(report.Alerts, Alert{
			Level:     "error",
			Component: "redis",
			Message:   fmt.Sprintf("Failed to get Redis info: %v", err),
			Timestamp: time.Now(),
		})
	} else {
		// 解析关键指标（简化版）
		report.Metrics["redis_connected"] = true
		// 在实际实现中，应解析info字符串获取具体指标
	}
	
	// 设置报告状态
	report.Status = "healthy"
	if len(report.Alerts) > 0 {
		hasError := false
		for _, alert := range report.Alerts {
			if alert.Level == "error" {
				hasError = true
				break
			}
		}
		
		if hasError {
			report.Status = "unhealthy"
		} else {
			report.Status = "degraded"
		}
	}
	
	return report, nil
}

// MonitorReport 监控报告
type MonitorReport struct {
	Timestamp time.Time              `json:"timestamp"`
	Status    string                 `json:"status"` // healthy, degraded, unhealthy
	Metrics   map[string]interface{} `json:"metrics"`
	Alerts    []Alert                `json:"alerts,omitempty"`
	Summary   string                 `json:"summary,omitempty"`
}

// Alert 告警
type Alert struct {
	Level     string    `json:"level"`     // info, warning, error
	Component string    `json:"component"` // redis, queue, consumer, dlq
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// SetupAlerting 设置告警
func (m *QueueMonitor) SetupAlerting(ctx context.Context, alertConfig *AlertConfig) error {
	// 启动告警检查循环
	go m.alertingLoop(ctx, alertConfig)
	return nil
}

// alertingLoop 告警循环
func (m *QueueMonitor) alertingLoop(ctx context.Context, config *AlertConfig) {
	ticker := time.NewTicker(config.CheckInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkAlerts(ctx, config)
		}
	}
}

// checkAlerts 检查告警
func (m *QueueMonitor) checkAlerts(ctx context.Context, config *AlertConfig) {
	// 检查队列长度告警
	if err := m.checkQueueLengthAlert(ctx, config); err != nil {
		m.logger.Printf("Failed to check queue length alert: %v", err)
	}
	
	// 检查消费者延迟告警
	if err := m.checkConsumerLagAlert(ctx, config); err != nil {
		m.logger.Printf("Failed to check consumer lag alert: %v", err)
	}
	
	// 检查死信队列告警
	if err := m.checkDLQAlert(ctx, config); err != nil {
		m.logger.Printf("Failed to check DLQ alert: %v", err)
	}
	
	// 检查Redis连接告警
	if err := m.checkRedisAlert(ctx, config); err != nil {
		m.logger.Printf("Failed to check Redis alert: %v", err)
	}
}

// checkQueueLengthAlert 检查队列长度告警
func (m *QueueMonitor) checkQueueLengthAlert(ctx context.Context, config *AlertConfig) error {
	queueLen, err := m.redisClient.XLen(ctx, m.config.StreamName).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to get queue length: %w", err)
	}
	
	// 检查告警阈值
	if queueLen > config.QueueLengthWarning {
		level := "warning"
		if queueLen > config.QueueLengthCritical {
			level = "critical"
		}
		
		m.triggerAlert(ctx, &Alert{
			Level:     level,
			Component: "queue",
			Message:   fmt.Sprintf("Queue length is %d (threshold: %d)", queueLen, config.QueueLengthWarning),
			Timestamp: time.Now(),
		})
	}
	
	return nil
}

// checkConsumerLagAlert 检查消费者延迟告警
func (m *QueueMonitor) checkConsumerLagAlert(ctx context.Context, config *AlertConfig) error {
	// 获取消费者组信息
	groups, err := m.redisClient.XInfoGroups(ctx, m.config.StreamName).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to get consumer groups: %w", err)
	}
	
	// 查找目标消费者组
	for _, group := range groups {
		if group.Name == m.config.ConsumerGroup {
			// 检查pending消息告警
			if group.Pending > config.ConsumerLagWarning {
				level := "warning"
				if group.Pending > config.ConsumerLagCritical {
					level = "critical"
				}
				
				m.triggerAlert(ctx, &Alert{
					Level:     level,
					Component: "consumer",
					Message:   fmt.Sprintf("Consumer lag is %d messages (threshold: %d)", group.Pending, config.ConsumerLagWarning),
					Timestamp: time.Now(),
				})
			}
			break
		}
	}
	
	return nil
}

// checkDLQAlert 检查死信队列告警
func (m *QueueMonitor) checkDLQAlert(ctx context.Context, config *AlertConfig) error {
	dlqLen, err := m.redisClient.XLen(ctx, m.config.DLQStreamName).Result()
	if err != nil && err != redis.Nil {
		// DLQ可能不存在
		return nil
	}
	
	// 检查DLQ告警
	if dlqLen > config.DLQLengthWarning {
		level := "warning"
		if dlqLen > config.DLQLengthCritical {
			level = "critical"
		}
		
		m.triggerAlert(ctx, &Alert{
			Level:     level,
			Component: "dlq",
			Message:   fmt.Sprintf("DLQ has %d messages (threshold: %d)", dlqLen, config.DLQLengthWarning),
			Timestamp: time.Now(),
		})
	}
	
	return nil
}

// checkRedisAlert 检查Redis告警
func (m *QueueMonitor) checkRedisAlert(ctx context.Context, config *AlertConfig) error {
	// 检查Redis连接
	if err := m.redisClient.Ping(ctx).Err(); err != nil {
		m.triggerAlert(ctx, &Alert{
			Level:     "critical",
			Component: "redis",
			Message:   fmt.Sprintf("Redis connection failed: %v", err),
			Timestamp: time.Now(),
		})
		return nil
	}
	
	// 获取Redis内存信息
	info, err := m.redisClient.Info(ctx).Result()
	if err != nil {
		m.triggerAlert(ctx, &Alert{
			Level:     "warning",
			Component: "redis",
			Message:   fmt.Sprintf("Failed to get Redis info: %v", err),
			Timestamp: time.Now(),
		})
		return nil
	}
	
	// 解析内存使用率（简化版）
	// 在实际实现中，应解析info字符串获取内存使用率
	// 这里使用固定值示例
	memoryUsage := 0.5 // 50%
	
	if memoryUsage > config.RedisMemoryWarning {
		level := "warning"
		if memoryUsage > config.RedisMemoryCritical {
			level = "critical"
		}
		
		m.triggerAlert(ctx, &Alert{
			Level:     level,
			Component: "redis",
			Message:   fmt.Sprintf("Redis memory usage is %.1f%% (threshold: %.1f%%)", memoryUsage*100, config.RedisMemoryWarning*100),
			Timestamp: time.Now(),
		})
	}
	
	return nil
}

// triggerAlert 触发告警
func (m *QueueMonitor) triggerAlert(ctx context.Context, alert *Alert) {
	m.logger.Printf("Alert: %s - %s: %s", alert.Level, alert.Component, alert.Message)
	
	// 在实际实现中，这里应发送告警到监控系统
	// 例如：发送到Slack、邮件、PagerDuty等
	
	// 记录到指标
	switch alert.Level {
	case "warning":
		m.metrics.MessagesFailed.Inc() // 使用现有指标示例
	case "critical":
		m.metrics.MessagesDLQ.Inc() // 使用现有指标示例
	}
}

// AlertConfig 告警配置
type AlertConfig struct {
	// 队列长度告警
	QueueLengthWarning  int64
	QueueLengthCritical int64
	
	// 消费者延迟告警
	ConsumerLagWarning  int64
	ConsumerLagCritical int64
	
	// 死信队列告警
	DLQLengthWarning    int64
	DLQLengthCritical   int64
	
	// Redis内存告警
	RedisMemoryWarning  float64 // 0.0-1.0
	RedisMemoryCritical float64 // 0.0-1.0
	
	// 检查间隔
	CheckInterval       time.Duration
	
	// 告警通知配置
	NotifySlack         bool
	SlackWebhookURL     string
	NotifyEmail         bool
	EmailRecipients     []string
	NotifyPagerDuty     bool
	PagerDutyServiceKey string
}

// DefaultAlertConfig 默认告警配置
func DefaultAlertConfig() *AlertConfig {
	return &AlertConfig{
		QueueLengthWarning:  1000,
		QueueLengthCritical: 5000,
		ConsumerLagWarning:  100,
		ConsumerLagCritical: 500,
		DLQLengthWarning:    10,
		DLQLengthCritical:   100,
		RedisMemoryWarning:  0.8,  // 80%
		RedisMemoryCritical: 0.95, // 95%
		CheckInterval:       30 * time.Second,
		NotifySlack:         false,
		NotifyEmail:         false,
		NotifyPagerDuty:     false,
	}
}

// HTTPHandler HTTP处理器
type HTTPHandler struct {
	monitor *QueueMonitor
}

// NewHTTPHandler 创建新的HTTP处理器
func NewHTTPHandler(monitor *QueueMonitor) *HTTPHandler {
	return &HTTPHandler{
		monitor: monitor,
	}
}

// ServeHTTP 处理HTTP请求
func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	switch r.URL.Path {
	case "/health":
		h.healthHandler(w, r)
	case "/metrics":
		h.metricsHandler(w, r)
	case "/report":
		h.reportHandler(w, r)
	case "/alerts":
		h.alertsHandler(w, r)
	default:
		http.NotFound(w, r)
	}
}

// healthHandler 健康检查处理器
func (h *HTTPHandler) healthHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	status, err := h.monitor.GetHealthStatus(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get health status: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// metricsHandler 指标处理器
func (h *HTTPHandler) metricsHandler(w http.ResponseWriter, r *http.Request) {
	// Prometheus指标端点
	promhttp.Handler().ServeHTTP(w, r)
}

// reportHandler 报告处理器
func (h *HTTPHandler) reportHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	report, err := h.monitor.GenerateReport(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate report: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(report)
}

// alertsHandler 告警处理器
func (h *HTTPHandler) alertsHandler(w http.ResponseWriter, r *http.Request) {
	// 返回当前活跃告警
	// 在实际实现中，应从告警存储中获取
	
	alerts := []Alert{
		{
			Level:     "info",
			Component: "system",
			Message:   "No active alerts",
			Timestamp: time.Now(),
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(alerts)
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
		StreamName:          "defi:stream:position_updates",
		ConsumerGroup:       "position_workers",
		DLQStreamName:       "defi:stream:dlq:position_updates",
		MetricsEnabled:      true,
		HealthCheckInterval: 30 * time.Second,
	}
	
	// 创建监控器
	monitor := NewQueueMonitor(redisClient, cfg, log.Default())
	
	// 启动监控
	ctx := context.Background()
	if err := monitor.Start(ctx); err != nil {
		log.Fatalf("Failed to start monitor: %v", err)
	}
	
	// 设置告警
	alertConfig := DefaultAlertConfig()
	alertConfig.NotifySlack = true
	alertConfig.SlackWebhookURL = "https://hooks.slack.com/services/..."
	
	if err := monitor.SetupAlerting(ctx, alertConfig); err != nil {
		log.Printf("Failed to setup alerting: %v", err)
	}
	
	// 启动HTTP服务器
	handler := NewHTTPHandler(monitor)
	http.Handle("/", handler)
	
	log.Println("Starting monitoring server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
	
	// 优雅关闭
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	
	<-signalChan
	monitor.Stop()
	log.Println("Monitor stopped")
}