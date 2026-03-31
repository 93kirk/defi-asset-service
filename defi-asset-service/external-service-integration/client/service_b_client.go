package client

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	
	"root/.openclaw/workspace/defi-asset-service/external-service-integration/circuit_breaker"
	"root/.openclaw/workspace/defi-asset-service/external-service-integration/rate_limit"
	"root/.openclaw/workspace/defi-asset-service/external-service-integration/retry"
)

// ServiceBConfig 服务B配置
type ServiceBConfig struct {
	BaseURL        string        `json:"base_url" yaml:"base_url"`
	APIKey         string        `json:"api_key" yaml:"api_key"`
	Timeout        time.Duration `json:"timeout" yaml:"timeout"`
	MaxConcurrency int           `json:"max_concurrency" yaml:"max_concurrency"`
	
	// 缓存配置
	CacheTTL       time.Duration `json:"cache_ttl" yaml:"cache_ttl"`
	CachePrefix    string        `json:"cache_prefix" yaml:"cache_prefix"`
	
	// 重试配置
	RetryConfig retry.RetryConfig `json:"retry_config" yaml:"retry_config"`
	
	// 限流配置
	RateLimitConfig rate_limit.RateLimitConfig `json:"rate_limit_config" yaml:"rate_limit_config"`
	
	// 熔断器配置
	CircuitBreakerConfig circuit_breaker.CircuitBreakerConfig `json:"circuit_breaker_config" yaml:"circuit_breaker_config"`
}

// DefaultServiceBConfig 默认服务B配置
func DefaultServiceBConfig() ServiceBConfig {
	return ServiceBConfig{
		BaseURL:        "https://api.service-b.com/v1",
		Timeout:        15 * time.Second,
		MaxConcurrency: 5,
		CacheTTL:       10 * time.Minute,
		CachePrefix:    "defi:position",
		RetryConfig:    retry.DefaultRetryConfig(),
		RateLimitConfig: rate_limit.DefaultRateLimitConfig(),
		CircuitBreakerConfig: circuit_breaker.DefaultCircuitBreakerConfig(),
	}
}

// PositionData 仓位数据
type PositionData struct {
	PositionID         string          `json:"position_id" gorm:"column:position_id"`
	ProtocolID         string          `json:"protocol_id" gorm:"column:protocol_id"`
	ProtocolName       string          `json:"protocol_name" gorm:"-"`
	PositionType       string          `json:"position_type" gorm:"column:position_type"`
	TokenAddress       string          `json:"token_address" gorm:"column:token_address"`
	TokenSymbol        string          `json:"token_symbol" gorm:"column:token_symbol"`
	TokenName          string          `json:"token_name" gorm:"column:token_name"`
	AmountRaw          string          `json:"amount_raw" gorm:"column:amount_raw"`
	Amount             string          `json:"amount" gorm:"column:amount_decimal"`
	PriceUSD           string          `json:"price_usd" gorm:"column:price_usd"`
	ValueUSD           string          `json:"value_usd" gorm:"column:value_usd"`
	APY                string          `json:"apy" gorm:"column:apy"`
	HealthFactor       string          `json:"health_factor" gorm:"column:health_factor"`
	LiquidationThreshold string        `json:"liquidation_threshold" gorm:"column:liquidation_threshold"`
	CollateralFactor   string          `json:"collateral_factor" gorm:"column:collateral_factor"`
	PositionDataRaw    json.RawMessage `json:"position_data" gorm:"column:position_data;type:json"`
	IsActive           bool            `json:"is_active" gorm:"column:is_active"`
	LastUpdatedAt      string          `json:"last_updated_at" gorm:"column:last_updated_at"`
}

// UserPositionsResponse 用户仓位响应
type UserPositionsResponse struct {
	Address        string         `json:"address"`
	ChainID        int            `json:"chain_id"`
	TotalValueUSD  string         `json:"total_value_usd"`
	Positions      []PositionData `json:"positions"`
	Cached         bool           `json:"cached"`
	CacheExpiresAt string         `json:"cache_expires_at,omitempty"`
	LastUpdatedAt  string         `json:"last_updated_at"`
}

// ProtocolPositionsResponse 协议仓位响应
type ProtocolPositionsResponse struct {
	ProtocolID     string         `json:"protocol_id"`
	ProtocolName   string         `json:"protocol_name"`
	TotalValueUSD  string         `json:"total_value_usd"`
	Positions      []PositionData `json:"positions"`
	UserCount      int            `json:"user_count"`
	LastUpdatedAt  string         `json:"last_updated_at"`
}

// ServiceBClient 服务B客户端
type ServiceBClient struct {
	httpClient      *HTTPClient
	db              *gorm.DB
	redisClient     *redis.Client
	config          ServiceBConfig
	rateLimiter     rate_limit.RateLimiter
	circuitBreaker  circuit_breaker.CircuitBreaker
	retryManager    *retry.RetryManager
}

