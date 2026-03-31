package circuit_breaker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// CircuitState 熔断器状态
type CircuitState int

const (
	StateClosed CircuitState = iota    // 闭合状态：正常处理请求
	StateOpen                         // 打开状态：拒绝所有请求
	StateHalfOpen                     // 半开状态：尝试恢复，允许部分请求通过
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "CLOSED"
	case StateOpen:
		return "OPEN"
	case StateHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	// 失败阈值配置
	FailureThreshold     int           `json:"failure_threshold" yaml:"failure_threshold"`
	FailureWindow        time.Duration `json:"failure_window" yaml:"failure_window"`
	
	// 半开状态配置
	HalfOpenMaxRequests  int           `json:"half_open_max_requests" yaml:"half_open_max_requests"`
	HalfOpenSuccessThreshold int       `json:"half_open_success_threshold" yaml:"half_open_success_threshold"`
	
	// 超时配置
	OpenStateTimeout     time.Duration `json:"open_state_timeout" yaml:"open_state_timeout"`
	ResetTimeout         time.Duration `json:"reset_timeout" yaml:"reset_timeout"`
	
	// 监控配置
	MonitorWindow        time.Duration `json:"monitor_window" yaml:"monitor_window"`
	MinRequests          int           `json:"min_requests" yaml:"min_requests"`
}

// DefaultCircuitBreakerConfig 默认熔断器配置
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold:     5,
		FailureWindow:        10 * time.Second,
		HalfOpenMaxRequests:  3,
		HalfOpenSuccessThreshold: 2,
		OpenStateTimeout:     30 * time.Second,
		ResetTimeout:         60 * time.Second,
		MonitorWindow:        60 * time.Second,
		MinRequests:          10,
	}
}

// CircuitBreakerStats 熔断器统计信息
type CircuitBreakerStats struct {
	TotalRequests     int64         `json:"total_requests"`
	SuccessfulRequests int64        `json:"successful_requests"`
	FailedRequests    int64         `json:"failed_requests"`
	RejectedRequests  int64         `json:"rejected_requests"`
	StateChanges      int64         `json:"state_changes"`
	CurrentState      CircuitState  `json:"current_state"`
	FailureRate       float64       `json:"failure_rate"`
	LastFailureTime   time.Time     `json:"last_failure_time"`
	LastSuccessTime   time.Time     `json:"last_success_time"`
	OpenedAt          time.Time     `json:"opened_at"`
	HalfOpenedAt      time.Time     `json:"half_opened_at"`
}

// CircuitBreaker 熔断器接口
type CircuitBreaker interface {
	// Execute 执行操作，受熔断器保护
	Execute(ctx context.Context, fn func() error) error
	
	// Allow 检查是否允许请求
	Allow() bool
	
	// GetState 获取当前状态
	GetState() CircuitState
	
	// GetStats 获取统计信息
	GetStats() CircuitBreakerStats
	
	// Reset 重置熔断器
	Reset()
	
	// ForceState 强制设置状态（用于测试）
	ForceState(state CircuitState)
}

// RequestResult 请求结果
type RequestResult struct {
	Success   bool
	Error     error
	Timestamp time.Time
}

// DefaultCircuitBreaker 默认熔断器实现
type DefaultCircuitBreaker struct {
	config CircuitBreakerConfig
	stats  CircuitBreakerStats
	state  CircuitState
	
	// 失败记录
	failures     []time.Time
	halfOpenReqs int
	halfOpenSucc int
	
	// 时间窗口
	lastResetTime time.Time
	stateChangeTime time.Time
	
	mu sync.RWMutex
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(config CircuitBreakerConfig) *DefaultCircuitBreaker {
	return &DefaultCircuitBreaker{
		config:         config,
		stats:          CircuitBreakerStats{CurrentState: StateClosed},
		state:          StateClosed,
		failures:       make([]time.Time, 0, config.FailureThreshold),
		lastResetTime:  time.Now(),
		stateChangeTime: time.Now(),
	}
}

// Execute 执行操作
func (cb *DefaultCircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	// 检查是否允许请求
	if !cb.Allow() {
		cb.mu.Lock()
		cb.stats.RejectedRequests++
		cb.mu.Unlock()
		
		log.Warn().
			Str("state", cb.GetState().String()).
			Msg("circuit breaker rejected request")
		
		return fmt.Errorf("circuit breaker is %s", cb.GetState().String())
	}
	
	// 执行操作
	cb.mu.Lock()
	cb.stats.TotalRequests++
	cb.mu.Unlock()
	
	startTime := time.Now()
	err := fn()
	duration := time.Since(startTime)
	
	cb.recordResult(err)
	
	if err != nil {
		log.Warn().
			Err(err).
			Str("state", cb.GetState().String()).
			Dur("duration", duration).
			Msg("circuit breaker operation failed")
	} else {
		log.Debug().
			Str("state", cb.GetState().String()).
			Dur("duration", duration).
			Msg("circuit breaker operation succeeded")
	}
	
	return err
}

// Allow 检查是否允许请求
func (cb *DefaultCircuitBreaker) Allow() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	switch cb.state {
	case StateClosed:
		return true
		
	case StateOpen:
		// 检查是否应该进入半开状态
		if time.Since(cb.stateChangeTime) >= cb.config.OpenStateTimeout {
			cb.mu.RUnlock()
			cb.transitionToHalfOpen()
			cb.mu.RLock()
			return cb.state == StateHalfOpen
		}
		return false
		
	case StateHalfOpen:
		// 半开状态下限制请求数量
		if cb.halfOpenReqs >= cb.config.HalfOpenMaxRequests {
			return false
		}
		return true
		
	default:
		return false
	}
}

