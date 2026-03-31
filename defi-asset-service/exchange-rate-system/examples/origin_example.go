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

// OriginExchangeCalculator Origin协议汇率计算器
type OriginExchangeCalculator struct {
	client          *ethclient.Client
	ogvContract     common.Address // OGV代币合约
	ousdContract    common.Address // OUSD稳定币合约
	stakingContract common.Address // 质押合约
}

// NewOriginExchangeCalculator 创建Origin计算器
func NewOriginExchangeCalculator(rpcURL string) (*OriginExchangeCalculator, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	
	return &OriginExchangeCalculator{
		client:          client,
		ogvContract:     common.HexToAddress("0x9c354503C38481a7A7a51629142963F98eCC12D0"), // OGV代币
		ousdContract:    common.HexToAddress("0x2A8e1E676Ec238d8A992307B495b45B3fEAa5e86"), // OUSD稳定币
		stakingContract: common.HexToAddress("0x0C4576Ca1c365868E162554AF8e385dc3e7C66D9"), // 质押合约
	}, nil
}

// CalculateExchangeRate 计算Origin协议汇率
func (c *OriginExchangeCalculator) CalculateExchangeRate(ctx context.Context) (*big.Float, error) {
	// Origin协议主要涉及：
	// 1. OUSD稳定币（与美元1:1锚定）
	// 2. OGV治理代币（质押获得收益）
	// 3. 质押奖励系统
	
	// 方法1：获取OUSD价格（应该接近1.0）
	ousdRate, err := c.getOUSDExchangeRate(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OUSD rate: %v", err)
	}
	
	// 方法2：获取OGV质押奖励率
	stakingRewardRate, err := c.getStakingRewardRate(ctx)
	if err != nil {
		// 如果获取失败，使用默认值
		stakingRewardRate = big.NewFloat(1.05) // 默认5%年化
	}
	
	// 综合汇率：OUSD汇率 × 质押奖励因子
	// 对于Origin协议，主要关注OUSD的稳定性
	combinedRate := new(big.Float).Mul(ousdRate, stakingRewardRate)
	
	return combinedRate, nil
}

// CalculateOGVStakingRewards 计算OGV质押奖励
func (c *OriginExchangeCalculator) CalculateOGVStakingRewards(ctx context.Context, ogvAmount *big.Float, days int) (*big.Float, error) {
	// 获取年化收益率
	apr, err := c.getStakingAPR(ctx)
	if err != nil {
		// 默认APR：8%
		apr = big.NewFloat(0.08)
	}
	
	// 计算日收益率
	dailyRate := new(big.Float).Quo(apr, big.NewFloat(365))
	
	// 计算复利
	totalRewards := new(big.Float).Set(ogvAmount)
	for i := 0; i < days; i++ {
		dailyReward := new(big.Float).Mul(totalRewards, dailyRate)
		totalRewards = new(big.Float).Add(totalRewards, dailyReward)
	}
	
	// 计算总奖励（减去本金）
	rewards := new(big.Float).Sub(totalRewards, ogvAmount)
	
	return rewards, nil
}

// CalculateOUSDYield 计算OUSD收益
func (c *OriginExchangeCalculator) CalculateOUSDYield(ctx context.Context, ousdAmount *big.Float) (*big.Float, error) {
	// OUSD通过算法稳定币机制产生收益
	// 获取当前收益率
	yieldRate, err := c.getOUSDYieldRate(ctx)
	if err != nil {
		// 默认收益率：3%
		yieldRate = big.NewFloat(1.03)
	}
	
	// 计算收益
	yieldAmount := new(big.Float).Mul(ousdAmount, yieldRate)
	
	return yieldAmount, nil
}

// 私有方法
func (c *OriginExchangeCalculator) getOUSDExchangeRate(ctx context.Context) (*big.Float, error) {
	// OUSD是算法稳定币，目标与美元1:1
	// 这里可以查询预言机或DEX价格
	
	// 简化实现：返回接近1.0的值
	// 实际应该查询Chainlink或Uniswap价格
	
	// 模拟价格查询
	rate := big.NewFloat(1.001) // 略高于1.0，反映收益
	
	return rate, nil
}

