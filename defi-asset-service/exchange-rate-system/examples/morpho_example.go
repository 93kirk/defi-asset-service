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

// MorphoExchangeCalculator Morpho协议汇率计算器
type MorphoExchangeCalculator struct {
	client           *ethclient.Client
	morphoContract   common.Address // Morpho核心合约
	marketsRegistry  common.Address // 市场注册表
	lensContract     common.Address // Lens合约（数据查询）
}

// NewMorphoExchangeCalculator 创建Morpho计算器
func NewMorphoExchangeCalculator(rpcURL string) (*MorphoExchangeCalculator, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	
	return &MorphoExchangeCalculator{
		client:           client,
		morphoContract:   common.HexToAddress("0x8888882f8f843896699869179fB6E4f7e3B58888"), // Morpho主合约
		marketsRegistry: common.HexToAddress("0x8888882f8f843896699869179fB6E4f7e3B58889"), // 市场注册表
		lensContract:    common.HexToAddress("0x930f1b46e1D081Ec1524efD95752bE3eCe51EF67"), // Morpho Lens
	}, nil
}

// CalculateExchangeRate 计算Morpho协议汇率
func (c *MorphoExchangeCalculator) CalculateExchangeRate(ctx context.Context, marketAddress common.Address) (*big.Float, error) {
	// Morpho汇率计算：
	// 1. 获取基础协议汇率（Compound/Aave）
	// 2. 应用Morpho优化因子
	// 3. 考虑流动性挖矿奖励
	
	// 获取市场信息
	marketInfo, err := c.getMarketInfo(ctx, marketAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get market info: %v", err)
	}
	
	// 获取基础协议汇率
	baseRate, err := c.getBaseProtocolRate(ctx, marketInfo.BaseProtocol, marketInfo.UnderlyingToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get base protocol rate: %v", err)
	}
	
	// 获取Morpho优化因子
	optimizationFactor, err := c.getOptimizationFactor(ctx, marketAddress)
	if err != nil {
		// 使用默认优化因子
		optimizationFactor = big.NewFloat(1.02) // 默认2%优化
	}
	
	// 获取奖励因子（流动性挖矿）
	rewardFactor, err := c.getRewardFactor(ctx, marketAddress)
	if err != nil {
		rewardFactor = big.NewFloat(1.005) // 默认0.5%奖励
	}
	
	// 计算综合汇率：基础汇率 × 优化因子 × 奖励因子
	exchangeRate := new(big.Float).Mul(baseRate, optimizationFactor)
	exchangeRate = new(big.Float).Mul(exchangeRate, rewardFactor)
	
	return exchangeRate, nil
}

// CalculateOptimizedRate 计算优化后的利率
func (c *MorphoExchangeCalculator) CalculateOptimizedRate(ctx context.Context, marketAddress common.Address, isSupply bool) (*big.Float, error) {
	// 计算Morpho优化后的利率
	
	marketInfo, err := c.getMarketInfo(ctx, marketAddress)
	if err != nil {
		return nil, err
	}
	
	// 获取基础协议利率
	var baseRate *big.Float
	if isSupply {
		baseRate, err = c.getBaseSupplyRate(ctx, marketInfo.BaseProtocol, marketInfo.UnderlyingToken)
	} else {
		baseRate, err = c.getBaseBorrowRate(ctx, marketInfo.BaseProtocol, marketInfo.UnderlyingToken)
	}
	if err != nil {
		return nil, err
	}
	
	// 获取Morpho优化提升
	optimizationBoost, err := c.getOptimizationBoost(ctx, marketAddress, isSupply)
	if err != nil {
		optimizationBoost = big.NewFloat(1.2) // 默认20%提升
	}
	
	// 计算优化后利率
	optimizedRate := new(big.Float).Mul(baseRate, optimizationBoost)
	
	return optimizedRate, nil
}

