package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"defi-asset-service/exchange-rate-system/internal/models"
)

// ExchangeRateProvider 汇率提供者接口
type ExchangeRateProvider interface {
	// GetRate 获取汇率
	GetRate(ctx context.Context, protocolID, underlyingToken string) (float64, error)
	
	// GetHistoricalRates 获取历史汇率
	GetHistoricalRates(ctx context.Context, protocolID, underlyingToken string, start, end time.Time) ([]models.ExchangeRate, error)
	
	// GetProtocolInfo 获取协议信息
	GetProtocolInfo(ctx context.Context, protocolID string) (*models.ProtocolRateInfo, error)
	
	// Name 提供者名称
	Name() string
	
	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error
}

// ChainlinkProvider Chainlink预言机提供者
type ChainlinkProvider struct {
	baseURL string
	client  *http.Client
}

// NewChainlinkProvider 创建新的Chainlink提供者
func NewChainlinkProvider(baseURL string) *ChainlinkProvider {
	return &ChainlinkProvider{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (p *ChainlinkProvider) GetRate(ctx context.Context, protocolID, underlyingToken string) (float64, error) {
	// 这里应该调用Chainlink API获取汇率
	// 暂时返回示例数据
	
	// 根据协议和代币返回不同的汇率
	rates := map[string]map[string]float64{
		"eth2": {
			"ETH": 1.02,
		},
		"lido": {
			"ETH": 1.02,
		},
		"aave3": {
			"ETH": 1.03,
			"USDC": 1.05,
			"DAI": 1.04,
		},
		"uniswap_v3": {
			"ETH": 1.01,
			"USDC": 1.01,
		},
	}
	
	if protocolRates, ok := rates[protocolID]; ok {
		if rate, ok := protocolRates[underlyingToken]; ok {
			return rate, nil
		}
	}
	
	// 默认返回1.0
	return 1.0, nil
}

func (p *ChainlinkProvider) Name() string {
	return "chainlink"
}

// ProtocolAPIProvider 协议原生API提供者
type ProtocolAPIProvider struct {
	client *http.Client
}

// NewProtocolAPIProvider 创建新的协议API提供者
func NewProtocolAPIProvider() *ProtocolAPIProvider {
	return &ProtocolAPIProvider{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (p *ProtocolAPIProvider) GetRate(ctx context.Context, protocolID, underlyingToken string) (float64, error) {
	// 这里应该调用各个协议的原生API
	// 暂时根据协议类型返回示例数据
	
	category := models.GetCategoryFromProtocolID(protocolID)
	
	switch category {
	case models.CategoryLiquidStaking:
		return 1.02, nil
	case models.CategoryLending:
		return 1.03, nil
	case models.CategoryAMM:
		return 1.01, nil
	case models.CategoryYieldAggregator:
		return 1.08, nil
	case models.CategoryLSDRewards:
		return 1.05, nil
	default:
		return 1.0, nil
	}
}

func (p *ProtocolAPIProvider) Name() string {
	return "protocol_api"
}

// CustomCalculatorProvider 自定义计算提供者
type CustomCalculatorProvider struct {
	calculators map[string]func(ctx context.Context, protocolID, token string) (float64, error)
}

// NewCustomCalculatorProvider 创建新的自定义计算提供者
func NewCustomCalculatorProvider() *CustomCalculatorProvider {
	return &CustomCalculatorProvider{
		calculators: make(map[string]func(ctx context.Context, protocolID, token string) (float64, error)),
	}
}

func (p *CustomCalculatorProvider) GetRate(ctx context.Context, protocolID, underlyingToken string) (float64, error) {
	// 检查是否有自定义计算器
	if calculator, ok := p.calculators[protocolID]; ok {
		return calculator(ctx, protocolID, underlyingToken)
	}
	
	// 使用默认计算逻辑
	return p.calculateDefaultRate(protocolID, underlyingToken)
}

func (p *CustomCalculatorProvider) calculateDefaultRate(protocolID, underlyingToken string) (float64, error) {
	// 基于协议ID的简单计算逻辑
	// 在实际应用中，这里应该实现更复杂的计算
	
	// 示例：根据协议ID的长度和代币名称计算一个伪随机汇率
	hash := 0
	for _, c := range protocolID {
		hash += int(c)
	}
	for _, c := range underlyingToken {
		hash += int(c)
	}
	
	// 生成1.0到1.1之间的汇率
	rate := 1.0 + float64(hash%100)/1000.0
	return rate, nil
}

func (p *CustomCalculatorProvider) Name() string {
	return "custom_calculator"
}

// DebankProvider DeBank数据提供者
type DebankProvider struct {
	baseURL string
	client  *http.Client
	apiKey  string
}

// NewDebankProvider 创建新的DeBank提供者
func NewDebankProvider(baseURL, apiKey string) *DebankProvider {
	return &DebankProvider{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		apiKey: apiKey,
	}
}

func (p *DebankProvider) GetRate(ctx context.Context, protocolID, underlyingToken string) (float64, error) {
	// 这里应该调用DeBank API获取协议数据
	// 暂时返回示例数据
	
	// 模拟从DeBank获取数据
	url := fmt.Sprintf("%s/protocol/%s", p.baseURL, protocolID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, err
	}
	
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	
	resp, err := p.client.Do(req)
	if err != nil {
		// 模拟返回数据
		return p.getMockRate(protocolID), nil
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return p.getMockRate(protocolID), nil
	}
	
	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return p.getMockRate(protocolID), nil
	}
	
	// 从DeBank响应中提取汇率
	if poolStats, ok := data["pool_stats"].([]interface{}); ok && len(poolStats) > 0 {
		if firstStat, ok := poolStats[0].(map[string]interface{}); ok {
			if rate, ok := firstStat["rate"].(float64); ok && rate > 0 {
				return rate, nil
			}
		}
	}
	
	return p.getMockRate(protocolID), nil
}

func (p *DebankProvider) getMockRate(protocolID string) float64 {
	// 根据协议ID返回模拟汇率
	rates := map[string]float64{
		"eth2":           1.02,
		"aave3":          1.03,
		"lido":           1.02,
		"uniswap_v3":     1.01,
		"yearn":          1.08,
		"etherfi":        1.05,
		"bsc_bnbchain":   1.01,
		"morphoblue":     1.04,
		"spark":          1.03,
		"rocketpool":     1.02,
	}
	
	if rate, ok := rates[protocolID]; ok {
		return rate
	}
	
	return 1.0
}

func (p *DebankProvider) Name() string {
	return "debank"
}

// 所有提供者的通用健康检查实现
func (p *ChainlinkProvider) HealthCheck(ctx context.Context) error {
	// 测试API连接
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("chainlink health check failed with status: %d", resp.StatusCode)
	}
	
	return nil
}

func (p *ProtocolAPIProvider) HealthCheck(ctx context.Context) error {
	// 协议API提供者总是健康的（因为它是多个API的聚合）
	return nil
}

func (p *CustomCalculatorProvider) HealthCheck(ctx context.Context) error {
	// 自定义计算提供者总是健康的
	return nil
}

func (p *DebankProvider) HealthCheck(ctx context.Context) error {
	// 测试DeBank API连接
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}
	
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("debank health check failed with status: %d", resp.StatusCode)
	}
	
	return nil
}

