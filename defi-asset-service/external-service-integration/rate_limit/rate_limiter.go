package rate_limit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	RequestsPerSecond float64       `json:"requests_per_second" yaml:"requests_per_second"`
	Burst             int           `json:"burst" yaml:"burst"`
	Window            time.Duration `json:"window" yaml:"window"`
	MaxQueueSize      int           `json:"max_queue_size" yaml:"max_queue_size"`
	MaxWaitTime       time.Duration `json:"max_wait_time" yaml:"max_wait_time"`
}

// DefaultRateLimitConfig 默认限流配置
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		RequestsPerSecond: 10.0,
		Burst:             20,
		Window:            time.Second,
		MaxQueueSize:      100,
		MaxWaitTime:       5 * time.Second,
	}
}

// RateLimiter 限流器接口
type RateLimiter interface {
	// Allow 检查是否允许请求（立即返回）
	Allow() bool
	
	// Wait 等待直到允许请求
	Wait(ctx context.Context) error
	
	// Reserve 预留一个请求，返回需要等待的时间
	Reserve() *Reservation
	
	// UpdateConfig 更新限流配置
	UpdateConfig(config RateLimitConfig)
	
	// GetStats 获取统计信息
	GetStats() RateLimitStats
}

// Reservation 请求预留
type Reservation struct {
	OK        bool
	Delay     time.Duration
	AllowedAt time.Time
}

// RateLimitStats 限流统计信息
type RateLimitStats struct {
	AllowedRequests  int64         `json:"allowed_requests"`
	RejectedRequests int64         `json:"rejected_requests"`
	TotalWaitTime    time.Duration `json:"total_wait_time"`
	AverageWaitTime  time.Duration `json:"average_wait_time"`
	LastRequestAt    time.Time     `json:"last_request_at"`
}

// TokenBucketRateLimiter 令牌桶限流器
type TokenBucketRateLimiter struct {
	limiter *rate.Limiter
	config  RateLimitConfig
	stats   RateLimitStats
	mu      sync.RWMutex
}

// NewTokenBucketRateLimiter 创建令牌桶限流器
func NewTokenBucketRateLimiter(config RateLimitConfig) *TokenBucketRateLimiter {
	limiter := rate.NewLimiter(rate.Limit(config.RequestsPerSecond), config.Burst)
	
	return &TokenBucketRateLimiter{
		limiter: limiter,
		config:  config,
		stats:   RateLimitStats{},
	}
}

// Allow 检查是否允许请求
func (rl *TokenBucketRateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	allowed := rl.limiter.Allow()
	if allowed {
		rl.stats.AllowedRequests++
	} else {
		rl.stats.RejectedRequests++
	}
	rl.stats.LastRequestAt = time.Now()
	
	return allowed
}

// Wait 等待直到允许请求
func (rl *TokenBucketRateLimiter) Wait(ctx context.Context) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	startTime := time.Now()
	err := rl.limiter.Wait(ctx)
	waitTime := time.Since(startTime)
	
	if err == nil {
		rl.stats.AllowedRequests++
		rl.stats.TotalWaitTime += waitTime
		if rl.stats.AllowedRequests > 0 {
			rl.stats.AverageWaitTime = rl.stats.TotalWaitTime / time.Duration(rl.stats.AllowedRequests)
		}
	} else {
		rl.stats.RejectedRequests++
	}
	rl.stats.LastRequestAt = time.Now()
	
	return err
}

// Reserve 预留一个请求
func (rl *TokenBucketRateLimiter) Reserve() *Reservation {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	reservation := rl.limiter.Reserve()
	if !reservation.OK() {
		rl.stats.RejectedRequests++
		rl.stats.LastRequestAt = time.Now()
		return &Reservation{OK: false}
	}
	
	delay := reservation.Delay()
	allowedAt := time.Now().Add(delay)
	
	rl.stats.AllowedRequests++
	rl.stats.TotalWaitTime += delay
	if rl.stats.AllowedRequests > 0 {
		rl.stats.AverageWaitTime = rl.stats.TotalWaitTime / time.Duration(rl.stats.AllowedRequests)
	}
	rl.stats.LastRequestAt = time.Now()
	
	return &Reservation{
		OK:        true,
		Delay:     delay,
		AllowedAt: allowedAt,
	}
}

// UpdateConfig 更新限流配置
func (rl *TokenBucketRateLimiter) UpdateConfig(config RateLimitConfig) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	rl.config = config
	rl.limiter.SetLimit(rate.Limit(config.RequestsPerSecond))
	rl.limiter.SetBurst(config.Burst)
}

// GetStats 获取统计信息
func (rl *TokenBucketRateLimiter) GetStats() RateLimitStats {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	
	return rl.stats
}

// ResetStats 重置统计信息
func (rl *TokenBucketRateLimiter) ResetStats() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	rl.stats = RateLimitStats{}
}

// MultiLimiter 多维度限流器
type MultiLimiter struct {
	limiters map[string]RateLimiter
	configs  map[string]RateLimitConfig
	mu       sync.RWMutex
}

// NewMultiLimiter 创建多维度限流器
func NewMultiLimiter() *MultiLimiter {
	return &MultiLimiter{
		limiters: make(map[string]RateLimiter),
		configs:  make(map[string]RateLimitConfig),
	}
}

// AddLimiter 添加限流器
func (ml *MultiLimiter) AddLimiter(key string, config RateLimitConfig) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	
	ml.configs[key] = config
	ml.limiters[key] = NewTokenBucketRateLimiter(config)
}