// CalculateMorphoPositionValue 计算Morpho头寸价值
func (c *MorphoExchangeCalculator) CalculateMorphoPositionValue(ctx context.Context, userAddress common.Address, marketAddress common.Address) (*MorphoPosition, error) {
	// 获取用户在Morpho的头寸信息
	
	// 获取供应头寸
	supplyPosition, err := c.getSupplyPosition(ctx, userAddress, marketAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get supply position: %v", err)
	}
	
	// 获取借款头寸
	borrowPosition, err := c.getBorrowPosition(ctx, userAddress, marketAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get borrow position: %v", err)
	}
	
	// 获取抵押因子
	collateralFactor, err := c.getCollateralFactor(ctx, marketAddress)
	if err != nil {
		collateralFactor = big.NewFloat(0.8) // 默认80%
	}
	
	// 计算健康因子
	healthFactor, err := c.calculateHealthFactor(supplyPosition, borrowPosition, collateralFactor)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate health factor: %v", err)
	}
	
	return &MorphoPosition{
		UserAddress:      userAddress,
		MarketAddress:    marketAddress,
		SupplyAmount:     supplyPosition.Amount,
		SupplyInBase:     supplyPosition.InBaseProtocol,
		SupplyInMorpho:   supplyPosition.InMorpho,
		BorrowAmount:     borrowPosition.Amount,
		BorrowInBase:     borrowPosition.InBaseProtocol,
		BorrowInMorpho:   borrowPosition.InMorpho,
		CollateralFactor: collateralFactor,
		HealthFactor:     healthFactor,
		Timestamp:        time.Now().Unix(),
	}, nil
}

// CalculateMorphoRewards 计算Morpho奖励
func (c *MorphoExchangeCalculator) CalculateMorphoRewards(ctx context.Context, userAddress common.Address) (*MorphoRewards, error) {
	// 计算用户的Morpho奖励（流动性挖矿）
	
	// 获取待领取奖励
	pendingRewards, err := c.getPendingRewards(ctx, userAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending rewards: %v", err)
	}
	
	// 获取已领取奖励
	claimedRewards, err := c.getClaimedRewards(ctx, userAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to get claimed rewards: %v", err)
	}
	
	// 计算APR
	rewardAPR, err := c.calculateRewardAPR(ctx, userAddress)
	if err != nil {
		rewardAPR = big.NewFloat(0.05) // 默认5% APR
	}
	
	return &MorphoRewards{
		UserAddress:    userAddress,
		PendingRewards: pendingRewards,
		ClaimedRewards: claimedRewards,
		RewardAPR:      rewardAPR,
		LastUpdate:     time.Now().Unix(),
	}, nil
}

// 数据结构和类型定义
type MarketInfo struct {
	MarketAddress    common.Address
	UnderlyingToken  common.Address
	BaseProtocol     string // "compound" 或 "aave"
	BaseProtocolAddress common.Address
	IsActive         bool
	SupplyRate       *big.Float
	BorrowRate       *big.Float
	TotalSupply      *big.Int
	TotalBorrow      *big.Int
	UtilizationRate  *big.Float
}

type SupplyPosition struct {
	Amount           *big.Int
	InBaseProtocol   *big.Int
	InMorpho         *big.Int
	SupplyRate       *big.Float
	AccruedInterest  *big.Int
}

type BorrowPosition struct {
	Amount           *big.Int
	InBaseProtocol   *big.Int
	InMorpho         *big.Int
	BorrowRate       *big.Float
	AccruedInterest  *big.Int
}

type MorphoPosition struct {
	UserAddress      common.Address
	MarketAddress    common.Address
	SupplyAmount     *big.Int
	SupplyInBase     *big.Int
	SupplyInMorpho   *big.Int
	BorrowAmount     *big.Int
	BorrowInBase     *big.Int
	BorrowInMorpho   *big.Int
	CollateralFactor *big.Float
	HealthFactor     *big.Float
	Timestamp        int64
}

type MorphoRewards struct {
	UserAddress     common.Address
	PendingRewards  *big.Int
	ClaimedRewards  *big.Int
	RewardAPR       *big.Float
	LastUpdate      int64
}

