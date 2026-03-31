package adapter

import (
	"context"
	"time"

	"defi-asset-service/exchange-rate-system/internal/models"
)

// ProtocolAdapter 协议适配器接口
type ProtocolAdapter interface {
	// Supports 检查是否支持该协议
	Supports(protocolID string) bool
	
	// GetProtocolInfo 获取协议信息
	GetProtocolInfo(ctx context.Context, protocolID string) (*models.ProtocolRateInfo, error)
	
	// CalculateRate 计算汇率
	CalculateRate(ctx context.Context, request models.RateCalculationRequest) (*models.RateCalculationResponse, error)
	
	// GetHistoricalRates 获取历史汇率
	GetHistoricalRates(ctx context.Context, query models.HistoricalRateQuery) ([]models.ExchangeRate, error)
	
	// GetSupportedTokens 获取支持的代币列表
	GetSupportedTokens(ctx context.Context, protocolID string) ([]models.SupportedToken, error)
	
	// GetRateSources 获取汇率数据源
	GetRateSources(ctx context.Context, protocolID string) ([]models.RateSource, error)
	
	// HealthCheck 健康检查
	HealthCheck(ctx context.Context) error
}

// BaseAdapter 基础适配器
type BaseAdapter struct {
	ProtocolID   string
	ProtocolName string
	ProtocolType models.ProtocolCategory
	Chain        string
	LastUpdate   time.Time
}

// LiquidStakingAdapter 流动性质押协议适配器
type LiquidStakingAdapter struct {
	BaseAdapter
	StakingPoolAddress string
	ReceiptToken       string
	UnderlyingToken    string
}

func (a *LiquidStakingAdapter) Supports(protocolID string) bool {
	return a.ProtocolID == protocolID
}

func (a *LiquidStakingAdapter) CalculateRate(ctx context.Context, request models.RateCalculationRequest) (*models.RateCalculationResponse, error) {
	startTime := time.Now()
	
	// 获取池子信息
	poolInfo, err := a.getStakingPoolInfo(ctx)
	if err != nil {
		return nil, err
	}
	
	// 计算兑换率: totalAssets / totalSupply
	exchangeRate := poolInfo.TotalAssets / poolInfo.TotalSupply
	
	// 考虑奖励和惩罚调整
	adjustedRate := exchangeRate * (1 + poolInfo.RewardRate - poolInfo.PenaltyRate)
	
	// 计算凭证数量
	receiptAmount := request.Amount * adjustedRate
	
	calculationTime := time.Since(startTime)
	
	return &models.RateCalculationResponse{
		Request: request,
		ExchangeRate: adjustedRate,
		ReceiptAmount: receiptAmount,
		CalculationTime: calculationTime,
		Confidence: 0.95, // 高置信度
		Sources: []models.RateSource{
			{
				Name: "staking_pool_contract",
				Rate: exchangeRate,
				Weight: 1.0,
				UpdatedAt: time.Now(),
			},
		},
		Metadata: map[string]interface{}{
			"pool_total_assets": poolInfo.TotalAssets,
			"pool_total_supply": poolInfo.TotalSupply,
			"reward_rate": poolInfo.RewardRate,
			"penalty_rate": poolInfo.PenaltyRate,
			"apy": poolInfo.APY,
		},
	}, nil
}

// LendingAdapter 借贷协议适配器
type LendingAdapter struct {
	BaseAdapter
	LendingPoolAddress string
	InterestRateModel  string
}

func (a *LendingAdapter) CalculateRate(ctx context.Context, request models.RateCalculationRequest) (*models.RateCalculationResponse, error) {
	startTime := time.Now()
	
	// 获取exchangeRateStored
	exchangeRate, err := a.getExchangeRateStored(ctx, request.UnderlyingToken)
	if err != nil {
		return nil, err
	}
	
	// 获取借贷利率
	supplyRate := a.getSupplyRate(ctx, request.UnderlyingToken)
	
	// 考虑时间因素（基于区块高度的复利）
	timeFactor := a.calculateTimeFactor(ctx)
	
	// 计算最终汇率
	finalRate := exchangeRate * (1 + supplyRate*timeFactor)
	
	// 计算凭证数量
	receiptAmount := request.Amount * finalRate
	
	calculationTime := time.Since(startTime)
	
	return &models.RateCalculationResponse{
		Request: request,
		ExchangeRate: finalRate,
		ReceiptAmount: receiptAmount,
		CalculationTime: calculationTime,
		Confidence: 0.90,
		Sources: []models.RateSource{
			{
				Name: "lending_pool_contract",
				Rate: exchangeRate,
				Weight: 1.0,
				UpdatedAt: time.Now(),
			},
		},
		Metadata: map[string]interface{}{
			"supply_rate": supplyRate,
			"time_factor": timeFactor,
			"borrow_rate": a.getBorrowRate(ctx, request.UnderlyingToken),
			"utilization_rate": a.getUtilizationRate(ctx, request.UnderlyingToken),
		},
	}, nil
}

