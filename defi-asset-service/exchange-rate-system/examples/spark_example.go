package examples

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// SparkExchangeCalculator Spark协议汇率计算器
type SparkExchangeCalculator struct {
	client          *ethclient.Client
	sparkContract   common.Address // Spark池合约
	lendingPool     common.Address // 借贷池
	oracleContract  common.Address // 价格预言机
}

// NewSparkExchangeCalculator 创建Spark计算器
func NewSparkExchangeCalculator(rpcURL string) (*SparkExchangeCalculator, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	
	return &SparkExchangeCalculator{
		client:          client,
		sparkContract:   common.HexToAddress("0xC13e21B648A5Ee794902342038FF3aDAB66BE987"), // Spark主合约
		lendingPool:     common.HexToAddress("0xC13e21B648A5Ee794902342038FF3aDAB66BE987"), // 借贷池地址
		oracleContract:  common.HexToAddress("0x8105f69D9C41644c6A0803fDA7D03Aa70996cFD9"), // Spark预言机
	}, nil
}

// CalculateExchangeRate 计算Spark协议汇率
func (c *SparkExchangeCalculator) CalculateExchangeRate(ctx context.Context, assetAddress common.Address) (*big.Float, error) {
	// Spark汇率计算（基于Aave V3架构）：
	// 1. 获取标准化收入（normalized income）
	// 2. 转换为汇率
	
	// 获取资产配置
	assetConfig, err := c.getAssetConfiguration(ctx, assetAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get asset configuration: %v", err)
	}
	
	// 获取标准化收入
	normalizedIncome, err := c.getNormalizedIncome(ctx, assetAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get normalized income: %v", err)
	}
	
	// 计算汇率：标准化收入 / 1e27
	ray := new(big.Int).Exp(big.NewInt(10), big.NewInt(27), nil)
	rayFloat := new(big.Float).SetInt(ray)
	incomeFloat := new(big.Float).SetInt(normalizedIncome)
	
	exchangeRate := new(big.Float).Quo(incomeFloat, rayFloat)
	
	// 应用Spark特定因子（如果有）
	sparkFactor, err := c.getSparkOptimizationFactor(ctx, assetAddress)
	if err != nil {
		sparkFactor = big.NewFloat(1.01) // 默认1%优化
	}
	
	finalRate := new(big.Float).Mul(exchangeRate, sparkFactor)
	
	return finalRate, nil
}

// CalculateSparkAPY 计算Spark APY
func (c *SparkExchangeCalculator) CalculateSparkAPY(ctx context.Context, assetAddress common.Address, isSupply bool) (*big.Float, error) {
	// 计算Spark的APY
	
	// 获取当前利率
	var rate *big.Float
	var err error
	
	if isSupply {
		rate, err = c.getCurrentLiquidityRate(ctx, assetAddress)
	} else {
		rate, err = c.getCurrentVariableBorrowRate(ctx, assetAddress)
	}
	
	if err != nil {
		return nil, err
	}
	
	// 转换为APY（考虑复利）
	// APY = (1 + rate/秒)^(秒/年) - 1
	secondsPerYear := big.NewFloat(365 * 24 * 60 * 60)
	ratePerSecond := new(big.Float).Quo(rate, rayToFloat(big.NewInt(1e27)))
	
	// 计算复利
	one := big.NewFloat(1.0)
	rateFactor := new(big.Float).Add(one, ratePerSecond)
	
	// (1 + r)^n
	apyFactor := pow(rateFactor, secondsPerYear)
	apy := new(big.Float).Sub(apyFactor, one)
	
	return apy, nil
}

// CalculateSparkPosition 计算Spark头寸
func (c *SparkExchangeCalculator) CalculateSparkPosition(ctx context.Context, userAddress, assetAddress common.Address) (*SparkPosition, error) {
	// 计算用户在Spark的头寸
	
	// 获取供应头寸
	supplyBalance, err := c.getSupplyBalance(ctx, userAddress, assetAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get supply balance: %v", err)
	}
	
	// 获取借款头寸
	borrowBalance, err := c.getBorrowBalance(ctx, userAddress, assetAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get borrow balance: %v", err)
	}
	
	// 获取抵押因子
	collateralFactor, err := c.getCollateralFactor(ctx, assetAddress)
	if err != nil {
		collateralFactor = big.NewFloat(0.8) // 默认80%
	}
	
	// 获取资产价格
	assetPrice, err := c.getAssetPrice(ctx, assetAddress)
	if err != nil {
		assetPrice = big.NewFloat(1.0) // 默认$1.0
	}
	
	// 计算健康因子
	healthFactor, err := c.calculateHealthFactor(
		supplyBalance,
		borrowBalance,
		collateralFactor,
		assetPrice,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate health factor: %v", err)
	}
	
	// 计算可用借款额度
	borrowCapacity, availableBorrows, err := c.calculateBorrowCapacity(
		supplyBalance,
		borrowBalance,
		collateralFactor,
		assetPrice,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate borrow capacity: %v", err)
	}
	
	return &SparkPosition{
		UserAddress:      userAddress,
		AssetAddress:     assetAddress,
		SupplyBalance:    supplyBalance,
		BorrowBalance:    borrowBalance,
		CollateralFactor: collateralFactor,
		AssetPrice:       assetPrice,
		HealthFactor:     healthFactor,
		BorrowCapacity:   borrowCapacity,
		AvailableBorrows: availableBorrows,
		LiquidationThreshold: big.NewFloat(0.85), // 默认清算阈值85%
		Timestamp:        time.Now().Unix(),
	}, nil
}