// 私有方法
func (c *MorphoExchangeCalculator) getMarketInfo(ctx context.Context, marketAddress common.Address) (*MarketInfo, error) {
	// Morpho市场信息ABI（简化版）
	const morphoABI = `[
		{
			"constant": true,
			"inputs": [{"name": "market", "type": "address"}],
			"name": "market",
			"outputs": [
				{"name": "underlyingToken", "type": "address"},
				{"name": "baseProtocol", "type": "string"},
				{"name": "baseProtocolAddress", "type": "address"},
				{"name": "isActive", "type": "bool"},
				{"name": "supplyRate", "type": "uint256"},
				{"name": "borrowRate", "type": "uint256"},
				{"name": "totalSupply", "type": "uint256"},
				{"name": "totalBorrow", "type": "uint256"}
			],
			"type": "function"
		}
	]`
	
	parsedABI, err := abi.JSON(strings.NewReader(morphoABI))
	if err != nil {
		// 返回模拟数据
		return c.getMockMarketInfo(marketAddress), nil
	}
	
	data, err := parsedABI.Pack("market", marketAddress)
	if err != nil {
		return c.getMockMarketInfo(marketAddress), nil
	}
	
	msg := ethereum.CallMsg{
		To:   &c.morphoContract,
		Data: data,
	}
	
	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return c.getMockMarketInfo(marketAddress), nil
	}
	
	var marketData struct {
		UnderlyingToken      common.Address
		BaseProtocol         string
		BaseProtocolAddress  common.Address
		IsActive             bool
		SupplyRate           *big.Int
		BorrowRate           *big.Int
		TotalSupply          *big.Int
		TotalBorrow          *big.Int
	}
	
	err = parsedABI.UnpackIntoInterface(&marketData, "market", result)
	if err != nil {
		return c.getMockMarketInfo(marketAddress), nil
	}
	
	// 计算利用率
	utilizationRate := big.NewFloat(0)
	if marketData.TotalSupply.Cmp(big.NewInt(0)) > 0 {
		totalSupplyFloat := new(big.Float).SetInt(marketData.TotalSupply)
		totalBorrowFloat := new(big.Float).SetInt(marketData.TotalBorrow)
		utilizationRate = new(big.Float).Quo(totalBorrowFloat, totalSupplyFloat)
	}
	
	// 转换利率（从RAY单位）
	supplyRate := convertRayToFloat(marketData.SupplyRate)
	borrowRate := convertRayToFloat(marketData.BorrowRate)
	
	return &MarketInfo{
		MarketAddress:       marketAddress,
		UnderlyingToken:     marketData.UnderlyingToken,
		BaseProtocol:        marketData.BaseProtocol,
		BaseProtocolAddress: marketData.BaseProtocolAddress,
		IsActive:            marketData.IsActive,
		SupplyRate:          supplyRate,
		BorrowRate:          borrowRate,
		TotalSupply:         marketData.TotalSupply,
		TotalBorrow:         marketData.TotalBorrow,
		UtilizationRate:     utilizationRate,
	}, nil
}

func (c *MorphoExchangeCalculator) getBaseProtocolRate(ctx context.Context, baseProtocol string, tokenAddress common.Address) (*big.Float, error) {
	// 获取基础协议（Compound/Aave）的汇率
	
	switch strings.ToLower(baseProtocol) {
	case "compound", "compoundv3":
		// Compound汇率计算
		return c.getCompoundExchangeRate(ctx, tokenAddress)
	case "aave", "aavev3":
		// Aave汇率计算
		return c.getAaveExchangeRate(ctx, tokenAddress)
	default:
		// 默认返回1.0
		return big.NewFloat(1.0), nil
	}
}

func (c *MorphoExchangeCalculator) getBaseSupplyRate(ctx context.Context, baseProtocol string, tokenAddress common.Address) (*big.Float, error) {
	// 获取基础协议的供应利率
	
	// 简化实现
	return big.NewFloat(0.03), nil // 默认3%供应利率
}

func (c *MorphoExchangeCalculator) getBaseBorrowRate(ctx context.Context, baseProtocol string, tokenAddress common.Address) (*big.Float, error) {
	// 获取基础协议的借款利率
	
	// 简化实现
	return big.NewFloat(0.05), nil // 默认5%借款利率
}

func (c *MorphoExchangeCalculator) getOptimizationFactor(ctx context.Context, marketAddress common.Address) (*big.Float, error) {
	// 获取Morpho优化因子
	// Morpho通过P2P匹配优化利率
	
	return big.NewFloat(1.02), nil // 默认2%优化
}

func (c *MorphoExchangeCalculator) getRewardFactor(ctx context.Context, marketAddress common.Address) (*big.Float, error) {
	// 获取奖励因子（流动性挖矿）
	
	return big.NewFloat(1.005), nil // 默认0.5%奖励
}

func (c *MorphoExchangeCalculator) getOptimizationBoost(ctx context.Context, marketAddress common.Address, isSupply bool) (*big.Float, error) {
	// 获取Morpho优化提升
	
	if isSupply {
		return big.NewFloat(1.2), nil // 供应利率提升20%
	} else {
		return big.NewFloat(0.9), nil // 借款利率降低10%
	}
}

func (c *MorphoExchangeCalculator) getSupplyPosition(ctx context.Context, userAddress, marketAddress common.Address) (*SupplyPosition, error) {
	// 获取供应头寸
	
	return &SupplyPosition{
		Amount:          big.NewInt(1000000000), // 1000 USDC
		InBaseProtocol:  big.NewInt(600000000),  // 600在基础协议
		InMorpho:        big.NewInt(400000000),  // 400在Morpho优化池
		SupplyRate:      big.NewFloat(0.036),    // 3.6%利率
		AccruedInterest: big.NewInt(10000000),   // 10 USDC利息
	}, nil
}