// NewServiceBClient 创建服务B客户端
func NewServiceBClient(config ServiceBConfig, db *gorm.DB, redisClient *redis.Client) (*ServiceBClient, error) {
	// 创建HTTP客户端
	httpConfig := DefaultHTTPClientConfig()
	httpConfig.Timeout = config.Timeout
	httpConfig.MaxConnsPerHost = config.MaxConcurrency
	
	httpClient, err := NewHTTPClient(config.BaseURL, httpConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}
	
	// 创建限流器
	rateLimiter := rate_limit.NewTokenBucketRateLimiter(config.RateLimitConfig)
	
	// 创建熔断器
	circuitBreaker := circuit_breaker.NewCircuitBreaker(config.CircuitBreakerConfig)
	
	// 创建重试管理器
	retryManager := retry.NewRetryManager(config.RetryConfig)
	
	return &ServiceBClient{
		httpClient:     httpClient,
		db:             db,
		redisClient:    redisClient,
		config:         config,
		rateLimiter:    rateLimiter,
		circuitBreaker: circuitBreaker,
		retryManager:   retryManager,
	}, nil
}

// GetUserPositions 获取用户仓位数据（带缓存）
func (c *ServiceBClient) GetUserPositions(ctx context.Context, address string, chainID int, forceRefresh bool) (*UserPositionsResponse, error) {
	log.Info().
		Str("address", address).
		Int("chain_id", chainID).
		Bool("force_refresh", forceRefresh).
		Msg("fetching user positions from service B")
	
	// 如果不强制刷新，先检查缓存
	if !forceRefresh {
		if cached, err := c.getFromCache(ctx, address, chainID); err == nil && cached != nil {
			log.Info().
				Str("address", address).
				Int("chain_id", chainID).
				Msg("returning cached user positions")
			return cached, nil
		}
	}
	
	// 构建请求参数
	queryParams := map[string]string{
		"address":  address,
		"chain_id": fmt.Sprintf("%d", chainID),
	}
	
	headers := c.getHeaders()
	
	// 执行受保护的请求
	var response *UserPositionsResponse
	err := c.executeProtectedRequest(ctx, func() error {
		// 等待限流器
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return &retry.NonRetryableError{Err: fmt.Errorf("rate limit wait failed: %w", err)}
		}
		
		// 执行HTTP请求
		resp, err := c.httpClient.Get("/positions", queryParams, headers)
		if err != nil {
			return &retry.RetryableError{Err: fmt.Errorf("HTTP request failed: %w", err)}
		}
		
		// 解析响应
		result, err := ParseJSONResponse[UserPositionsResponse](resp)
		if err != nil {
			return &retry.NonRetryableError{Err: fmt.Errorf("failed to parse response: %w", err)}
		}
		
		response = result
		response.Cached = false
		
		return nil
	})
	
	if err != nil {
		log.Error().
			Err(err).
			Str("address", address).
			Int("chain_id", chainID).
			Msg("failed to get user positions from service B")
		
		// 如果外部服务失败，尝试从数据库获取
		if dbData, dbErr := c.getFromDatabase(ctx, address, chainID); dbErr == nil && dbData != nil {
			log.Warn().
				Str("address", address).
				Int("chain_id", chainID).
				Msg("falling back to database data")
			dbData.Cached = true
			return dbData, nil
		}
		
		return nil, err
	}
	
	// 保存到数据库
	if err := c.saveToDatabase(ctx, address, chainID, response); err != nil {
		log.Error().
			Err(err).
			Str("address", address).
			Int("chain_id", chainID).
			Msg("failed to save positions to database")
	}
	
	// 保存到缓存
	if err := c.saveToCache(ctx, address, chainID, response); err != nil {
		log.Error().
			Err(err).
			Str("address", address).
			Int("chain_id", chainID).
			Msg("failed to cache positions")
	}
	
	log.Info().
		Str("address", address).
		Int("chain_id", chainID).
		Str("total_value", response.TotalValueUSD).
		Int("position_count", len(response.Positions)).
		Msg("successfully fetched user positions from service B")
	
	return response, nil
}

// GetUserPositionsByProtocol 获取用户在特定协议的仓位
func (c *ServiceBClient) GetUserPositionsByProtocol(ctx context.Context, address string, chainID int, protocolID string) (*ProtocolPositionsResponse, error) {
	log.Info().
		Str("address", address).
		Int("chain_id", chainID).
		Str("protocol_id", protocolID).
		Msg("fetching user protocol positions from service B")
	
	// 构建请求参数
	queryParams := map[string]string{
		"address":     address,
		"chain_id":    fmt.Sprintf("%d", chainID),
		"protocol_id": protocolID,
	}
	
	headers := c.getHeaders()
	
	// 执行受保护的请求
	var response *ProtocolPositionsResponse
	err := c.executeProtectedRequest(ctx, func() error {
		// 等待限流器
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return &retry.NonRetryableError{Err: fmt.Errorf("rate limit wait failed: %w", err)}
		}
		
		// 执行HTTP请求
		resp, err := c.httpClient.Get("/positions/protocol", queryParams, headers)
		if err != nil {
			return &retry.RetryableError{Err: fmt.Errorf("HTTP request failed: %w", err)}
		}
		
		// 解析响应
		result, err := ParseJSONResponse[ProtocolPositionsResponse](resp)
		if err != nil {
			return &retry.NonRetryableError{Err: fmt.Errorf("failed to parse response: %w", err)}
		}
		
		response = result
		return nil
	})
	
	if err != nil {
		log.Error().
			Err(err).
			Str("address", address).
			Int("chain_id", chainID).
			Str("protocol_id", protocolID).
			Msg("failed to get user protocol positions from service B")
		return nil, err
	}
	
	log.Info().
		Str("address", address).
		Int("chain_id", chainID).
		Str("protocol_id", protocolID).
		Str("total_value", response.TotalValueUSD).
		Int("position_count", len(response.Positions)).
		Msg("successfully fetched user protocol positions from service B")
	
	return response, nil
}