// 所有提供者的通用历史汇率实现（简化版）
func (p *ChainlinkProvider) GetHistoricalRates(ctx context.Context, protocolID, underlyingToken string, start, end time.Time) ([]models.ExchangeRate, error) {
	// 这里应该调用Chainlink历史数据API
	// 暂时返回示例数据
	return generateMockHistoricalRates(protocolID, underlyingToken, start, end), nil
}

func (p *ProtocolAPIProvider) GetHistoricalRates(ctx context.Context, protocolID, underlyingToken string, start, end time.Time) ([]models.ExchangeRate, error) {
	return generateMockHistoricalRates(protocolID, underlyingToken, start, end), nil
}

func (p *CustomCalculatorProvider) GetHistoricalRates(ctx context.Context, protocolID, underlyingToken string, start, end time.Time) ([]models.ExchangeRate, error) {
	return generateMockHistoricalRates(protocolID, underlyingToken, start, end), nil
}

func (p *DebankProvider) GetHistoricalRates(ctx context.Context, protocolID, underlyingToken string, start, end time.Time) ([]models.ExchangeRate, error) {
	return generateMockHistoricalRates(protocolID, underlyingToken, start, end), nil
}

// 生成模拟历史汇率数据
func generateMockHistoricalRates(protocolID, underlyingToken string, start, end time.Time) []models.ExchangeRate {
	var rates []models.ExchangeRate
	
	// 基础汇率
	baseRate := 1.0
	switch protocolID {
	case "eth2", "lido":
		baseRate = 1.02
	case "aave3":
		baseRate = 1.03
	case "uniswap_v3":
		baseRate = 1.01
	}
	
	// 生成每天的数据点
	for t := start; t.Before(end); t = t.Add(24 * time.Hour) {
		// 添加一些随机波动
		variation := 0.95 + 0.1*float64(t.Day()%10)/10.0
		rate := baseRate * variation
		
		rates = append(rates, models.ExchangeRate{
			ProtocolID:      protocolID,
			UnderlyingToken: underlyingToken,
			ReceiptToken:    "receipt_token",
			ExchangeRate:    rate,
			UpdatedAt:       t,
			ValidUntil:      t.Add(24 * time.Hour),
		})
	}
	
	return rates
}

// 所有提供者的通用协议信息实现（简化版）
func (p *ChainlinkProvider) GetProtocolInfo(ctx context.Context, protocolID string) (*models.ProtocolRateInfo, error) {
	return generateMockProtocolInfo(protocolID), nil
}

func (p *ProtocolAPIProvider) GetProtocolInfo(ctx context.Context, protocolID string) (*models.ProtocolRateInfo, error) {
	return generateMockProtocolInfo(protocolID), nil
}

func (p *CustomCalculatorProvider) GetProtocolInfo(ctx context.Context, protocolID string) (*models.ProtocolRateInfo, error) {
	return generateMockProtocolInfo(protocolID), nil
}

func (p *DebankProvider) GetProtocolInfo(ctx context.Context, protocolID string) (*models.ProtocolRateInfo, error) {
	return generateMockProtocolInfo(protocolID), nil
}

// 生成模拟协议信息
func generateMockProtocolInfo(protocolID string) *models.ProtocolRateInfo {
	category := models.GetCategoryFromProtocolID(protocolID)
	
	baseRate := 1.0
	switch category {
	case models.CategoryLiquidStaking:
		baseRate = 1.02
	case models.CategoryLending:
		baseRate = 1.03
	case models.CategoryAMM:
		baseRate = 1.01
	case models.CategoryYieldAggregator:
		baseRate = 1.08
	case models.CategoryLSDRewards:
		baseRate = 1.05
	}
	
	return &models.ProtocolRateInfo{
		ProtocolID:   protocolID,
		ProtocolName: protocolID,
		ProtocolType: string(category),
		Chain:        "eth",
		CurrentRate: models.ExchangeRate{
			ProtocolID:      protocolID,
			UnderlyingToken: "underlying_token",
			ReceiptToken:    "receipt_token",
			ExchangeRate:    baseRate,
			UpdatedAt:       time.Now(),
			ValidUntil:      time.Now().Add(1 * time.Hour),
		},
	}
}