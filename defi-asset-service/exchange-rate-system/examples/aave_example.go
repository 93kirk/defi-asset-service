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

// AaveExchangeCalculator Aave汇率计算器
type AaveExchangeCalculator struct {
	client      *ethclient.Client
	poolAddress common.Address
	aTokenABI   abi.ABI
}

// NewAaveExchangeCalculator 创建Aave计算器
func NewAaveExchangeCalculator(rpcURL string, aTokenAddress string) (*AaveExchangeCalculator, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	
	// Aave aToken ABI（简化版）
	const aTokenABI = `[
		{
			"constant": true,
			"inputs": [],
			"name": "exchangeRateStored",
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
			"name": "totalBorrows",
			"outputs": [{"name": "", "type": "uint256"}],
			"type": "function"
		},
		{
			"constant": true,
			"inputs": [],
			"name": "totalReserves",
			"outputs": [{"name": "", "type": "uint256"}],
			"type": "function"
		},
		{
			"constant": true,
			"inputs": [],
			"name": "getReserveNormalizedIncome",
			"outputs": [{"name": "", "type": "uint256"}],
			"type": "function"
		}
	]`
	
	parsedABI, err := abi.JSON(strings.NewReader(aTokenABI))
	if err != nil {
		return nil, err
	}
	
	return &AaveExchangeCalculator{
		client:      client,
		poolAddress: common.HexToAddress(aTokenAddress),
		aTokenABI:   parsedABI,
	}, nil
}

// CalculateExchangeRateStored 计算存储的汇率
func (c *AaveExchangeCalculator) CalculateExchangeRateStored(ctx context.Context) (*big.Float, error) {
	// 调用exchangeRateStored()函数
	data, err := c.aTokenABI.Pack("exchangeRateStored")
	if err != nil {
		return nil, fmt.Errorf("failed to pack data: %v", err)
	}
	
	result, err := c.client.CallContract(ctx, ethereum.CallMsg{
		To:   &c.poolAddress,
		Data: data,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call contract: %v", err)
	}
	
	// 解析结果
	var exchangeRateRaw *big.Int
	err = c.aTokenABI.UnpackIntoInterface(&exchangeRateRaw, "exchangeRateStored", result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack result: %v", err)
	}
	
	// Aave汇率以RAY单位存储（1e27）
	// 实际汇率 = exchangeRateRaw / 1e27
	ray := new(big.Int).Exp(big.NewInt(10), big.NewInt(27), nil)
	exchangeRate := new(big.Float).Quo(
		new(big.Float).SetInt(exchangeRateRaw),
		new(big.Float).SetInt(ray),
	)
	
	return exchangeRate, nil
}

// CalculateExchangeRateManual 手动计算汇率（理解原理）
func (c *AaveExchangeCalculator) CalculateExchangeRateManual(ctx context.Context) (*big.Float, error) {
	// 1. 获取总借款
	totalBorrows, err := c.getTotalBorrows(ctx)
	if err != nil {
		return nil, err
	}
	
	// 2. 获取总储备
	totalReserves, err := c.getTotalReserves(ctx)
	if err != nil {
		return nil, err
	}
	
	// 3. 获取总供应量
	totalSupply, err := c.getTotalSupply(ctx)
	if err != nil {
		return nil, err
	}
	
	// 4. 计算汇率：汇率 = (总借款 + 总储备) / 总供应量
	// 注意：Aave使用复利计算，这里简化
	
	numerator := new(big.Int).Add(totalBorrows, totalReserves)
	
	if totalSupply.Cmp(big.NewInt(0)) == 0 {
		return big.NewFloat(1.0), nil
	}
	
	// 转换为浮点数计算
	numeratorFloat := new(big.Float).SetInt(numerator)
	totalSupplyFloat := new(big.Float).SetInt(totalSupply)
	
	exchangeRate := new(big.Float).Quo(numeratorFloat, totalSupplyFloat)
	
	return exchangeRate, nil
}

// CalculateNormalizedIncome 计算标准化收入（更准确的方法）
func (c *AaveExchangeCalculator) CalculateNormalizedIncome(ctx context.Context) (*big.Float, error) {
	// Aave V3使用getReserveNormalizedIncome
	data, err := c.aTokenABI.Pack("getReserveNormalizedIncome")
	if err != nil {
		return nil, err
	}
	
	result, err := c.client.CallContract(ctx, ethereum.CallMsg{
		To:   &c.poolAddress,
		Data: data,
	}, nil)
	if err != nil {
		return nil, err
	}
	
	var normalizedIncome *big.Int
	err = c.aTokenABI.UnpackIntoInterface(&normalizedIncome, "getReserveNormalizedIncome", result)
	if err != nil {
		return nil, err
	}
	
	// 标准化收入以RAY单位存储
	ray := new(big.Int).Exp(big.NewInt(10), big.NewInt(27), nil)
	rate := new(big.Float).Quo(
		new(big.Float).SetInt(normalizedIncome),
		new(big.Float).SetInt(ray),
	)
	
	return rate, nil
}

// CalculateATokenAmount 计算aToken数量
func (c *AaveExchangeCalculator) CalculateATokenAmount(ctx context.Context, underlyingAmount *big.Float, assetDecimals int) (*big.Float, error) {
	// 获取汇率
	exchangeRate, err := c.CalculateExchangeRateStored(ctx)
	if err != nil {
		return nil, err
	}
	
	// aToken数量 = 基础资产数量 × 汇率
	aTokenAmount := new(big.Float).Mul(underlyingAmount, exchangeRate)
	
	// 考虑小数位数调整
	decimalsFactor := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(assetDecimals)), nil))
	aTokenAmount = new(big.Float).Quo(aTokenAmount, decimalsFactor)
	
	return aTokenAmount, nil
}

