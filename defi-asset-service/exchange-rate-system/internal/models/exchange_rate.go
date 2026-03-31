package models

import (
	"time"
)

// ExchangeRate 汇率信息
type ExchangeRate struct {
	ID              string    `json:"id" db:"id"`
	ProtocolID      string    `json:"protocol_id" db:"protocol_id"`
	UnderlyingToken string    `json:"underlying_token" db:"underlying_token"`
	ReceiptToken    string    `json:"receipt_token" db:"receipt_token"`
	ExchangeRate    float64   `json:"exchange_rate" db:"exchange_rate"`
	APY             float64   `json:"apy,omitempty" db:"apy"`
	TVL             float64   `json:"tvl,omitempty" db:"tvl"`
	Source          string    `json:"source" db:"source"`
	Confidence      float64   `json:"confidence" db:"confidence"` // 0-1的置信度
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
	ValidUntil      time.Time `json:"valid_until" db:"valid_until"`
}

// RateCalculationRequest 汇率计算请求
type RateCalculationRequest struct {
	ProtocolID      string            `json:"protocol_id"`
	UnderlyingToken string            `json:"underlying_token"`
	Amount          float64           `json:"amount"`
	Timestamp       *time.Time        `json:"timestamp,omitempty"`
	Parameters      map[string]string `json:"parameters,omitempty"`
}

// RateCalculationResponse 汇率计算响应
type RateCalculationResponse struct {
	Request         RateCalculationRequest `json:"request"`
	ExchangeRate    float64                `json:"exchange_rate"`
	ReceiptAmount   float64                `json:"receipt_amount"`
	USDValue        float64                `json:"usd_value,omitempty"`
	CalculationTime time.Duration          `json:"calculation_time_ms"`
	Confidence      float64                `json:"confidence"`
	Sources         []RateSource          `json:"sources"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// RateSource 汇率数据源
type RateSource struct {
	Name      string    `json:"name"`
	URL       string    `json:"url,omitempty"`
	Rate      float64   `json:"rate"`
	Weight    float64   `json:"weight"` // 权重 (0-1)
	UpdatedAt time.Time `json:"updated_at"`
}

// ProtocolRateInfo 协议汇率信息
type ProtocolRateInfo struct {
	ProtocolID      string                  `json:"protocol_id"`
	ProtocolName    string                  `json:"protocol_name"`
	ProtocolType    string                  `json:"protocol_type"`
	Chain           string                  `json:"chain"`
	CurrentRate     ExchangeRate            `json:"current_rate"`
	HistoricalRates []ExchangeRate          `json:"historical_rates,omitempty"`
	RateStatistics  RateStatistics          `json:"statistics,omitempty"`
	SupportedTokens []SupportedToken        `json:"supported_tokens,omitempty"`
	RateProviders   []RateProviderInfo      `json:"rate_providers,omitempty"`
}

// RateStatistics 汇率统计信息
type RateStatistics struct {
	AverageRate     float64   `json:"average_rate"`
	MinRate         float64   `json:"min_rate"`
	MaxRate         float64   `json:"max_rate"`
	Volatility      float64   `json:"volatility"`
	Last24HChange   float64   `json:"last_24h_change"`
	UpdateFrequency time.Time `json:"update_frequency"`
}

// SupportedToken 支持的代币
type SupportedToken struct {
	TokenAddress string  `json:"token_address,omitempty"`
	TokenSymbol  string  `json:"token_symbol"`
	TokenName    string  `json:"token_name"`
	Decimals     int     `json:"decimals"`
	IsUnderlying bool    `json:"is_underlying"`
	IsReceipt    bool    `json:"is_receipt"`
	CurrentRate  float64 `json:"current_rate,omitempty"`
}

// RateProviderInfo 汇率提供者信息
type RateProviderInfo struct {
	Name         string    `json:"name"`
	Type         string    `json:"type"` // onchain, api, custom
	IsActive     bool      `json:"is_active"`
	LastUpdate   time.Time `json:"last_update"`
	SuccessRate  float64   `json:"success_rate"`
	AverageDelay time.Time `json:"average_delay"`
}

// HistoricalRateQuery 历史汇率查询
type HistoricalRateQuery struct {
	ProtocolID      string    `json:"protocol_id"`
	UnderlyingToken string    `json:"underlying_token,omitempty"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time"`
	Interval        string    `json:"interval,omitempty"` // 1m, 5m, 1h, 1d
	Limit           int       `json:"limit,omitempty"`
}

// RateAlert 汇率告警
type RateAlert struct {
	ID          string    `json:"id"`
	ProtocolID  string    `json:"protocol_id"`
	Condition   string    `json:"condition"` // above, below, change
	Threshold   float64   `json:"threshold"`
	IsTriggered bool      `json:"is_triggered"`
	LastChecked time.Time `json:"last_checked"`
	CreatedAt   time.Time `json:"created_at"`
}

// ProtocolCategory 协议分类
type ProtocolCategory string

const (
	CategoryLiquidStaking   ProtocolCategory = "liquid_staking"
	CategoryLending         ProtocolCategory = "lending"
	CategoryAMM            ProtocolCategory = "amm"
	CategoryYieldAggregator ProtocolCategory = "yield_aggregator"
	CategoryLSDRewards     ProtocolCategory = "lsd_rewards"
	CategoryRestaking      ProtocolCategory = "restaking"
	CategoryOthers         ProtocolCategory = "others"
)

// GetCategoryFromProtocolID 根据协议ID获取分类
func GetCategoryFromProtocolID(protocolID string) ProtocolCategory {
	// 根据协议ID的模式识别分类
	id := protocolID
	
	switch {
	case containsAny(id, "lido", "rocket", "stake", "steth", "reth"):
		return CategoryLiquidStaking
	case containsAny(id, "aave", "compound", "morpho", "spark", "venus"):
		return CategoryLending
	case containsAny(id, "uniswap", "curve", "balancer", "pancake", "sushiswap"):
		return CategoryAMM
	case containsAny(id, "yearn", "convex", "aura", "pendle"):
		return CategoryYieldAggregator
	case containsAny(id, "etherfi", "kelp", "swell", "renzo"):
		return CategoryLSDRewards
	case containsAny(id, "eigenlayer", "restake"):
		return CategoryRestaking
	default:
		return CategoryOthers
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || 
		contains(s[1:], substr))))
}

// RateCalculationMethod 汇率计算方法
type RateCalculationMethod string

const (
	MethodStakingPoolRatio   RateCalculationMethod = "staking_pool_ratio"
	MethodExchangeRateStored RateCalculationMethod = "exchange_rate_stored"
	MethodReserveRatio       RateCalculationMethod = "reserve_ratio"
	MethodPricePerShare      RateCalculationMethod = "price_per_share"
	MethodCompoundYield      RateCalculationMethod = "compound_yield"
	MethodProtocolSpecific   RateCalculationMethod = "protocol_specific"
)

// GetCalculationMethod 获取协议的计算方法
func GetCalculationMethod(category ProtocolCategory) RateCalculationMethod {
	switch category {
	case CategoryLiquidStaking:
		return MethodStakingPoolRatio
	case CategoryLending:
		return MethodExchangeRateStored
	case CategoryAMM:
		return MethodReserveRatio
	case CategoryYieldAggregator:
		return MethodPricePerShare
	case CategoryLSDRewards, CategoryRestaking:
		return MethodCompoundYield
	default:
		return MethodProtocolSpecific
	}
}