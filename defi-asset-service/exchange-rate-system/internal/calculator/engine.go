package calculator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"defi-asset-service/exchange-rate-system/internal/adapter"
	"defi-asset-service/exchange-rate-system/internal/cache"
	"defi-asset-service/exchange-rate-system/internal/models"
	"defi-asset-service/exchange-rate-system/internal/provider"
)

// ExchangeRateEngine 汇率计算引擎
type ExchangeRateEngine struct {
	adapterRegistry *adapter.AdapterRegistry
	rateProviders   []provider.ExchangeRateProvider
	cache           cache.RateCache
	mu              sync.RWMutex
	metrics         *Metrics
	config          Config
}

// Config 引擎配置
type Config struct {
	CacheTTL           time.Duration
	MaxRetries         int
	RetryDelay         time.Duration
	Timeout            time.Duration
	EnableCache        bool
	EnableValidation   bool
	ValidationSources  int
	FallbackEnabled    bool
	FallbackThreshold  float64
	RateLimitPerSecond int
}

// DefaultConfig 默认配置
var DefaultConfig = Config{
	CacheTTL:           5 * time.Minute,
	MaxRetries:         3,
	RetryDelay:         1 * time.Second,
	Timeout:            10 * time.Second,
	EnableCache:        true,
	EnableValidation:   true,
	ValidationSources:  2,
	FallbackEnabled:    true,
	FallbackThreshold:  0.8,
	RateLimitPerSecond: 10,
}

// NewExchangeRateEngine 创建新的汇率计算引擎
func NewExchangeRateEngine(
	adapterRegistry *adapter.AdapterRegistry,
	rateProviders []provider.ExchangeRateProvider,
	cache cache.RateCache,
	config Config,
) *ExchangeRateEngine {
	if cache == nil {
		cache = cache.NewMemoryCache(1000, 5*time.Minute)
	}
	
	return &ExchangeRateEngine{
		adapterRegistry: adapterRegistry,
		rateProviders:   rateProviders,
		cache:           cache,
		metrics:         NewMetrics(),
		config:          config,
	}
}

// CalculateRate 计算汇率
func (e *ExchangeRateEngine) CalculateRate(ctx context.Context, request models.RateCalculationRequest) (*models.RateCalculationResponse, error) {
	startTime := time.Now()
	
	// 检查缓存
	if e.config.EnableCache {
		if cachedResponse, found := e.getFromCache(request); found {
			e.metrics.IncrementCacheHit()
			return cachedResponse, nil
		}
		e.metrics.IncrementCacheMiss()
	}
	
	// 获取适配器
	adapter, found := e.adapterRegistry.GetAdapter(request.ProtocolID)
	if !found {
		// 尝试创建通用适配器
		adapter = e.createGenericAdapter(request.ProtocolID)
		if adapter == nil {
			return nil, fmt.Errorf("no adapter found for protocol: %s", request.ProtocolID)
		}
	}
	
	// 执行计算
	var response *models.RateCalculationResponse
	var err error
	
	for i := 0; i < e.config.MaxRetries; i++ {
		ctxWithTimeout, cancel := context.WithTimeout(ctx, e.config.Timeout)
		defer cancel()
		
		response, err = adapter.CalculateRate(ctxWithTimeout, request)
		if err == nil {
			break
		}
		
		if i < e.config.MaxRetries-1 {
			time.Sleep(e.config.RetryDelay)
		}
	}
	
	if err != nil {
		// 尝试使用备用数据源
		if e.config.FallbackEnabled {
			response, err = e.calculateWithFallback(ctx, request)
		}
		if err != nil {
			e.metrics.IncrementError()
			return nil, err
		}
	}
	
	// 验证结果
	if e.config.EnableValidation {
		if err := e.validateResponse(response); err != nil {
			e.metrics.IncrementValidationError()
			// 记录验证错误但返回结果
		}
	}
	
	// 更新缓存
	if e.config.EnableCache {
		e.updateCache(request, response)
	}
	
	// 记录指标
	e.metrics.RecordCalculationTime(time.Since(startTime))
	e.metrics.IncrementRequest()
	
	return response, nil
}

// BatchCalculateRates 批量计算汇率
func (e *ExchangeRateEngine) BatchCalculateRates(ctx context.Context, requests []models.RateCalculationRequest) ([]*models.RateCalculationResponse, error) {
	var wg sync.WaitGroup
	results := make([]*models.RateCalculationResponse, len(requests))
	errors := make([]error, len(requests))
	
	// 限制并发数
	semaphore := make(chan struct{}, e.config.RateLimitPerSecond)
	
	for i, req := range requests {
		wg.Add(1)
		go func(idx int, request models.RateCalculationRequest) {
			defer wg.Done()
			
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			response, err := e.CalculateRate(ctx, request)
			results[idx] = response
			errors[idx] = err
		}(i, req)
	}
	
	wg.Wait()
	
	// 检查是否有错误
	var hasError bool
	for _, err := range errors {
		if err != nil {
			hasError = true
			break
		}
	}
	
	if hasError {
		return results, fmt.Errorf("some calculations failed")
	}
	
	return results, nil
}

