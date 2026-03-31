package retry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/rs/zerolog/log"
)

// RetryableError 可重试错误
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("retryable error: %v", e.Err)
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// NonRetryableError 不可重试错误
type NonRetryableError struct {
	Err error
}

func (e *NonRetryableError) Error() string {
	return fmt.Sprintf("non-retryable error: %v", e.Err)
}

func (e *NonRetryableError) Unwrap() error {
	return e.Err
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxAttempts      int           `json:"max_attempts" yaml:"max_attempts"`
	InitialDelay     time.Duration `json:"initial_delay" yaml:"initial_delay"`
	MaxDelay         time.Duration `json:"max_delay" yaml:"max_delay"`
	Multiplier       float64       `json:"multiplier" yaml:"multiplier"`
	Jitter           float64       `json:"jitter" yaml:"jitter"`
	RetryableStatusCodes []int     `json:"retryable_status_codes" yaml:"retryable_status_codes"`
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:      3,
		InitialDelay:     100 * time.Millisecond,
		MaxDelay:         10 * time.Second,
		Multiplier:       2.0,
		Jitter:           0.1,
		RetryableStatusCodes: []int{
			408, // Request Timeout
			429, // Too Many Requests
			500, // Internal Server Error
			502, // Bad Gateway
			503, // Service Unavailable
			504, // Gateway Timeout
		},
	}
}

// RetryStats 重试统计信息
type RetryStats struct {
	Attempts      int           `json:"attempts"`
	TotalDelay    time.Duration `json:"total_delay"`
	LastError     error         `json:"last_error"`
	LastAttemptAt time.Time     `json:"last_attempt_at"`
}

// Retry 重试执行函数
type Retry func(ctx context.Context) error

// RetryManager 重试管理器
type RetryManager struct {
	config RetryConfig
	stats  RetryStats
}

// NewRetryManager 创建重试管理器
func NewRetryManager(config RetryConfig) *RetryManager {
	return &RetryManager{
		config: config,
		stats:  RetryStats{},
	}
}

// Execute 执行重试操作
func (rm *RetryManager) Execute(ctx context.Context, fn Retry) error {
	var lastErr error
	
	for attempt := 1; attempt <= rm.config.MaxAttempts; attempt++ {
		rm.stats.Attempts = attempt
		rm.stats.LastAttemptAt = time.Now()
		
		// 执行操作
		err := fn(ctx)
		if err == nil {
			log.Debug().
				Int("attempt", attempt).
				Msg("operation succeeded")
			return nil
		}
		
		lastErr = err
		rm.stats.LastError = err
		
		// 检查是否可重试
		if !rm.isRetryable(err) {
			log.Warn().
				Int("attempt", attempt).
				Err(err).
				Msg("non-retryable error, stopping retries")
			return &NonRetryableError{Err: err}
		}
		
		// 如果是最后一次尝试，直接返回错误
		if attempt == rm.config.MaxAttempts {
			log.Error().
				Int("attempt", attempt).
				Err(err).
				Msg("max retry attempts reached")
			return &RetryableError{Err: fmt.Errorf("max retry attempts reached: %w", err)}
		}
		
		// 计算延迟时间
		delay := rm.calculateDelay(attempt)
		rm.stats.TotalDelay += delay
		
		log.Warn().
			Int("attempt", attempt).
			Err(err).
			Dur("delay", delay).
			Int("max_attempts", rm.config.MaxAttempts).
			Msg("operation failed, retrying")
		
		// 等待延迟时间
		if err := rm.wait(ctx, delay); err != nil {
			return err
		}
	}
	
	return lastErr
}

// ExecuteWithResult 执行重试操作并返回结果
func ExecuteWithResult[T any](ctx context.Context, config RetryConfig, fn func(ctx context.Context) (T, error)) (T, error) {
	var zero T
	rm := NewRetryManager(config)
	
	var result T
	err := rm.Execute(ctx, func(ctx context.Context) error {
		var fnErr error
		result, fnErr = fn(ctx)
		return fnErr
	})
	
	if err != nil {
		return zero, err
	}
	
	return result, nil
}

// isRetryable 检查错误是否可重试
func (rm *RetryManager) isRetryable(err error) bool {
	// 检查是否为可重试错误类型
	var retryableErr *RetryableError
	var nonRetryableErr *NonRetryableError
	
	if errors.As(err, &nonRetryableErr) {
		return false
	}
	
	if errors.As(err, &retryableErr) {
		return true
	}
	
	// 检查HTTP状态码
	if httpErr, ok := err.(interface{ StatusCode() int }); ok {
		statusCode := httpErr.StatusCode()
		for _, retryableCode := range rm.config.RetryableStatusCodes {
			if statusCode == retryableCode {
				return true
			}
		}
	}
	
	// 默认情况下，网络错误和超时错误可重试
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return false
	}
	
	// 常见的网络错误
	errStr := err.Error()
	networkErrors := []string{
		"connection refused",
		"connection reset",
		"timeout",
		"deadline exceeded",
		"temporary failure",
		"network is unreachable",
	}
	
	for _, networkErr := range networkErrors {
		if contains(errStr, networkErr) {
			return true
		}
	}
	
	return false
}

// calculateDelay 计算延迟时间
func (rm *RetryManager) calculateDelay(attempt int) time.Duration {
	// 指数退避算法
	delay := float64(rm.config.InitialDelay) * math.Pow(rm.config.Multiplier, float64(attempt-1))
	
	// 添加抖动
	if rm.config.Jitter > 0 {
		jitter := rand.Float64() * rm.config.Jitter * delay
		delay += jitter
	}
	
	// 限制最大延迟
	if delay > float64(rm.config.MaxDelay) {
		delay = float64(rm.config.MaxDelay)
	}
	
	return time.Duration(delay)
}

// wait 等待延迟时间
func (rm *RetryManager) wait(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetStats 获取重试统计信息
func (rm *RetryManager) GetStats() RetryStats {
	return rm.stats
}

// ResetStats 重置统计信息
func (rm *RetryManager) ResetStats() {
	rm.stats = RetryStats{}
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

// HTTPRetryWrapper HTTP重试包装器
type HTTPRetryWrapper struct {
	client *http.Client
	config RetryConfig
}

// NewHTTPRetryWrapper 创建HTTP重试包装器
func NewHTTPRetryWrapper(client *http.Client, config RetryConfig) *HTTPRetryWrapper {
	return &HTTPRetryWrapper{
		client: client,
		config: config,
	}
}

// DoWithRetry 执行HTTP请求并重试
func (w *HTTPRetryWrapper) DoWithRetry(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error
	
	_, err = ExecuteWithResult(req.Context(), w.config, func(ctx context.Context) (*http.Response, error) {
		// 创建新的请求副本，避免修改原始请求
		reqCopy := req.Clone(ctx)
		
		// 执行请求
		resp, err := w.client.Do(reqCopy)
		if err != nil {
			return nil, err
		}
		
		// 检查状态码是否可重试
		for _, retryableCode := range w.config.RetryableStatusCodes {
			if resp.StatusCode == retryableCode {
				resp.Body.Close()
				return nil, &RetryableError{Err: fmt.Errorf("HTTP status %d", resp.StatusCode)}
			}
		}
		
		// 2xx状态码表示成功
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}
		
		// 其他状态码视为不可重试错误
		resp.Body.Close()
		return nil, &NonRetryableError{Err: fmt.Errorf("HTTP status %d", resp.StatusCode)}
	})
	
	return resp, err
}