// 私有方法
func (c *AaveExchangeCalculator) getTotalBorrows(ctx context.Context) (*big.Int, error) {
	data, err := c.aTokenABI.Pack("totalBorrows")
	if err != nil {
		return nil, err
	}
	
	result, err := c.client.CallContract(ctx, ethereum.CallMsg{
		To:   &c.poolAddress,
		Data: data,
	}, nil)
	if err != nil {
		return nil, err
	}
	
	var totalBorrows *big.Int
	err = c.aTokenABI.UnpackIntoInterface(&totalBorrows, "totalBorrows", result)
	return totalBorrows, err
}

func (c *AaveExchangeCalculator) getTotalReserves(ctx context.Context) (*big.Int, error) {
	data, err := c.aTokenABI.Pack("totalReserves")
	if err != nil {
		return nil, err
	}
	
	result, err := c.client.CallContract(ctx, ethereum.CallMsg{
		To:   &c.poolAddress,
		Data: data,
	}, nil)
	if err != nil {
		return nil, err
	}
	
	var totalReserves *big.Int
	err = c.aTokenABI.UnpackIntoInterface(&totalReserves, "totalReserves", result)
	return totalReserves, err
}

func (c *AaveExchangeCalculator) getTotalSupply(ctx context.Context) (*big.Int, error) {
	data, err := c.aTokenABI.Pack("totalSupply")
	if err != nil {
		return nil, err
	}
	
	result, err := c.client.CallContract(ctx, ethereum.CallMsg{
		To:   &c.poolAddress,
		Data: data,
	}, nil)
	if err != nil {
		return nil, err
	}
	
	var totalSupply *big.Int
	err = c.aTokenABI.UnpackIntoInterface(&totalSupply, "totalSupply", result)
	return totalSupply, err
}

// 示例：Aave USDC市场
func ExampleAaveUSDC() {
	ctx := context.Background()
	
	// Aave V3 USDC aToken地址
	aUSDCAddress := "0x98C23E9d8f34FEFb1B7BD6a91B7FF122F4e16F5c"
	
	calculator, err := NewAaveExchangeCalculator(
		"https://eth-mainnet.g.alchemy.com/v2/demo",
		aUSDCAddress,
	)
	if err != nil {
		fmt.Printf("Failed to create calculator: %v\n", err)
		return
	}
	
	// 方法1：使用exchangeRateStored
	exchangeRate, err := calculator.CalculateExchangeRateStored(ctx)
	if err != nil {
		fmt.Printf("Failed to calculate exchange rate: %v\n", err)
		return
	}
	
	fmt.Printf("Aave USDC → aUSDC 汇率 (exchangeRateStored): %s\n", exchangeRate.String())
	
	// 方法2：手动计算
	manualRate, err := calculator.CalculateExchangeRateManual(ctx)
	if err != nil {
		fmt.Printf("Failed to calculate manual rate: %v\n", err)
		return
	}
	
	fmt.Printf("Aave USDC → aUSDC 汇率 (手动计算): %s\n", manualRate.String())
	
	// 方法3：使用标准化收入（Aave V3推荐）
	normalizedIncome, err := calculator.CalculateNormalizedIncome(ctx)
	if err != nil {
		fmt.Printf("Failed to calculate normalized income: %v\n", err)
		return
	}
	
	fmt.Printf("Aave USDC 标准化收入: %s\n", normalizedIncome.String())
	
	// 计算10,000 USDC对应的aUSDC数量
	usdcAmount := big.NewFloat(10000.0) // 10,000 USDC
	aTokenAmount, err := calculator.CalculateATokenAmount(ctx, usdcAmount, 6) // USDC有6位小数
	if err != nil {
		fmt.Printf("Failed to calculate aToken amount: %v\n", err)
		return
	}
	
	fmt.Printf("10,000 USDC = %s aUSDC\n", aTokenAmount.String())
	
	// 解释汇率含义
	fmt.Println("\n汇率解释:")
	fmt.Println("1. exchangeRateStored: 存储的汇率，考虑利息累积")
	fmt.Println("2. 手动计算: (总借款 + 总储备) / 总供应量")
	fmt.Println("3. 标准化收入: Aave V3的推荐方法，考虑复利")
	fmt.Println("汇率 > 1.0 表示有正的利息收入")
}

// Aave汇率计算原理说明
func ExplainAaveExchangeRate() {
	fmt.Println("=== Aave汇率计算原理 ===")
	fmt.Println()
	fmt.Println("1. 基本概念:")
	fmt.Println("   - aToken: 计息代币，代表存款+利息")
	fmt.Println("   - 汇率: 1个基础资产 = X个aToken")
	fmt.Println("   - 汇率随时间增长（利息累积）")
	fmt.Println()
	fmt.Println("2. 计算公式:")
	fmt.Println("   exchangeRate = (totalBorrows + totalReserves) / totalSupply")
	fmt.Println()
	fmt.Println("3. 关键组件:")
	fmt.Println("   - totalBorrows: 总借款金额（含利息）")
	fmt.Println("   - totalReserves: 协议储备金")
	fmt.Println("   - totalSupply: aToken总供应量")
	fmt.Println()
	fmt.Println("4. 实际实现:")
	fmt.Println("   - Aave使用复利计算: rate = (1 + supplyRate)^time")
	fmt.Println("   - 标准化收入: 考虑区块高度的复利累积")
	fmt.Println("   - exchangeRateStored: 上一次交互时的汇率")
	fmt.Println()
	fmt.Println("5. 示例:")
	fmt.Println("   初始: 存入100 USDC，获得100 aUSDC，汇率=1.0")
	fmt.Println("   1年后: 利率5%，汇率=1.05，100 aUSDC可赎回105 USDC")
}