// GetHistoricalRates 获取历史汇率
func (e *ExchangeRateEngine) GetHistoricalRates(ctx context.Context, query models.HistoricalRateQuery) ([]models.ExchangeRate, error) {
	// 检查缓存
	cacheKey := fmt.Sprintf("historical:%s:%s:%d", query.ProtocolID, query.Interval, query.StartTime.Unix())
	if cached, found := e.cache.Get(cacheKey); found {
		if rates, ok := cached.([]models.ExchangeRate); ok {
			return rates, nil
		}
	}
	
	// 获取适配器
	adapter, found := e.adapterRegistry.GetAdapter(query.ProtocolID)
	if !found {
		return nil, fmt.Errorf("no adapter found for protocol: %s", query.ProtocolID)
	}
	
	// 查询历史数据
	rates, err := adapter.GetHistoricalRates(ctx, query)
	if err != nil {
		return nil, err
	}
	
	// 更新缓存
	e.cache.Set(cacheKey, rates, 24*time.Hour)
	
	return rates, nil
}

// GetProtocolInfo 获取协议信息
func (e *ExchangeRateEngine) GetProtocolInfo(ctx context.Context, protocolID string) (*models.ProtocolRateInfo, error) {
	// 获取适配器
	adapter, found := e.adapterRegistry.GetAdapter(protocolID)
	if !found {
		return nil, fmt.Errorf("no adapter found for protocol: %s", protocolID)
	}
	
	// 获取协议信息
	info, err := adapter.GetProtocolInfo(ctx, protocolID)
	if err != nil {
		return nil, err
	}
	
	// 从多个数据源获取汇率进行验证
	if e.config.EnableValidation {
		e.enrichWithValidationData(ctx, info)
	}
	
	return info, nil
}

// HealthCheck 健康检查
func (e *ExchangeRateEngine) HealthCheck(ctx context.Context) map[string]interface{} {
	health := make(map[string]interface{})
	
	// 检查适配器
	adapterErrors := e.adapterRegistry.HealthCheck(ctx)
	health["adapters"] = map[string]interface{}{
		"total":     len(e.adapterRegistry.GetAllAdapters()),
		"errors":    len(adapterErrors),
		"error_map": adapterErrors,
	}
	
	// 检查缓存
	cacheHealth := e.cache.HealthCheck()
	health["cache"] = cacheHealth
	
	// 检查数据源
	var providerHealth []map[string]interface{}
	for _, p := range e.rateProviders {
		if err := p.HealthCheck(ctx); err != nil {
			providerHealth = append(providerHealth, map[string]interface{}{
				"name":  p.Name(),
				"error": err.Error(),
			})
		} else {
			providerHealth = append(providerHealth, map[string]interface{}{
				"name":   p.Name(),
				"status": "healthy",
			})
		}
	}
	health["providers"] = providerHealth
	
	// 系统指标
	health["metrics"] = e.metrics.GetMetrics()
	
	return health
}

// 私有方法
func (e *ExchangeRateEngine) getFromCache(request models.RateCalculationRequest) (*models.RateCalculationResponse, bool) {
	cacheKey := e.generateCacheKey(request)
	
	cached, found := e.cache.Get(cacheKey)
	if !found {
		return nil, false
	}
	
	response, ok := cached.(*models.RateCalculationResponse)
	if !ok {
		return nil, false
	}
	
	// 检查缓存是否过期
	if time.Since(response.Request.Timestamp) > e.config.CacheTTL {
		e.cache.Delete(cacheKey)
		return nil, false
	}
	
	return response, true
}

func (e *ExchangeRateEngine) updateCache(request models.RateCalculationRequest, response *models.RateCalculationResponse) {
	cacheKey := e.generateCacheKey(request)
	e.cache.Set(cacheKey, response, e.config.CacheTTL)
}

func (e *ExchangeRateEngine) generateCacheKey(request models.RateCalculationRequest) string {
	timestamp := ""
	if request.Timestamp != nil {
		timestamp = request.Timestamp.Format("20060102150405")
	}
	return fmt.Sprintf("rate:%s:%s:%f:%s", 
		request.ProtocolID, 
		request.UnderlyingToken, 
		request.Amount,
		timestamp)
}

