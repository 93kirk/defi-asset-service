package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"root/.openclaw/workspace/defi-asset-service/external-service-integration/circuit_breaker"
	"root/.openclaw/workspace/defi-asset-service/external-service-integration/rate_limit"
	"root/.openclaw/workspace/defi-asset-service/external-service-integration/retry"
)

// ServiceAConfig 服务A配置
type ServiceAConfig struct {
	BaseURL        string        `json:"base_url" yaml:"base_url"`
	APIKey         string        `json:"api_key" yaml:"api_key"`
	Timeout        time.Duration `json:"timeout" yaml:"timeout"`
	MaxConcurrency int           `json:"max_concurrency" yaml:"max_concurrency"`
	
	// 重试配置
	RetryConfig retry.RetryConfig `json:"retry_config" yaml:"retry_config"`
	
	// 限流配置
	RateLimitConfig rate_limit.RateLimitConfig `json:"rate_limit_config" yaml:"rate_limit_config"`
	
	// 熔断器配置
	CircuitBreakerConfig circuit_breaker.CircuitBreakerConfig `json:"circuit_breaker_config" yaml:"circuit_breaker_config"`
}

// DefaultServiceAConfig 默认服务A配置
func DefaultServiceAConfig() ServiceAConfig {
	return ServiceAConfig{
		BaseURL:        "https://api.service-a.com/v1",
		Timeout:        10 * time.Second,
		MaxConcurrency: 10,
		RetryConfig:    retry.DefaultRetryConfig(),
		RateLimitConfig: rate_limit.DefaultRateLimitConfig(),
		CircuitBreakerConfig: circuit_breaker.DefaultCircuitBreakerConfig(),
	}
}

// AssetBalance 资产余额
type AssetBalance struct {
	TokenAddress  string `json:"token_address"`
	TokenSymbol   string `json:"token_symbol"`
	TokenName     string `json:"token_name"`
	TokenDecimals int    `json:"token_decimals"`
	BalanceRaw    string `json:"balance_raw"`
	Balance       string `json:"balance"`
	PriceUSD      string `json:"price_usd"`
	ValueUSD      string `json:"value_usd"`
	ProtocolID    string `json:"protocol_id,omitempty"`
	AssetType     string `json:"asset_type"`
	ChainID       int    `json:"chain_id"`
	QueriedAt     string `json:"queried_at"`
}

// UserAssetsResponse 用户资产响应
type UserAssetsResponse struct {
	Address       string         `json:"address"`
	ChainID       int            `json:"chain_id"`
	TotalValueUSD string         `json:"total_value_usd"`
	Assets        []AssetBalance `json:"assets"`
	QueriedAt     string         `json:"queried_at"`
}

// ProtocolAssetsResponse 协议资产响应
type ProtocolAssetsResponse struct {
	ProtocolID    string         `json:"protocol_id"`
	ProtocolName  string         `json:"protocol_name"`
	TotalValueUSD string         `json:"total_value_usd"`
	Assets        []AssetBalance `json:"assets"`
	QueriedAt     string         `json:"queried_at"`
}

// ServiceAClient 服务A客户端
type ServiceAClient struct {
	httpClient      *HTTPClient
	config          ServiceAConfig
	rateLimiter     rate_limit.RateLimiter
	circuitBreaker  circuit_breaker.CircuitBreaker
	retryManager    *retry.RetryManager
}

// NewServiceAClient 创建服务A客户端
func NewServiceAClient(config ServiceAConfig) (*ServiceAClient, error) {
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
	
	return &ServiceAClient{
		httpClient:     httpClient,
		config:         config,
		rateLimiter:    rateLimiter,
		circuitBreaker: circuitBreaker,
		retryManager:   retryManager,
	}, nil
}