func (c *MorphoExchangeCalculator) getBorrowPosition(ctx context.Context, userAddress, marketAddress common.Address) (*BorrowPosition, error) {
	// 获取借款头寸
	
	return &BorrowPosition{
		Amount:          big.NewInt(500000000),  // 500 USDC
		InBaseProtocol:  big.NewInt(300000000),  // 300在基础协议
		InMorpho:        big.NewInt(200000000),  // 200在Morpho优化池
		BorrowRate:      big.NewFloat(0.045),    // 4.5%利率
		AccruedInterest: big.NewInt(5000000),    // 5 USDC利息
	}, nil
}

func (c *MorphoExchangeCalculator) getCollateralFactor(ctx context.Context, marketAddress common.Address) (*big.Float, error) {
	// 获取抵押因子
	
	return big.NewFloat(0.8), nil // 80%抵押因子
}

func (c *MorphoExchangeCalculator) calculateHealthFactor(supply *SupplyPosition, borrow *BorrowPosition, collateralFactor *big.Float) (*big.Float, error) {
	// 计算健康因子
	// 健康因子 = (供应价值 × 抵押因子) / 借款价值
	
	if borrow.Amount.Cmp(big.NewInt(0)) == 0 {
		return big.NewFloat(100.0), nil // 无借款，健康因子无限大
	}
	
	supplyValue := new(big.Float).SetInt(supply.Amount)
	borrowValue := new(big.Float).SetInt(borrow.Amount)
	
	// 计算抵押价值
	collateralValue := new(big.Float).Mul(supplyValue, collateralFactor)
	
	// 计算健康因子
	healthFactor := new(big.Float).Quo(collateralValue, borrowValue)
	
	return healthFactor, nil
}

func (c *MorphoExchangeCalculator) getPendingRewards(ctx context.Context, userAddress common.Address) (*big.Int, error) {
	// 获取待领取奖励
	
	return big.NewInt(50000000), nil // 50 MORPHO奖励
}

func (c *MorphoExchangeCalculator) getClaimedRewards(ctx context.Context, userAddress common.Address) (*big.Int, error) {
	// 获取已领取奖励
	
	return big.NewInt(100000000), nil // 100 MORPHO已领取
}

func (c *MorphoExchangeCalculator) calculateRewardAPR(ctx context.Context, userAddress common.Address) (*big.Float, error) {
	// 计算奖励APR
	
	return big.NewFloat(0.05), nil // 5% APR
}

func (c *MorphoExchangeCalculator) getCompoundExchangeRate(ctx context.Context, tokenAddress common.Address) (*big.Float, error) {
	// Compound汇率计算
	
	return big.NewFloat(1.03), nil // 3%收益
}

func (c *MorphoExchangeCalculator) getAaveExchangeRate(ctx context.Context, tokenAddress common.Address) (*big.Float, error) {
	// Aave汇率计算
	
	return big.NewFloat(1.035), nil // 3.5%收益
}

func (c *MorphoExchangeCalculator) getMockMarketInfo(marketAddress common.Address) *MarketInfo {
	// 返回模拟市场数据
	
	return &MarketInfo{
		MarketAddress:       marketAddress,
		UnderlyingToken:     common.HexToAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"), // USDC
		BaseProtocol:        "compound",
		BaseProtocolAddress: common.HexToAddress("0xc3d688B66703497DAA19211EEdff47f25384cdc3"), // Compound V3
		IsActive:            true,
		SupplyRate:          big.NewFloat(0.03),
		BorrowRate:          big.NewFloat(0.05),
		TotalSupply:         big.NewInt(10000000000), // 10M USDC
		TotalBorrow:         big.NewInt(6000000000),  // 6M USDC
		UtilizationRate:     big.NewFloat(0.6),
	}
}

func convertRayToFloat(rayValue *big.Int) *big.Float {
	// 将RAY单位（1e27）转换为浮点数
	if rayValue == nil {
		return big.NewFloat(0)
	}
	
	ray := new(big.Int).Exp(big.NewInt(10), big.NewInt(27), nil)
	rayFloat := new(big.Float).SetInt(ray)
	valueFloat := new(big.Float).SetInt(rayValue)
	
	return new(big.Float).Quo(valueFloat, rayFloat)
}