// Allow 检查是否允许请求（所有限流器都允许）
func (ml *MultiLimiter) Allow() bool {
	ml.mu.RLock()
	defer ml.mu.RUnlock()
	
	for _, limiter := range ml.limiters {
		if !limiter.Allow() {
			return false
		}
	}
	return true
}

// Wait 等待直到所有限流器都允许请求
func (ml *MultiLimiter) Wait(ctx context.Context) error {
	ml.mu.RLock()
	defer ml.mu.RUnlock()
	
	for key, limiter := range ml.limiters {
		if err := limiter.Wait(ctx); err != nil {
			log.Warn().
				Str("limiter", key).
				Err(err).
				Msg("rate limit wait failed")
			return fmt.Errorf("rate limit wait failed for %s: %w", key, err)
		}
	}
	return nil
}

// GetLimiter 获取指定键的限流器
func (ml *MultiLimiter) GetLimiter(key string) (RateLimiter, bool) {
	ml.mu.RLock()
	defer ml.mu.RUnlock()
	
	limiter, exists := ml.limiters[key]
	return limiter, exists
}

// UpdateLimiterConfig 更新限流器配置
func (ml *MultiLimiter) UpdateLimiterConfig(key string, config RateLimitConfig) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	
	ml.configs[key] = config
	if limiter, exists := ml.limiters[key]; exists {
		limiter.UpdateConfig(config)
	} else {
		ml.limiters[key] = NewTokenBucketRateLimiter(config)
	}
}

// RemoveLimiter 移除限流器
func (ml *MultiLimiter) RemoveLimiter(key string) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	
	delete(ml.configs, key)
	delete(ml.limiters, key)
}

// GetStats 获取所有限流器的统计信息
func (ml *MultiLimiter) GetStats() map[string]RateLimitStats {
	ml.mu.RLock()
	defer ml.mu.RUnlock()
	
	stats := make(map[string]RateLimitStats)
	for key, limiter := range ml.limiters {
		stats[key] = limiter.GetStats()
	}
	return stats
}

// WindowedRateLimiter 滑动窗口限流器
type WindowedRateLimiter struct {
	requests   []time.Time
	window     time.Duration
	maxRequests int
	mu         sync.Mutex
}

// NewWindowedRateLimiter 创建滑动窗口限流器
func NewWindowedRateLimiter(window time.Duration, maxRequests int) *WindowedRateLimiter {
	return &WindowedRateLimiter{
		requests:    make([]time.Time, 0, maxRequests),
		window:     window,
		maxRequests: maxRequests,
	}
}

// Allow 检查是否允许请求
func (wrl *WindowedRateLimiter) Allow() bool {
	wrl.mu.Lock()
	defer wrl.mu.Unlock()
	
	now := time.Now()
	windowStart := now.Add(-wrl.window)
	
	// 移除窗口外的请求
	i := 0
	for ; i < len(wrl.requests); i++ {
		if wrl.requests[i].After(windowStart) {
			break
		}
	}
	if i > 0 {
		wrl.requests = wrl.requests[i:]
	}
	
	// 检查是否超过最大请求数
	if len(wrl.requests) >= wrl.maxRequests {
		return false
	}
	
	// 添加当前请求
	wrl.requests = append(wrl.requests, now)
	return true
}

// GetRemainingRequests 获取剩余请求数
func (wrl *WindowedRateLimiter) GetRemainingRequests() int {
	wrl.mu.Lock()
	defer wrl.mu.Unlock()
	
	now := time.Now()
	windowStart := now.Add(-wrl.window)
	
	// 移除窗口外的请求
	i := 0
	for ; i < len(wrl.requests); i++ {
		if wrl.requests[i].After(windowStart) {
			break
		}
	}
	if i > 0 {
		wrl.requests = wrl.requests[i:]
	}
	
	return wrl.maxRequests - len(wrl.requests)
}

// GetNextResetTime 获取下次重置时间
func (wrl *WindowedRateLimiter) GetNextResetTime() time.Time {
	wrl.mu.Lock()
	defer wrl.mu.Unlock()
	
	if len(wrl.requests) == 0 {
		return time.Now()
	}
	
	oldestRequest := wrl.requests[0]
	return oldestRequest.Add(wrl.window)
}

// RateLimitMiddleware HTTP限流中间件
type RateLimitMiddleware struct {
	limiter RateLimiter
	next    http.Handler
}

// NewRateLimitMiddleware 创建限流中间件
func NewRateLimitMiddleware(limiter RateLimiter, next http.Handler) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		limiter: limiter,
		next:    next,
	}
}

// ServeHTTP 实现http.Handler接口
func (m *RateLimitMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// 设置超时上下文
	if m.limiter.(*TokenBucketRateLimiter).config.MaxWaitTime > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.limiter.(*TokenBucketRateLimiter).config.MaxWaitTime)
		defer cancel()
	}
	
	// 等待限流器允许请求
	if err := m.limiter.Wait(ctx); err != nil {
		log.Warn().
			Err(err).
			Str("path", r.URL.Path).
			Str("method", r.Method).
			Msg("rate limit exceeded")
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    429,
			"message": "Rate limit exceeded",
			"data":    nil,
		})
		return
	}
	
	// 调用下一个处理器
	m.next.ServeHTTP(w, r)
}

// http包导入
import (
	"encoding/json"
	"net/http"
)