// GetUserAssets 获取用户资产（实时查询）
func (c *ServiceAClient) GetUserAssets(ctx context.Context, address string, chainID int) (*UserAssetsResponse, error) {
	log.Info().
		Str("address", address).
		Int("chain_id", chainID).
		Msg("fetching user assets from service A")
	
	// 构建请求参数
	queryParams := map[string]string{
		"address":  address,
		"chain_id": fmt.Sprintf("%d", chainID),
	}
	
	headers := c.getHeaders()
	
	// 执行受保护的请求
	var response *UserAssetsResponse
	err := c.executeProtectedRequest(ctx, func() error {
		// 等待限流器
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return &retry.NonRetryableError{Err: fmt.Errorf("rate limit wait failed: %w", err)}
		}
		
		// 执行HTTP请求
		resp, err := c.httpClient.Get("/assets", queryParams, headers)
		if err != nil {
			return &retry.RetryableError{Err: fmt.Errorf("HTTP request failed: %w", err)}
		}
		
		// 解析响应
		result, err := ParseJSONResponse[UserAssetsResponse](resp)
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
			Msg("failed to get user assets from service A")
		return nil, err
	}
	
	log.Info().
		Str("address", address).
		Int("chain_id", chainID).
		Str("total_value", response.TotalValueUSD).
		Int("asset_count", len(response.Assets)).
		Msg("successfully fetched user assets from service A")
	
	return response, nil
}

// GetUserAssetsByProtocol 获取用户在特定协议的资产
func (c *ServiceAClient) GetUserAssetsByProtocol(ctx context.Context, address string, chainID int, protocolID string) (*ProtocolAssetsResponse, error) {
	log.Info().
		Str("address", address).
		Int("chain_id", chainID).
		Str("protocol_id", protocolID).
		Msg("fetching user protocol assets from service A")
	
	// 构建请求参数
	queryParams := map[string]string{
		"address":     address,
		"chain_id":    fmt.Sprintf("%d", chainID),
		"protocol_id": protocolID,
	}
	
	headers := c.getHeaders()
	
	// 执行受保护的请求
	var response *ProtocolAssetsResponse
	err := c.executeProtectedRequest(ctx, func() error {
		// 等待限流器
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return &retry.NonRetryableError{Err: fmt.Errorf("rate limit wait failed: %w", err)}
		}
		
		// 执行HTTP请求
		resp, err := c.httpClient.Get("/assets/protocol", queryParams, headers)
		if err != nil {
			return &retry.RetryableError{Err: fmt.Errorf("HTTP request failed: %w", err)}
		}
		
		// 解析响应
		result, err := ParseJSONResponse[ProtocolAssetsResponse](resp)
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
			Msg("failed to get user protocol assets from service A")
		return nil, err
	}
	
	log.Info().
		Str("address", address).
		Int("chain_id", chainID).
		Str("protocol_id", protocolID).
		Str("total_value", response.TotalValueUSD).
		Int("asset_count", len(response.Assets)).
		Msg("successfully fetched user protocol assets from service A")
	
	return response, nil
}

// BatchGetUserAssets 批量获取用户资产
func (c *ServiceAClient) BatchGetUserAssets(ctx context.Context, addresses []string, chainID int) (map[string]*UserAssetsResponse, error) {
	log.Info().
		Int("address_count", len(addresses)).
		Int("chain_id", chainID).
		Msg("batch fetching user assets from service A")
	
	// 构建请求体
	requestBody := map[string]interface{}{
		"addresses": addresses,
		"chain_id":  chainID,
	}
	
	headers := c.getHeaders()
	
	// 执行受保护的请求
	var batchResponse struct {
		Results []*UserAssetsResponse `json:"results"`
	}
	
	err := c.executeProtectedRequest(ctx, func() error {
		// 等待限流器
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return &retry.NonRetryableError{Err: fmt.Errorf("rate limit wait failed: %w", err)}
		}
		
		// 执行HTTP请求
		resp, err := c.httpClient.Post("/assets/batch", requestBody, headers)
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
			Msg("failed to batch get user assets from service A")
		return nil, err
	}
	
	// 构建地址到响应的映射
	result := make(map[string]*UserAssetsResponse)
	for _, response := range batchResponse.Results {
		result[response.Address] = response
	}
	
	log.Info().
		Int("address_count", len(addresses)).
		Int("chain_id", chainID).
		Int("success_count", len(result)).
		Msg("successfully batch fetched user assets from service A")
	
	return result, nil
}

