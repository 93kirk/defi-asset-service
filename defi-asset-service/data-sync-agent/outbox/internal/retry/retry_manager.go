package retry

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// RetryManager 重试管理器
type RetryManager struct {
	maxRetries   int
	baseDelay    time.Duration
	maxDelay     time.Duration
	jitterFactor float64
	logger       *slog.Logger
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries   int           `yaml:"max_retries"`
	BaseDelay    time.Duration `yaml:"base_delay"`
	MaxDelay     time.Duration `yaml:"max_delay"`
	JitterFactor float64       `yaml:"jitter_factor"` // 0-1之间的抖动因子
}

// NewRetryManager 创建新的重试管理器
func NewRetryManager(config *RetryConfig, logger *slog.Logger) *RetryManager {
	if config == nil {
		config = &RetryConfig{
			MaxRetries:   3,
			BaseDelay:    1 * time.Second,
			MaxDelay:     30 * time.Second,
			JitterFactor: 0.2,
		}
	}

	return &RetryManager{
		maxRetries:   config.MaxRetries,
		baseDelay:    config.BaseDelay,
		maxDelay:     config.MaxDelay,
		jitterFactor: config.JitterFactor,
		logger:       logger,
	}
}

// Do 执行带重试的操作
func (r *RetryManager) Do(ctx context.Context, operationName string, fn func() error) error {
	var lastErr error
	
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		// 执行操作
		err := fn()
		if err == nil {
			// 成功，返回
			if attempt > 0 {
				r.logger.Info("操作重试成功",
					"operation", operationName,
					"attempt", attempt+1)
			}
			return nil
		}

		lastErr = err
		
		// 如果是最后一次尝试，不再等待
		if attempt == r.maxRetries {
			break
		}

		// 计算下一次重试的延迟
		delay := r.calculateDelay(attempt)
		
		r.logger.Warn("操作失败，准备重试",
			"operation", operationName,
			"attempt", attempt+1,
			"max_retries", r.maxRetries,
			"error", err,
			"next_retry_in", delay)

		// 等待重试
		if err := r.waitForRetry(ctx, delay); err != nil {
			return fmt.Errorf("等待重试时被取消: %w", err)
		}
	}

	return fmt.Errorf("操作失败，达到最大重试次数 (%d): %w", r.maxRetries, lastErr)
}

// DoWithResult 执行带重试的操作并返回结果
func (r *RetryManager) DoWithResult[T any](ctx context.Context, operationName string, fn func() (T, error)) (T, error) {
	var zero T
	var lastErr error
	
	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		// 执行操作
		result, err := fn()
		if err == nil {
			// 成功，返回结果
			if attempt > 0 {
				r.logger.Info("操作重试成功",
					"operation", operationName,
					"attempt", attempt+1)
			}
			return result, nil
		}

		lastErr = err
		
		// 如果是最后一次尝试，不再等待
		if attempt == r.maxRetries {
			break
		}

		// 计算下一次重试的延迟
		delay := r.calculateDelay(attempt)
		
		r.logger.Warn("操作失败，准备重试",
			"operation", operationName,
			"attempt", attempt+1,
			"max_retries", r.maxRetries,
			"error", err,
			"next_retry_in", delay)

		// 等待重试
		if err := r.waitForRetry(ctx, delay); err != nil {
			return zero, fmt.Errorf("等待重试时被取消: %w", err)
		}
	}

	return zero, fmt.Errorf("操作失败，达到最大重试次数 (%d): %w", r.maxRetries, lastErr)
}

// calculateDelay 计算重试延迟
func (r *RetryManager) calculateDelay(attempt int) time.Duration {
	// 指数退避：baseDelay * 2^attempt
	delay := float64(r.baseDelay) * math.Pow(2, float64(attempt))
	
	// 添加随机抖动
	if r.jitterFactor > 0 {
		jitter := 1 + r.jitterFactor*(rand.Float64()*2-1) // 在 [1-jitter, 1+jitter] 范围内
		delay *= jitter
	}
	
	// 限制最大延迟
	if delay > float64(r.maxDelay) {
		delay = float64(r.maxDelay)
	}
	
	return time.Duration(delay)
}

// waitForRetry 等待重试
func (r *RetryManager) waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// IsRetryableError 判断错误是否可重试
func (r *RetryManager) IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// 网络错误通常可重试
	errStr := err.Error()
	retryableErrors := []string{
		"timeout",
		"deadline exceeded",
		"connection refused",
		"connection reset",
		"network is unreachable",
		"no route to host",
		"temporary failure",
		"too many connections",
		"service unavailable",
		"gateway timeout",
		"bad gateway",
		"EOF",
	}

	for _, retryableErr := range retryableErrors {
		if containsIgnoreCase(errStr, retryableErr) {
			return true
		}
	}

	return false
}

// containsIgnoreCase 检查字符串是否包含子字符串（忽略大小写）
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// RetryPolicy 重试策略接口
type RetryPolicy interface {
	ShouldRetry(attempt int, err error) bool
	GetDelay(attempt int) time.Duration
}