// AMMAdapter AMM协议适配器
type AMMAdapter struct {
	BaseAdapter
	PoolAddress string
	PoolType    string // uniswap_v2, uniswap_v3, curve, balancer
}

func (a *AMMAdapter) CalculateRate(ctx context.Context, request models.RateCalculationRequest) (*models.RateCalculationResponse, error) {
	startTime := time.Now()
	
	// 解析输入金额
	amounts, err := a.parseAmounts(request.Parameters)
	if err != nil {
		return nil, err
	}
	
	// 获取池子储备
	reserves, err := a.getPoolReserves(ctx)
	if err != nil {
		return nil, err
	}
	
	// 根据AMM类型计算LP代币数量
	var lpAmount float64
	switch a.PoolType {
	case "uniswap_v2":
		lpAmount = a.calculateUniswapV2LP(amounts, reserves)
	case "uniswap_v3":
		lpAmount = a.calculateUniswapV3LP(amounts, reserves, request.Parameters)
	case "curve":
		lpAmount = a.calculateCurveLP(amounts, reserves)
	case "balancer":
		lpAmount = a.calculateBalancerLP(amounts, reserves)
	default:
		lpAmount = a.calculateGenericLP(amounts, reserves)
	}
	
	// 计算总价值和汇率
	totalValue := a.calculateTotalValue(amounts)
	exchangeRate := 0.0
	if totalValue > 0 {
		exchangeRate = lpAmount / totalValue
	}
	
	calculationTime := time.Since(startTime)
	
	return &models.RateCalculationResponse{
		Request: request,
		ExchangeRate: exchangeRate,
		ReceiptAmount: lpAmount,
		CalculationTime: calculationTime,
		Confidence: 0.85,
		Sources: []models.RateSource{
			{
				Name: "amm_pool_contract",
				Rate: exchangeRate,
				Weight: 1.0,
				UpdatedAt: time.Now(),
			},
		},
		Metadata: map[string]interface{}{
			"pool_type": a.PoolType,
			"reserves": reserves,
			"total_value": totalValue,
			"virtual_price": a.getVirtualPrice(ctx),
		},
	}, nil
}

// YieldAggregatorAdapter 收益聚合器适配器
type YieldAggregatorAdapter struct {
	BaseAdapter
	VaultAddress string
	Strategy     string
}

func (a *YieldAggregatorAdapter) CalculateRate(ctx context.Context, request models.RateCalculationRequest) (*models.RateCalculationResponse, error) {
	startTime := time.Now()
	
	// 获取pricePerShare
	pricePerShare, err := a.getPricePerShare(ctx)
	if err != nil {
		return nil, err
	}
	
	// 计算金库份额数量
	shareAmount := request.Amount * pricePerShare
	
	// 考虑费用调整
	feeAdjustedAmount := shareAmount * (1 - a.getManagementFee() - a.getPerformanceFee())
	
	calculationTime := time.Since(startTime)
	
	return &models.RateCalculationResponse{
		Request: request,
		ExchangeRate: pricePerShare,
		ReceiptAmount: feeAdjustedAmount,
		CalculationTime: calculationTime,
		Confidence: 0.88,
		Sources: []models.RateSource{
			{
				Name: "vault_contract",
				Rate: pricePerShare,
				Weight: 1.0,
				UpdatedAt: time.Now(),
			},
		},
		Metadata: map[string]interface{}{
			"management_fee": a.getManagementFee(),
			"performance_fee": a.getPerformanceFee(),
			"strategy_apy": a.getStrategyAPY(ctx),
			"total_assets": a.getTotalAssets(ctx),
		},
	}, nil
}