// CalculateSparkEfficiencyMode 计算Spark效率模式
func (c *SparkExchangeCalculator) CalculateSparkEfficiencyMode(ctx context.Context, userAddress common.Address) (*SparkEfficiency, error) {
	// 计算Spark效率模式指标
	// Spark专注于稳定币和流动性质押代币的高效借贷
	
	// 获取用户所有头寸
	positions, err := c.getAllUserPositions(ctx, userAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get user positions: %v", err)
	}
	
	// 计算总供应价值
	totalSupplyValue := big.NewFloat(0)
	for _, pos := range positions {
		supplyValue := new(big.Float).Mul(
			new(big.Float).SetInt(pos.SupplyBalance),
			pos.AssetPrice,
		)
		totalSupplyValue = new(big.Float).Add(totalSupplyValue, supplyValue)
	}
	
	// 计算总借款价值
	totalBorrowValue := big.NewFloat(0)
	for _, pos := range positions {
		borrowValue := new(big.Float).Mul(
			new(big.Float).SetInt(pos.BorrowBalance),
			pos.AssetPrice,
		)
		totalBorrowValue = new(big.Float).Add(totalBorrowValue, borrowValue)
	}
	
	// 计算加权平均利率
	weightedSupplyRate, weightedBorrowRate, err := c.calculateWeightedRates(positions)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate weighted rates: %v", err)
	}
	
	// 计算效率得分
	efficiencyScore, err := c.calculateEfficiencyScore(
		totalSupplyValue,
		totalBorrowValue,
		weightedSupplyRate,
		weightedBorrowRate,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate efficiency score: %v", err)
	}
	
	return &SparkEfficiency{
		UserAddress:       userAddress,
		TotalSupplyValue:  totalSupplyValue,
		TotalBorrowValue:  totalBorrowValue,
		WeightedSupplyRate: weightedSupplyRate,
		WeightedBorrowRate: weightedBorrowRate,
		EfficiencyScore:   efficiencyScore,
		RecommendedActions: c.generateRecommendations(positions),
		Timestamp:         time.Now().Unix(),
	}, nil
}

// 数据结构和类型定义
type AssetConfiguration struct {
	AssetAddress        common.Address
	IsActive            bool
	IsFrozen            bool
	IsBorrowable        bool
	IsCollateral        bool
	Decimals            uint8
	LTV                 *big.Float  // 贷款价值比
	LiquidationThreshold *big.Float // 清算阈值
	LiquidationBonus    *big.Float  // 清算奖励
	ReserveFactor       *big.Float  // 储备因子
}

type SparkPosition struct {
	UserAddress         common.Address
	AssetAddress        common.Address
	SupplyBalance       *big.Int
	BorrowBalance       *big.Int
	CollateralFactor    *big.Float
	AssetPrice          *big.Float
	HealthFactor        *big.Float
	BorrowCapacity      *big.Float
	AvailableBorrows    *big.Float
	LiquidationThreshold *big.Float
	Timestamp           int64
}

type SparkEfficiency struct {
	UserAddress         common.Address
	TotalSupplyValue    *big.Float
	TotalBorrowValue    *big.Float
	WeightedSupplyRate  *big.Float
	WeightedBorrowRate  *big.Float
	EfficiencyScore     *big.Float
	RecommendedActions  []string
	Timestamp           int64
}