// ExponentialBackoffPolicy 指数退避策略
type ExponentialBackoffPolicy struct {
	MaxRetries   int
	BaseDelay    time.Duration
	MaxDelay     time.Duration
	JitterFactor float64
}

// ShouldRetry 判断是否应该重试
func (p *ExponentialBackoffPolicy) ShouldRetry(attempt int, err error) bool {
	return attempt < p.MaxRetries && err != nil
}

// GetDelay 获取重试延迟
func (p *ExponentialBackoffPolicy) GetDelay(attempt int) time.Duration {
	delay := float64(p.BaseDelay) * math.Pow(2, float64(attempt))
	
	if p.JitterFactor > 0 {
		jitter := 1 + p.JitterFactor*(rand.Float64()*2-1)
		delay *= jitter
	}
	
	if delay > float64(p.MaxDelay) {
		delay = float64(p.MaxDelay)
	}
	
	return time.Duration(delay)
}

// FixedDelayPolicy 固定延迟策略
type FixedDelayPolicy struct {
	MaxRetries int
	Delay      time.Duration
}

// ShouldRetry 判断是否应该重试
func (p *FixedDelayPolicy) ShouldRetry(attempt int, err error) bool {
	return attempt < p.MaxRetries && err != nil
}

// GetDelay 获取重试延迟
func (p *FixedDelayPolicy) GetDelay(attempt int) time.Duration {
	return p.Delay
}

// NoRetryPolicy 不重试策略
type NoRetryPolicy struct{}

// ShouldRetry 判断是否应该重试
func (p *NoRetryPolicy) ShouldRetry(attempt int, err error) bool {
	return false
}

// GetDelay 获取重试延迟
func (p *NoRetryPolicy) GetDelay(attempt int) time.Duration {
	return 0
}

// WithRetry 使用指定策略执行带重试的操作
func WithRetry(ctx context.Context, policy RetryPolicy, operationName string, fn func() error) error {
	var lastErr error
	
	for attempt := 0; ; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}
		
		lastErr = err
		
		if !policy.ShouldRetry(attempt, err) {
			break
		}
		
		delay := policy.GetDelay(attempt)
		timer := time.NewTimer(delay)
		
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("操作被取消: %w", ctx.Err())
		case <-timer.C:
			// 继续重试
		}
	}
	
	return fmt.Errorf("操作失败: %w", lastErr)
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	failureThreshold int
	resetTimeout     time.Duration
	failureCount     int
	lastFailureTime  time.Time
	state            CircuitState
	mu               sync.RWMutex
}

// CircuitState 熔断器状态
type CircuitState int

const (
	StateClosed   CircuitState = iota // 闭合状态，正常请求
	StateOpen                         // 打开状态，拒绝所有请求
	StateHalfOpen                     // 半开状态，允许部分请求测试
)

// NewCircuitBreaker 创建新的熔断器
func NewCircuitBreaker(failureThreshold int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		failureThreshold: failureThreshold,
		resetTimeout:     resetTimeout,
		state:            StateClosed,
	}
}

// Execute 执行操作，受熔断器保护
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.RLock()
	state := cb.state
	cb.mu.RUnlock()

	// 检查熔断器状态
	switch state {
	case StateOpen:
		// 检查是否应该进入半开状态
		cb.mu.RLock()
		timeSinceLastFailure := time.Since(cb.lastFailureTime)
		cb.mu.RUnlock()
		
		if timeSinceLastFailure >= cb.resetTimeout {
			cb.mu.Lock()
			cb.state = StateHalfOpen
			cb.mu.Unlock()
			// 继续执行，进入半开状态
		} else {
			return fmt.Errorf("熔断器打开，拒绝请求")
		}
	case StateHalfOpen:
		// 半开状态，允许一个请求通过
		// 继续执行
	case StateClosed:
		// 闭合状态，正常执行
		// 继续执行
	}

	// 执行操作
	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	// 更新熔断器状态
	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = time.Now()
		
		if cb.state == StateHalfOpen {
			// 半开状态下失败，重新打开
			cb.state = StateOpen
		} else if cb.failureCount >= cb.failureThreshold {
			// 达到失败阈值，打开熔断器
			cb.state = StateOpen
		}
	} else {
		// 成功，重置熔断器
		if cb.state == StateHalfOpen {
			// 半开状态下成功，恢复闭合状态
			cb.state = StateClosed
			cb.failureCount = 0
		} else {
			// 闭合状态下成功，重置失败计数
			cb.failureCount = 0
		}
	}

	return err
}

// GetState 获取熔断器状态
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// GetFailureCount 获取失败计数
func (cb *CircuitBreaker) GetFailureCount() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failureCount
}

// Reset 重置熔断器
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.state = StateClosed
	cb.failureCount = 0
	cb.lastFailureTime = time.Time{}
}