func (e *ExchangeRateEngine) createGenericAdapter(protocolID string) adapter.ProtocolAdapter {
	// 这里可以创建一个基于协议数据的通用适配器
	// 暂时返回nil，实际实现需要从数据库或配置中获取协议信息
	return nil
}

func (e *ExchangeRateEngine) calculateWithFallback(ctx context.Context, request models.RateCalculationRequest) (*models.RateCalculationResponse, error) {
	// 尝试使用备用数据源计算
	for _, provider := range e.rateProviders {
		rate, err := provider.GetRate(ctx, request.ProtocolID, request.UnderlyingToken)
		if err == nil && rate > 0 {
			return &models.RateCalculationResponse{
				Request:       request,
				ExchangeRate:  rate,
				ReceiptAmount: request.Amount * rate,
				Confidence:    0.5, // 较低置信度
				Sources: []models.RateSource{
					{
						Name:   provider.Name(),
						Rate:   rate,
						Weight: 0.5,
					},
				},
			}, nil
		}
	}
	
	return nil, fmt.Errorf("all calculation methods failed")
}

func (e *ExchangeRateEngine) validateResponse(response *models.RateCalculationResponse) error {
	// 检查汇率是否有效
	if response.ExchangeRate <= 0 {
		return fmt.Errorf("invalid exchange rate: %f", response.ExchangeRate)
	}
	
	// 检查置信度
	if response.Confidence < e.config.FallbackThreshold {
		return fmt.Errorf("low confidence: %f", response.Confidence)
	}
	
	// 检查计算时间
	if response.CalculationTime > 10*time.Second {
		return fmt.Errorf("calculation took too long: %v", response.CalculationTime)
	}
	
	return nil
}

func (e *ExchangeRateEngine) enrichWithValidationData(ctx context.Context, info *models.ProtocolRateInfo) {
	// 从多个数据源获取汇率进行对比
	var validationRates []float64
	
	for _, provider := range e.rateProviders {
		rate, err := provider.GetRate(ctx, info.ProtocolID, info.CurrentRate.UnderlyingToken)
		if err == nil && rate > 0 {
			validationRates = append(validationRates, rate)
		}
	}
	
	if len(validationRates) > 0 {
		// 计算平均验证汇率
		var sum float64
		for _, rate := range validationRates {
			sum += rate
		}
		avgValidationRate := sum / float64(len(validationRates))
		
		// 计算偏差
		deviation := (info.CurrentRate.ExchangeRate - avgValidationRate) / avgValidationRate
		
		// 如果偏差太大，调整置信度
		if abs(deviation) > 0.1 { // 10%偏差
			info.CurrentRate.Confidence *= 0.8
		}
		
		// 添加验证信息到metadata
		if info.CurrentRate.Metadata == nil {
			info.CurrentRate.Metadata = make(map[string]interface{})
		}
		info.CurrentRate.Metadata["validation_rates"] = validationRates
		info.CurrentRate.Metadata["validation_deviation"] = deviation
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// Metrics 指标收集
type Metrics struct {
	requestsTotal        int64
	cacheHits           int64
	cacheMisses         int64
	errorsTotal         int64
	validationErrors    int64
	calculationTimeSum  time.Duration
	calculationCount    int64
	mu                  sync.RWMutex
}

func NewMetrics() *Metrics {
	return &Metrics{}
}

func (m *Metrics) IncrementRequest() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestsTotal++
}

func (m *Metrics) IncrementCacheHit() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheHits++
}

func (m *Metrics) IncrementCacheMiss() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheMisses++
}

func (m *Metrics) IncrementError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorsTotal++
}

func (m *Metrics) IncrementValidationError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validationErrors++
}

func (m *Metrics) RecordCalculationTime(duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calculationTimeSum += duration
	m.calculationCount++
}

func (m *Metrics) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	avgCalculationTime := time.Duration(0)
	if m.calculationCount > 0 {
		avgCalculationTime = m.calculationTimeSum / time.Duration(m.calculationCount)
	}
	
	cacheHitRate := 0.0
	totalCacheAccess := m.cacheHits + m.cacheMisses
	if totalCacheAccess > 0 {
		cacheHitRate = float64(m.cacheHits) / float64(totalCacheAccess)
	}
	
	errorRate := 0.0
	if m.requestsTotal > 0 {
		errorRate = float64(m.errorsTotal) / float64(m.requestsTotal)
	}
	
	return map[string]interface{}{
		"requests_total":        m.requestsTotal,
		"cache_hits":           m.cacheHits,
		"cache_misses":         m.cacheMisses,
		"cache_hit_rate":       cacheHitRate,
		"errors_total":         m.errorsTotal,
		"validation_errors":    m.validationErrors,
		"error_rate":           errorRate,
		"avg_calculation_time": avgCalculationTime.String(),
		"calculation_count":    m.calculationCount,
	}
}