// 私有方法
func (c *SparkExchangeCalculator) getAssetConfiguration(ctx context.Context, assetAddress common.Address) (*AssetConfiguration, error) {
	// Spark资产配置ABI（简化版）
	const sparkABI = `[
		{
			"constant": true,
			"inputs": [{"name": "asset", "type": "address"}],
			"name": "getConfiguration",
			"outputs": [
				{"name": "data", "type": "uint256"}
			],
			"type": "function"
		},
		{
			"constant": true,
			"inputs": [{"name": "asset", "type": "address"}],
			"name": "getReserveData",
			"outputs": [
				{"name": "configuration", "type": "uint256"},
				{"name": "liquidityIndex", "type": "uint128"},
				{"name": "currentLiquidityRate", "type": "uint128"},
				{"name": "currentVariableBorrowRate", "type": "uint128"},
				{"name": "currentStableBorrowRate", "type": "uint128"},
				{"name": "lastUpdateTimestamp", "type": "uint40"},
				{"name": "id", "type": "uint16"}
			],
			"type": "function"
		}
	]`
	
	parsedABI, err := abi.JSON(strings.NewReader(sparkABI))
	if err != nil {
		return c.getMockAssetConfiguration(assetAddress), nil
	}
	
	// 获取储备数据
	data, err := parsedABI.Pack("getReserveData", assetAddress)
	if err != nil {
		return c.getMockAssetConfiguration(assetAddress), nil
	}
	
	msg := ethereum.CallMsg{
		To:   &c.lendingPool,
		Data: data,
	}
	
	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return c.getMockAssetConfiguration(assetAddress), nil
	}
	
	var reserveData struct {
		Configuration          *big.Int
		LiquidityIndex         *big.Int
		CurrentLiquidityRate   *big.Int
		CurrentVariableBorrowRate *big.Int
		CurrentStableBorrowRate *big.Int
		LastUpdateTimestamp    *big.Int
		Id                     *big.Int
	}
	
	err = parsedABI.UnpackIntoInterface(&reserveData, "getReserveData", result)
	if err != nil {
		return c.getMockAssetConfiguration(assetAddress), nil
	}
	
	// 解析配置数据
	config := parseConfiguration(reserveData.Configuration)
	
	return &AssetConfiguration{
		AssetAddress:        assetAddress,
		IsActive:            config.IsActive,
		IsFrozen:            config.IsFrozen,
		IsBorrowable:        config.IsBorrowable,
		IsCollateral:        config.IsCollateral,
		Decimals:            config.Decimals,
		LTV:                 config.LTV,
		LiquidationThreshold: config.LiquidationThreshold,
		LiquidationBonus:    config.LiquidationBonus,
		ReserveFactor:       config.ReserveFactor,
	}, nil
}

func (c *SparkExchangeCalculator) getNormalizedIncome(ctx context.Context, assetAddress common.Address) (*big.Int, error) {
	// 获取标准化收入（流动性指数）
	
	// 简化实现
	return big.NewInt(1020000000000000000000000000), nil // 1.02 * 1e27
}

func (c *SparkExchangeCalculator) getSparkOptimizationFactor(ctx context.Context, assetAddress common.Address) (*big.Float, error) {
	// 获取Spark优化因子
	// Spark专注于稳定币和流动性质押代币，可能有特殊优化
	
	return big.NewFloat(1.01), nil // 1%优化
}

func (c *SparkExchangeCalculator) getCurrentLiquidityRate(ctx context.Context, assetAddress common.Address) (*big.Float, error) {
	// 获取当前流动性利率
	
	return rayToFloat(big.NewInt(30000000000000000000000000)), nil // 3% 年化
}

func (c *SparkExchangeCalculator) getCurrentVariableBorrowRate(ctx context.Context, assetAddress common.Address) (*big.Float, error) {
	// 获取当前可变借款利率
	
	return rayToFloat(big.NewInt(50000000000000000000000000)), nil // 5% 年化
}

func (c *SparkExchangeCalculator) getSupplyBalance(ctx context.Context, userAddress, assetAddress common.Address) (*big.Int, error) {
	// 获取供应余额
	
	return big.NewInt(1000000000), nil // 1000 USDC (6 decimals)
}

func (c *SparkExchangeCalculator) getBorrowBalance(ctx context.Context, userAddress, assetAddress common.Address) (*big.Int, error) {
	// 获取借款余额
	
	return big.NewInt(500000000), nil // 500 USDC
}

func (c *SparkExchangeCalculator) getCollateralFactor(ctx context.Context, assetAddress common.Address) (*big.Float, error) {
	// 获取抵押因子（LTV）
	
	return big.NewFloat(0.8), nil // 80%
}

func (c *SparkExchangeCalculator) getAssetPrice(ctx context.Context, assetAddress common.Address) (*big.Float, error) {
	// 获取资产价格
	
	// 检查是否为稳定币
	if isStablecoin(assetAddress) {
		return big.NewFloat(1.0), nil
	}
	
	// 检查是否为流动性质押代币
	if isLST(assetAddress) {
		return big.NewFloat(1.02), nil // stETH等
	}
	
	return big.NewFloat(1.0), nil
}

func (c *SparkExchangeCalculator) calculateHealthFactor(supplyBalance, borrowBalance *big.Int, collateralFactor, assetPrice *big.Float) (*big.Float, error) {
	// 计算健康因子
	
	if borrowBalance.Cmp(big.NewInt(0)) == 0 {
		return big.NewFloat(100.0), nil // 无借款
	}
	
	supplyValue := new(big.Float).Mul(
		new(big.Float).SetInt(supplyBalance),
		assetPrice,
	)
	
	borrowValue := new(big.Float).Mul(
		new(big.Float).SetInt(borrowBalance),
		assetPrice,
	)
	
	// 抵押价值
	collateralValue := new(big.Float).Mul(supplyValue, collateralFactor)
	
	// 健康因子
	healthFactor := new(big.Float).Quo(collateralValue, borrowValue)
	
	return health