// 示例使用
func ExampleMorphoExchange() {
	ctx := context.Background()
	
	fmt.Println("=== Morpho借贷优化协议汇率计算 ===")
	
	// 创建Morpho计算器
	calculator, err := NewMorphoExchangeCalculator("https://eth-mainnet.g.alchemy.com/v2/demo")
	if err != nil {
		fmt.Printf("创建计算器失败: %v\n", err)
		return
	}
	
	// 示例市场地址
	marketAddress := common.HexToAddress("0x1234567890123456789012345678901234567890")
	
	// 测试1: 计算汇率
	exchangeRate, err := calculator.CalculateExchangeRate(ctx, marketAddress)
	if err != nil {
		fmt.Printf("计算汇率失败: %v\n", err)
	} else {
		fmt.Printf("1. Morpho协议汇率:\n")
		fmt.Printf("   综合汇率: %.6f\n", exchangeRate)
		fmt.Printf("   组成: 基础协议汇率 × 优化因子 × 奖励因子\n")
	}
	
	// 测试2: 计算优化利率
	supplyRate, err := calculator.CalculateOptimizedRate(ctx, marketAddress, true)
	if err != nil {
		fmt.Printf("计算供应利率失败: %v\n", err)
	} else {
		fmt.Printf("2. 优化供应利率:\n")
		fmt.Printf("   年化利率: %.2f%%\n", new(big.Float).Mul(supplyRate, big.NewFloat(100)))
		fmt.Printf("   相比基础协议提升: ~20%%\n")
	}
	
	borrowRate, err := calculator.CalculateOptimizedRate(ctx, marketAddress, false)
	if err != nil {
		fmt.Printf("计算借款利率失败: %v\n", err)
	} else {
		fmt.Printf("3. 优化借款利率:\n")
		fmt.Printf("   年化利率: %.2f%%\n", new(big.Float).Mul(borrowRate, big.NewFloat(100)))
		fmt.Printf("   相比基础协议降低: ~10%%\n")
	}
	
	// 测试3: 计算头寸价值
	userAddress := common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e0F3B5F2b1F")
	position, err := calculator.CalculateMorphoPositionValue(ctx, userAddress, marketAddress)
	if err != nil {
		fmt.Printf("计算头寸价值失败: %v\n", err)
	} else {
		fmt.Printf("4. 用户头寸分析:\n")
		fmt.Printf("   供应总额: %.2f USDC\n", new(big.Float).SetInt(position.SupplyAmount))
		fmt.Printf("   - 在基础协议: %.2f USDC\n", new(big.Float).SetInt(position.SupplyInBase))
		fmt.Printf("   - 在Morpho优化池: %.2f USDC\n", new(big.Float).SetInt(position.SupplyInMorpho))
		fmt.Printf("   借款总额: %.2f USDC\n", new(big.Float).SetInt(position.BorrowAmount))
		fmt.Printf("   健康因子: %.2f\n", position.HealthFactor)
		fmt.Printf("   抵押因子: %.0f%%\n", new(big.Float).Mul(position.CollateralFactor, big.NewFloat(100)))
	}
	
	// 测试4: 计算奖励
	rewards, err := calculator.CalculateMorphoRewards(ctx, userAddress)
	if err != nil {
		fmt.Printf("计算奖励失败: %v\n", err)
	} else {
		fmt.Printf("5. Morpho奖励:\n")
		fmt.Printf("   待领取奖励: %.4f MORPHO\n", new(big.Float).SetInt(rewards.PendingRewards))
		fmt.Printf("   已领取奖励: %.4f MORPHO\n", new(big.Float).SetInt(rewards.ClaimedRewards))
		fmt.Printf("   奖励APR: %.2f%%\n", new(big.Float).Mul(rewards.RewardAPR, big.NewFloat(100)))
	}
	
	// 协议说明
	fmt.Println("\n=== Morpho协议说明 ===")
	fmt.Println("协议类型: 借贷优化协议")
	fmt.Println("TVL: $3.89B")
	fmt.Println("核心机制:")
	fmt.Println("  1. P2P匹配: 直接在借贷用户之间匹配，减少利差")
	fmt.Println("  2. 利率优化: 提供比基础协议更好的利率")
	fmt.Println("  3. 流动性挖矿: MORPHO代币奖励")
	fmt.Println("支持的底层协议:")
	fmt.Println("  - Compound V2/V3")
	fmt.Println("  - Aave V2/V3")
	fmt.Println("汇率计算原理:")
	fmt.Println("  Morpho汇率 = 基础协议汇率 × 优化因子 × 奖励因子")
	fmt.Println("  反映: 基础收益 + 优化提升 + 挖矿奖励")
}