// GetState 获取当前状态
func (cb *DefaultCircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetStats 获取统计信息
func (cb *DefaultCircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	stats := cb.stats
	stats.CurrentState = cb.state
	
	// 计算失败率
	if stats.TotalRequests > 0 {
		stats.FailureRate = float64(stats.FailedRequests) / float64(stats.TotalRequests) * 100
	}
	
	return stats
}

// Reset 重置熔断器
func (cb *DefaultCircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.resetInternal()
}

// ForceState 强制设置状态
func (cb *DefaultCircuitBreaker) ForceState(state CircuitState) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.transitionTo(state)
}

// recordResult 记录请求结果
func (cb *DefaultCircuitBreaker) recordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	now := time.Now()
	
	if err != nil {
		cb.stats.FailedRequests++
		cb.stats.LastFailureTime = now
		
		// 记录失败时间
		cb.failures = append(cb.failures, now)
		
		// 清理过期的失败记录
		cb.cleanupFailures(now)
		
		// 检查是否需要打开熔断器
		if cb.state == StateClosed && len(cb.failures) >= cb.config.FailureThreshold {
			cb.transitionTo(StateOpen)
		} else if cb.state == StateHalfOpen {
			// 半开状态下失败，重新打开熔断器
			cb.halfOpenReqs++
			cb.transitionTo(StateOpen)
		}
	} else {
		cb.stats.SuccessfulRequests++
		cb.stats.LastSuccessTime = now
		
		if cb.state == StateHalfOpen {
			cb.halfOpenReqs++
			cb.halfOpenSucc++
			
			// 检查是否应该关闭熔断器
			if cb.halfOpenSucc >= cb.config.HalfOpenSuccessThreshold {
				cb.transitionTo(StateClosed)
			} else if cb.halfOpenReqs >= cb.config.HalfOpenMaxRequests {
				// 达到最大尝试次数，重新打开熔断器
				cb.transitionTo(StateOpen)
			}
		}
		
		// 成功请求后清理失败记录
		cb.cleanupFailures(now)
	}
	
	// 检查是否需要重置统计信息
	if time.Since(cb.lastResetTime) >= cb.config.ResetTimeout {
		cb.resetInternal()
	}
}

// cleanupFailures 清理过期的失败记录
func (cb *DefaultCircuitBreaker) cleanupFailures(now time.Time) {
	cutoff := now.Add(-cb.config.FailureWindow)
	
	// 移除窗口外的失败记录
	i := 0
	for ; i < len(cb.failures); i++ {
		if cb.failures[i].After(cutoff) {
			break
		}
	}
	
	if i > 0 {
		cb.failures = cb.failures[i:]
	}
}

// transitionTo 转换状态
func (cb *DefaultCircuitBreaker) transitionTo(newState CircuitState) {
	if cb.state == newState {
		return
	}
	
	oldState := cb.state
	cb.state = newState
	cb.stateChangeTime = time.Now()
	cb.stats.StateChanges++
	
	log.Info().
		Str("old_state", oldState.String()).
		Str("new_state", newState.String()).
		Msg("circuit breaker state changed")
	
	switch newState {
	case StateOpen:
		cb.stats.OpenedAt = time.Now()
		// 重置半开状态计数器
		cb.halfOpenReqs = 0
		cb.halfOpenSucc = 0
		
	case StateHalfOpen:
		cb.stats.HalfOpenedAt = time.Now()
		// 重置半开状态计数器
		cb.halfOpenReqs = 0
		cb.halfOpenSucc = 0
		
	case StateClosed:
		// 重置失败记录
		cb.failures = make([]time.Time, 0, cb.config.FailureThreshold)
		cb.halfOpenReqs = 0
		cb.halfOpenSucc = 0
	}
}

// transitionToHalfOpen 转换到半开状态
func (cb *DefaultCircuitBreaker) transitionToHalfOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.transitionTo(StateHalfOpen)
}