// BatchGetUserPositions 批量获取用户仓位
func (c *ServiceBClient) BatchGetUserPositions(ctx context.Context, addresses []string, chainID int) (map[string]*UserPositionsResponse, error) {
	log.Info().
		Int("address_count", len(addresses)).
		Int("chain_id", chainID).
		Msg("batch fetching user positions from service B")
	
	// 构建请求体
	requestBody := map[string]interface{}{
		"addresses": addresses,
		"chain_id":  chainID,
	}
	
	headers := c.getHeaders()
	
	// 执行受保护的请求
	var batchResponse struct {
		Results []*UserPositionsResponse `json:"results"`
	}
	
	err := c.executeProtectedRequest(ctx, func() error {
		// 等待限流器
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return &retry.NonRetryableError{Err: fmt.Errorf("rate limit wait failed: %w", err)}
		}
		
		// 执行HTTP请求
		resp, err := c.httpClient.Post("/positions/batch", requestBody, headers)
		if err != nil {
			return &retry.RetryableError{Err: fmt.Errorf("HTTP request failed: %w", err)}
		}
		
		// 解析响应
		if err := json.Unmarshal(resp.Body, &batchResponse); err != nil {
			return &retry.NonRetryableError{Err: fmt.Errorf("failed to parse batch response: %w", err)}
		}
		
		return nil
	})
	
	if err != nil {
		log.Error().
			Err(err).
			Int("address_count", len(addresses)).
			Int("chain_id", chainID).
			Msg("failed to batch get user positions from service B")
		return nil, err
	}
	
	// 构建地址到响应的映射
	result := make(map[string]*UserPositionsResponse)
	for _, response := range batchResponse.Results {
		result[response.Address] = response
		
		// 异步保存到数据库和缓存
		go func(addr string, resp *UserPositionsResponse) {
			ctx := context.Background()
			if err := c.saveToDatabase(ctx, addr, chainID, resp); err != nil {
				log.Error().Err(err).Str("address", addr).Msg("failed to save batch positions to database")
			}
			if err := c.saveToCache(ctx, addr, chainID, resp); err != nil {
				log.Error().Err(err).Str("address", addr).Msg("failed to cache batch positions")
			}
		}(response.Address, response)
	}
	
	log.Info().
		Int("address_count", len(addresses)).
		Int("chain_id", chainID).
		Int("success_count", len(result)).
		Msg("successfully batch fetched user positions from service B")
	
	return result, nil
}

// UpdatePosition 更新仓位数据（用于队列处理）
func (c *ServiceBClient) UpdatePosition(ctx context.Context, positionData PositionData) error {
	log.Info().
		Str("position_id", positionData.PositionID).
		Str("protocol_id", positionData.ProtocolID).
		Msg("updating position data")
	
	// 保存到数据库
	if err := c.updatePositionInDatabase(ctx, positionData); err != nil {
		log.Error().
			Err(err).
			Str("position_id", positionData.PositionID).
			Msg("failed to update position in database")
		return err
	}
	
	// 清除相关缓存
	if err := c.clearPositionCache(ctx, positionData); err != nil {
		log.Warn().
			Err(err).
			Str("position_id", positionData.PositionID).
			Msg("failed to clear position cache")
	}
	
	log.Info().
		Str("position_id", positionData.PositionID).
		Str("protocol_id", positionData.ProtocolID).
		Msg("successfully updated position data")
	
	return nil
}

// HealthCheck 健康检查
func (c *ServiceBClient) HealthCheck(ctx context.Context) error {
	log.Debug().Msg("checking service B health")
	
	return c.executeProtectedRequest(ctx, func() error {
		// 等待限流器
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit wait failed: %w", err)
		}
		
		// 执行健康检查
		resp, err := c.httpClient.Get("/health", nil, c.getHeaders())
		if err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}
		
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("health check returned status %d", resp.StatusCode)
		}
		
		log.Debug().Msg("service B health check passed")
		return nil
	})
}

// GetStats 获取客户端统计信息
func (c *ServiceBClient) GetStats() map[string]interface{} {
	rateLimitStats := c.rateLimiter.GetStats()
	circuitBreakerStats := c.circuitBreaker.GetStats()
	retryStats := c.retryManager.GetStats()
	
	return map[string]interface{}{
		"rate_limiter": map[string]interface{}{
			"allowed_requests":  rateLimitStats.AllowedRequests,
			"rejected_requests