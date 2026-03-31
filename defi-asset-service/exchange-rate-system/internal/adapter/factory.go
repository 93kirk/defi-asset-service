package adapter

import (
	"fmt"
	"strings"
)

// AdapterFactory 适配器工厂
type AdapterFactory struct {
	rpcURL string
}

// NewAdapterFactory 创建适配器工厂
func NewAdapterFactory(rpcURL string) *AdapterFactory {
	return &AdapterFactory{
		rpcURL: rpcURL,
	}
}

// CreateAdapter 创建协议适配器
func (f *AdapterFactory) CreateAdapter(protocolID string) (ProtocolAdapter, error) {
	protocolID = strings.ToLower(protocolID)
	
	switch {
	case strings.Contains(protocolID, "lido"):
		return NewLidoAdapter(f.rpcURL)
	case strings.Contains(protocolID, "aave") || strings.Contains(protocolID, "aave_v3"):
		// 暂时返回基础适配器，稍后实现
		return f.createBaseAdapter(protocolID, "Aave V3", "lending", "ETH", "aToken")
	case strings.Contains(protocolID, "etherfi") || strings.Contains(protocolID, "eeth"):
		return f.createBaseAdapter(protocolID, "ether.fi", "lsd_rewards", "ETH", "eETH")
	case strings.Contains(protocolID, "compound") || strings.Contains(protocolID, "compound_v3"):
		return f.createBaseAdapter(protocolID, "Compound V3", "lending", "USDC", "cUSDC")
	case strings.Contains(protocolID, "rocketpool") || strings.Contains(protocolID, "reth"):
		return f.createBaseAdapter(protocolID, "Rocket Pool", "liquid_staking", "ETH", "rETH")
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocolID)
	}
}

// CreateAllAdapters 创建所有支持的适配器
func (f *AdapterFactory) CreateAllAdapters() (map[string]ProtocolAdapter, error) {
	protocols := []string{
		"lido",
		"aave_v3",
		"etherfi",
		"compound_v3",
		"rocketpool",
	}
	
	adapters := make(map[string]ProtocolAdapter)
	
	for _, protocolID := range protocols {
		adapter, err := f.CreateAdapter(protocolID)
		if err != nil {
			return nil, fmt.Errorf("failed to create adapter for %s: %v", protocolID, err)
		}
		adapters[protocolID] = adapter
	}
	
	return adapters, nil
}

// createBaseAdapter 创建基础适配器（临时方案）
func (f *AdapterFactory) createBaseAdapter(protocolID, name, protocolType, underlying, receipt string) (ProtocolAdapter, error) {
	// 返回一个实现了最小接口的基础适配器
	return &baseAdapterImpl{
		BaseAdapter: BaseAdapter{
			ProtocolID:   protocolID,
			ProtocolName: name,
			ProtocolType: protocolType,
		},
		underlyingToken: underlying,
		receiptToken:    receipt,
	}, nil
}

// baseAdapterImpl 基础适配器实现（临时）
type baseAdapterImpl struct {
	BaseAdapter
	underlyingToken string
	receiptToken    string
}

func (a *baseAdapterImpl) Supports(protocolID string) bool {
	return strings.Contains(strings.ToLower(protocolID), strings.ToLower(a.ProtocolID))
}

func (a *baseAdapterImpl) GetProtocolInfo(ctx context.Context, protocolID string) (*models.ProtocolRateInfo, error) {
	// 简化实现
	return &models.ProtocolRateInfo{
		ProtocolID:      a.ProtocolID,
		ProtocolName:    a.ProtocolName,
		ProtocolType:    a.ProtocolType,
		UnderlyingToken: a.underlyingToken,
		ReceiptToken:    a.receiptToken,
		IsActive:        true,
	}, nil
}

func (a *baseAdapterImpl) CalculateRate(ctx context.Context, request models.RateCalculationRequest) (*models.RateCalculationResponse, error) {
	// 简化实现，返回模拟汇率
	return &models.RateCalculationResponse{
		ProtocolID:      a.ProtocolID,
		ProtocolName:    a.ProtocolName,
		UnderlyingToken: a.underlyingToken,
		ReceiptToken:    a.receiptToken,
		ExchangeRate:    big.NewFloat(1.02), // 模拟2%收益
		APY:             big.NewFloat(3.5),
		Source:          "mock",
	}, nil
}

func (a *baseAdapterImpl) GetHistoricalRates(ctx context.Context, query models.HistoricalRateQuery) ([]models.ExchangeRate, error) {
	return []models.ExchangeRate{}, nil
}

func (a *baseAdapterImpl) GetSupportedTokens(ctx context.Context, protocolID string) ([]models.SupportedToken, error) {
	return []models.SupportedToken{
		{
			TokenSymbol: a.underlyingToken,
			TokenName:   a.underlyingToken,
			IsActive:    true,
		},
	}, nil
}

func (a *baseAdapterImpl) GetRateSources(ctx context.Context, protocolID string) ([]models.RateSource, error) {
	return []models.RateSource{
		{
			SourceType: "contract",
			Priority:   1,
			IsActive:   true,
		},
	}, nil
}

func (a *baseAdapterImpl) HealthCheck(ctx context.Context) error {
	return nil
}