func (c *OriginExchangeCalculator) getStakingRewardRate(ctx context.Context) (*big.Float, error) {
	// 获取OGV质押奖励率
	// 查询质押合约获取当前奖励率
	
	const stakingABI = `[
		{
			"constant": true,
			"inputs": [],
			"name": "rewardRate",
			"outputs": [{"name": "", "type": "uint256"}],
			"type": "function"
		},
		{
			"constant": true,
			"inputs": [],
			"name": "totalSupply",
			"outputs": [{"name": "", "type": "uint256"}],
			"type": "function"
		},
		{
			"constant": true,
			"inputs": [],
			"name": "balanceOf",
			"outputs": [{"name": "", "type": "uint256"}],
			"type": "function"
		}
	]`
	
	parsedABI, err := abi.JSON(strings.NewReader(stakingABI))
	if err != nil {
		return big.NewFloat(1.05), nil // 默认5%
	}
	
	// 尝试调用合约
	data, err := parsedABI.Pack("rewardRate")
	if err != nil {
		return big.NewFloat(1.05), nil
	}
	
	msg := ethereum.CallMsg{
		To:   &c.stakingContract,
		Data: data,
	}
	
	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return big.NewFloat(1.05), nil
	}
	
	if len(result) == 0 {
		return big.NewFloat(1.05), nil
	}
	
	var rewardRate *big.Int
	err = parsedABI.UnpackIntoInterface(&rewardRate, "rewardRate", result)
	if err != nil {
		return big.NewFloat(1.05), nil
	}
	
	// 转换为年化收益率（假设奖励率以每秒计算）
	// rewardRate通常以每秒token数量表示
	secondsPerYear := big.NewInt(365 * 24 * 60 * 60)
	annualRewards := new(big.Int).Mul(rewardRate, secondsPerYear)
	
	// 获取总质押量
	totalStaked, err := c.getTotalStaked(ctx, parsedABI)
	if err != nil {
		return big.NewFloat(1.05), nil
	}
	
	if totalStaked.Cmp(big.NewInt(0)) == 0 {
		return big.NewFloat(1.05), nil
	}
	
	// 计算APR
	annualRewardsFloat := new(big.Float).SetInt(annualRewards)
	totalStakedFloat := new(big.Float).SetInt(totalStaked)
	apr := new(big.Float).Quo(annualRewardsFloat, totalStakedFloat)
	
	// 转换为奖励率因子（1 + APR）
	rewardRateFactor := new(big.Float).Add(big.NewFloat(1.0), apr)
	
	return rewardRateFactor, nil
}

func (c *OriginExchangeCalculator) getTotalStaked(ctx context.Context, stakingABI abi.ABI) (*big.Int, error) {
	data, err := stakingABI.Pack("totalSupply")
	if err != nil {
		return nil, err
	}
	
	msg := ethereum.CallMsg{
		To:   &c.stakingContract,
		Data: data,
	}
	
	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, err
	}
	
	if len(result) == 0 {
		return big.NewInt(0), nil
	}
	
	var totalStaked *big.Int
	err = stakingABI.UnpackIntoInterface(&totalStaked, "totalSupply", result)
	return totalStaked, err
}

func (c *OriginExchangeCalculator) getStakingAPR(ctx context.Context) (*big.Float, error) {
	// 简化实现
	// 实际应该从合约或API获取
	
	return big.NewFloat(0.08), nil // 8% APR
}

func (c *OriginExchangeCalculator) getOUSDYieldRate(ctx context.Context) (*big.Float, error) {
	// OUSD收益率
	// 实际应该查询协议数据
	
	return big.NewFloat(1.03), nil // 3% 收益率
}

// 示例使用
func ExampleOriginExchange() {
	ctx := context.Background()
	
	// 创建计算器
	calculator, err := NewOriginExchangeCalculator("https://eth-mainnet.g.alchemy.com/v2/demo")
	if err != nil {
		fmt.Printf("Failed to create calculator: %v\n", err)
		return
	}
	
	// 计算汇率
	exchangeRate, err := calculator.CalculateExchangeRate(ctx)
	if err != nil {
		fmt.Printf("Failed to calculate exchange rate: %v\n", err)
		return
	}
	
	fmt.Printf("Origin协议综合汇率: %s\n", exchangeRate.String())
	
	// 计算OGV质押奖励
	ogvAmount := big.NewFloat(1000.0) // 1000 OGV
	rewards, err := calculator.CalculateOGVStakingRewards(ctx, ogvAmount, 30) // 30天
	if err != nil {
		fmt.Printf("Failed to calculate staking rewards: %v\n", err)
		return
	}
	
	fmt.Printf("1000 OGV 30天质押奖励: %s OGV\n", rewards.String())
	
	// 计算OUSD收益
	ousdAmount := big.NewFloat(10000.0) // 10,000 OUSD
	yield, err := calculator.CalculateOUSDYield(ctx, ousdAmount)
	if err != nil {
		fmt.Printf("Failed to calculate OUSD yield: %v\n", err)
		return
	}
	
	fmt.Printf("10,000 OUSD 收益: %s OUSD\n", yield.String())
	
	// 解释Origin协议
	fmt.Println("\n=== Origin协议说明 ===")
	fmt.Println("1. OUSD: 算法稳定币，目标与美元1:1锚定")
	fmt.Println("2. OGV: 治理代币，质押获得奖励")
	fmt.Println("3. 收益来源:")
	fmt.Println("   - OUSD: 通过算法稳定币机制产生收益")
	fmt.Println("   - OGV: 质押奖励和治理权利")
	fmt.Println("4. 汇率计算:")
	fmt.Println("   - 综合汇率 = OUSD汇率 × 质押奖励因子")
	fmt.Println("   - OUSD汇率 ≈ 1.0 (稳定币)")
	fmt.Println("   - 质押奖励因子 > 1.0 (反映收益)")
}