// LSDRewardsAdapter LSD收益协议适配器
type LSDRewardsAdapter struct {
	BaseAdapter
	StakingLayer   string
	RewardsLayer   string
	LSDToken       string
}

func (a *LSDRewardsAdapter) CalculateRate(ctx context.Context, request models.RateCalculationRequest) (*models.RateCalculationResponse, error) {
	startTime := time.Now()
	
	// 获取底层质押汇率
	stakingRate, err := a.getStakingRate(ctx)
	if err != nil {
		return nil, err
	}
	
	// 获取LSD层奖励率
	rewardRate := a.getRewardRate(ctx)
	
	// 计算复合汇率
	compoundRate := stakingRate * (1 + rewardRate)
	
	// 计算LSD代币数量
	lsdAmount := request.Amount * compoundRate
	
	calculationTime := time.Since(startTime)
	
	return &models.RateCalculationResponse{
		Request: request,
		ExchangeRate: compoundRate,
		ReceiptAmount: lsdAmount,
		CalculationTime: calculationTime,
		Confidence: 0.80,
		Sources: []models.RateSource{
			{
				Name: "staking_layer",
				Rate: stakingRate,
				Weight: 0.6,
				UpdatedAt: time.Now(),
			},
			{
				Name: "rewards_layer",
				Rate: rewardRate,
				Weight: 0.4,
				UpdatedAt: time.Now(),
			},
		},
		Metadata: map[string]interface{}{
			"base_staking_rate": stakingRate,
			"lsd_reward_rate": rewardRate,
			"total_staked": a.getTotalStaked(ctx),
			"lsd_premium": a.getLSDPremium(ctx),
		},
	}, nil
}

// GenericAdapter 通用协议适配器
type GenericAdapter struct {
	BaseAdapter
	RateCalculationFunc func(ctx context.Context, request models.RateCalculationRequest) (float64, error)
}

func (a *GenericAdapter) CalculateRate(ctx context.Context, request models.RateCalculationRequest) (*models.RateCalculationResponse, error) {
	startTime := time.Now()
	
	// 使用自定义计算函数
	exchangeRate, err := a.RateCalculationFunc(ctx, request)
	if err != nil {
		return nil, err
	}
	
	// 计算凭证数量
	receiptAmount := request.Amount * exchangeRate
	
	calculationTime := time.Since(startTime)
	
	return &models.RateCalculationResponse{
		Request: request,
		ExchangeRate: exchangeRate,
		ReceiptAmount: receiptAmount,
		CalculationTime: calculationTime,
		Confidence: 0.70,
		Sources: []models.RateSource{
			{
				Name: "generic_calculation",
				Rate: exchangeRate,
				Weight: 1.0,
				UpdatedAt: time.Now(),
			},
		},
		Metadata: map[string]interface{}{
			"calculation_method": "generic",
			"protocol_type": a.ProtocolType,
		},
	}, nil
}

// 辅助类型和函数
type StakingPoolInfo struct {
	TotalAssets  float64
	TotalSupply  float64
	RewardRate   float64
	PenaltyRate  float64
	APY          float64
	UpdatedAt    time.Time
}

// 这些是接口方法，需要在具体实现中定义
func (a *LiquidStakingAdapter) getStakingPoolInfo(ctx context.Context) (*StakingPoolInfo, error) {
	// 实现从合约或API获取池子信息
	return &StakingPoolInfo{
		TotalAssets: 1000000,
		TotalSupply: 950000,
		RewardRate:  0.05,
		PenaltyRate: 0.01,
		APY:         0.042,
		UpdatedAt:   time.Now(),
	}, nil
}

func (a *LendingAdapter) getExchangeRateStored(ctx context.Context, token string) (float64, error) {
	// 实现从借贷合约获取汇率
	return 1.02, nil
}

func (a *LendingAdapter) getSupplyRate(ctx context.Context, token string) float64 {
	return 0.03
}

func (a *LendingAdapter) calculateTimeFactor(ctx context.Context) float64 {
	return 1.0
}

// 其他辅助方法类似实现...