// GetTokenBalances 获取代币余额
func (c *ServiceAClient) GetTokenBalances(ctx context.Context, address string, chainID int, tokenAddresses []string) ([]AssetBalance, error) {
	log.Info().
		Str("address", address).
		Int("chain_id", chainID).
		Int("token_count", len(tokenAddresses)).
		Msg("fetching token balances from service A")
	
	// 构建请求体
	requestBody := map[string]interface{}{
		"address":         address,
		"chain_id":        chainID,
		"token_addresses": tokenAddresses,
	}
	
	headers := c.getHeaders()
	
	// 执行受保护的请求
	var response struct {
		Balances []AssetBalance `json:"balances"`
	}
	
	err := c.executeProtectedRequest(ctx, func() error {
		// 等待限流器
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return &retry.NonRetryableError{Err: fmt.Errorf("rate limit wait failed: %w", err)}
		}
		
		// 执行HTTP请求
		resp, err := c.httpClient.Post("/balances/tokens", requestBody, headers)
		if err != nil {
			return &retry.RetryableError{Err: fmt.Errorf("HTTP request failed: %w", err)}
		}
		
		// 解析响应
		if err := json.Unmarshal(resp.Body, &response); err != nil {
			return &retry.NonRetryableError{Err: fmt.Errorf("failed to parse token balances response: %w", err)}
		}
		
		return nil
	})
	
	if err != nil {
		log.Error().
			Err(err).
			Str("address", address).
			Int("chain_id", chainID).
			Int("token_count", len(tokenAddresses)).
			Msg("failed to get token balances from service A")
		return nil, err
	}
	
	log.Info().
		Str("address", address).
		Int("chain_id", chainID).
		Int("token_count", len(tokenAddresses)).
		Int("balance_count", len(response.Balances)).
		Msg("successfully fetched token balances from service A")
	
	return response.Balances, nil
}

// HealthCheck 健康检查
func (c *ServiceAClient) HealthCheck(ctx context.Context) error {
	log.Debug().Msg("checking service A health")
	
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
		
		log.Debug().Msg("service A health check passed")
		return nil
	})
}

// GetStats 获取客户端统计信息
func (c *ServiceAClient) GetStats() map[string]interface{} {
	rateLimitStats := c.rateLimiter.GetStats()
	circuitBreakerStats := c.circuitBreaker.GetStats()
	retryStats := c.retryManager.GetStats()
	
	return map[string]interface{}{
		"rate_limiter": map[string]interface{}{
			"allowed_requests":  rateLimitStats.AllowedRequests,
			"rejected_requests": rateLimitStats.RejectedRequests,
			"average_wait_time": rateLimitStats.AverageWaitTime.String(),
		},
		"circuit_breaker": map[string]interface{}{
			"state":           circuitBreakerStats.CurrentState.String(),
			"total_requests":  circuitBreakerStats.TotalRequests,
			"failed_requests": circuitBreakerStats.FailedRequests,
			"failure_rate":    fmt.Sprintf("%.2f%%", circuitBreakerStats.FailureRate),
		},
		"retry": map[string]interface{}{
			"attempts":     retryStats.Attempts,
			"total_delay":  retryStats.TotalDelay.String(),
			"last_error":   retryStats.LastError,
		},
	}
}

// executeProtectedRequest 执行受保护的请求（熔断器 + 重试）
func (c *ServiceAClient) executeProtectedRequest(ctx context.Context, fn func() error) error {
	// 使用熔断器执行操作
	err := c.circuitBreaker.Execute(ctx, func() error {
		// 使用重试管理器执行操作
		return c.retryManager.Execute(ctx, func(ctx context.Context) error {
			return fn()
		})
	})
	
	return err
}

// getHeaders 获取请求头
func (c *ServiceAClient) getHeaders() map[string]string {
	headers := map[string]string{
		"Accept": "application/json",
	}
	
	if c.config.APIKey != "" {
		headers["Authorization"] = fmt.Sprintf("Bearer %s", c.config.APIKey)
		headers["X-API-Key"] = c.config.APIKey
	}
	
	return headers
}

// Close 关闭客户端
func (c *ServiceAClient) Close() {
	c.httpClient.Close()
}

// http包导入
import (
	"net/http"
)