// resetInternal 内部重置方法
func (cb *DefaultCircuitBreaker) resetInternal() {
	cb.state = StateClosed
	cb.failures = make([]time.Time, 0, cb.config.FailureThreshold)
	cb.halfOpenReqs = 0
	cb.halfOpenSucc = 0
	cb.lastResetTime = time.Now()
	cb.stateChangeTime = time.Now()
	
	// 重置统计信息（保留状态变化计数）
	stateChanges := cb.stats.StateChanges
	cb.stats = CircuitBreakerStats{
		CurrentState: StateClosed,
		StateChanges: stateChanges,
	}
	
	log.Info().Msg("circuit breaker reset")
}

// CircuitBreakerManager 熔断器管理器
type CircuitBreakerManager struct {
	breakers map[string]CircuitBreaker
	configs  map[string]CircuitBreakerConfig
	mu       sync.RWMutex
}

// NewCircuitBreakerManager 创建熔断器管理器
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]CircuitBreaker),
		configs:  make(map[string]CircuitBreakerConfig),
	}
}

// GetOrCreate 获取或创建熔断器
func (m *CircuitBreakerManager) GetOrCreate(key string, config CircuitBreakerConfig) CircuitBreaker {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if breaker, exists := m.breakers[key]; exists {
		return breaker
	}
	
	breaker := NewCircuitBreaker(config)
	m.breakers[key] = breaker
	m.configs[key] = config
	
	return breaker
}

// Get 获取熔断器
func (m *CircuitBreakerManager) Get(key string) (CircuitBreaker, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	breaker, exists := m.breakers[key]
	return breaker, exists
}

// UpdateConfig 更新熔断器配置
func (m *CircuitBreakerManager) UpdateConfig(key string, config CircuitBreakerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.configs[key] = config
	if breaker, exists := m.breakers[key]; exists {
		// 对于DefaultCircuitBreaker，需要重新创建
		// 在实际应用中，可以添加UpdateConfig方法
		delete(m.breakers, key)
		m.breakers[key] = NewCircuitBreaker(config)
	} else {
		m.breakers[key] = NewCircuitBreaker(config)
	}
}

// Remove 移除熔断器
func (m *CircuitBreakerManager) Remove(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	delete(m.breakers, key)
	delete(m.configs, key)
}

// GetAllStats 获取所有熔断器的统计信息
func (m *CircuitBreakerManager) GetAllStats() map[string]CircuitBreakerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := make(map[string]CircuitBreakerStats)
	for key, breaker := range m.breakers {
		stats[key] = breaker.GetStats()
	}
	return stats
}

// ResetAll 重置所有熔断器
func (m *CircuitBreakerManager) ResetAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, breaker := range m.breakers {
		breaker.Reset()
	}
}

// Monitor 监控熔断器状态
func (m *CircuitBreakerManager) Monitor() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[string]interface{})
	for key, breaker := range m.breakers {
		stats := breaker.GetStats()
		result[key] = map[string]interface{}{
			"state":        breaker.GetState().String(),
			"failure_rate": fmt.Sprintf("%.2f%%", stats.FailureRate),
			"total_requests": stats.TotalRequests,
			"failed_requests": stats.FailedRequests,
			"rejected_requests": stats.RejectedRequests,
		}
	}
	return result
}

// HTTPCircuitBreakerMiddleware HTTP熔断器中间件
type HTTPCircuitBreakerMiddleware struct {
	breaker CircuitBreaker
	next    http.Handler
}

// NewHTTPCircuitBreakerMiddleware 创建HTTP熔断器中间件
func NewHTTPCircuitBreakerMiddleware(breaker CircuitBreaker, next http.Handler) *HTTPCircuitBreakerMiddleware {
	return &HTTPCircuitBreakerMiddleware{
		breaker: breaker,
		next:    next,
	}
}

// ServeHTTP 实现http.Handler接口
func (m *HTTPCircuitBreakerMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 检查熔断器是否允许请求
	if !m.breaker.Allow() {
		log.Warn().
			Str("state", m.breaker.GetState().String()).
			Str("path", r.URL.Path).
			Msg("circuit breaker rejected HTTP request")
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    503,
			"message": "Service temporarily unavailable",
			"data":    nil,
		})
		return
	}
	
	// 记录响应状态码
	rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
	
	// 执行请求
	err := m.breaker.Execute(r.Context(), func() error {
		m.next.ServeHTTP(rw, r)
		
		// 检查HTTP状态码
		if rw.statusCode >= 500 {
			return fmt.Errorf("HTTP error %d", rw.statusCode)
		}
		return nil
	})
	
	if err != nil {
		log.Warn().
			Err(err).
			Str("path", r.URL.Path).
			Int("status", rw.statusCode).
			Msg("HTTP request failed through circuit breaker")
	}
}

// responseWriter 包装http.ResponseWriter以捕获状态码
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// http包导入
import (
	"encoding/json"
	"net/http"
)