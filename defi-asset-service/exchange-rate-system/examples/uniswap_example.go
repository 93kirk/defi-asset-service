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

// UniswapV3Calculator Uniswap V3计算器
type UniswapV3Calculator struct {
	client      *ethclient.Client
	poolAddress common.Address
	poolABI     abi.ABI
}

// NewUniswapV3Calculator 创建Uniswap V3计算器
func NewUniswapV3Calculator(rpcURL, poolAddress string) (*UniswapV3Calculator, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	
	// Uniswap V3 Pool ABI（简化版）
	const poolABI = `[
		{
			"constant": true,
			"inputs": [],
			"name": "slot0",
			"outputs": [
				{"name": "sqrtPriceX96", "type": "uint160"},
				{"name": "tick", "type": "int24"},
				{"name": "observationIndex", "type": "uint16"},
				{"name": "observationCardinality", "type": "uint16"},
				{"name": "observationCardinalityNext", "type": "uint16"},
				{"name": "feeProtocol", "type": "uint8"},
				{"name": "unlocked", "type": "bool"}
			],
			"type": "function"
		},
		{
			"constant": true,
			"inputs": [],
			"name": "liquidity",
			"outputs": [{"name": "", "type": "uint128"}],
			"type": "function"
		},
		{
			"constant": true,
			"inputs": [],
			"name": "token0",
			"outputs": [{"name": "", "type": "address"}],
			"type": "function"
		},
		{
			"constant": true,
			"inputs": [],
			"name": "token1",
			"outputs": [{"name": "", "type": "address"}],
			"type": "function"
		},
		{
			"constant": true,
			"inputs": [],
			"name": "fee",
			"outputs": [{"name": "", "type": "uint24"}],
			"type": "function"
		}
	]`
	
	parsedABI, err := abi.JSON(strings.NewReader(poolABI))
	if err != nil {
		return nil, err
	}
	
	return &UniswapV3Calculator{
		client:      client,
		poolAddress: common.HexToAddress(poolAddress),
		poolABI:     parsedABI,
	}, nil
}

// GetPoolState 获取池子状态
func (c *UniswapV3Calculator) GetPoolState(ctx context.Context) (*PoolState, error) {
	// 获取slot0信息（包含当前价格）
	slot0Data, err := c.poolABI.Pack("slot0")
	if err != nil {
		return nil, err
	}
	
	slot0Result, err := c.client.CallContract(ctx, ethereum.CallMsg{
		To:   &c.poolAddress,
		Data: slot0Data,
	}, nil)
	if err != nil {
		return nil, err
	}
	
	// 解析slot0
	var slot0 struct {
		SqrtPriceX96            *big.Int
		Tick                    int24
		ObservationIndex        uint16
		ObservationCardinality  uint16
		ObservationCardinalityNext uint16
		FeeProtocol            uint8
		Unlocked               bool
	}
	
	err = c.poolABI.UnpackIntoInterface(&slot0, "slot0", slot0Result)
	if err != nil {
		return nil, err
	}
	
	// 获取流动性
	liquidityData, err := c.poolABI.Pack("liquidity")
	if err != nil {
		return nil, err
	}
	
	liquidityResult, err := c.client.CallContract(ctx, ethereum.CallMsg{
		To:   &c.poolAddress,
		Data: liquidityData,
	}, nil)
	if err != nil {
		return nil, err
	}
	
	var liquidity *big.Int
	err = c.poolABI.UnpackIntoInterface(&liquidity, "liquidity", liquidityResult)
	if err != nil {
		return nil, err
	}
	
	// 获取代币信息
	token0Data, _ := c.poolABI.Pack("token0")
	token1Data, _ := c.poolABI.Pack("token1")
	feeData, _ := c.poolABI.Pack("fee")
	
	token0Result, _ := c.client.CallContract(ctx, ethereum.CallMsg{To: &c.poolAddress, Data: token0Data}, nil)
	token1Result, _ := c.client.CallContract(ctx, ethereum.CallMsg{To: &c.poolAddress, Data: token1Data}, nil)
	feeResult, _ := c.client.CallContract(ctx, ethereum.CallMsg{To: &c.poolAddress, Data: feeData}, nil)
	
	var token0, token1 common.Address
	var fee *big.Int
	
	c.poolABI.UnpackIntoInterface(&token0, "token0", token0Result)
	c.poolABI.UnpackIntoInterface(&token1, "token1", token1Result)
	c.poolABI.UnpackIntoInterface(&fee, "fee", feeResult)
	
	// 计算实际价格
	price := c.calculatePriceFromSqrtPriceX96(slot0.SqrtPriceX96)
	
	return &PoolState{
		SqrtPriceX96: slot0.SqrtPriceX96,
		Tick:         slot0.Tick,
		Liquidity:    liquidity,
		Token0:       token0,
		Token1:       token1,
		Fee:          fee,
		Price:        price,
	}, nil
}

// CalculateLPTokens 计算LP代币数量（Uniswap V3使用NFT，这里计算流动性份额）
func (c *UniswapV3Calculator) CalculateLPTokens(ctx context.Context, amount0, amount1 *big.Int, tickLower, tickUpper int24) (*big.Int, error) {
	// Uniswap V3的流动性计算基于tick范围
	
	// 1. 获取当前tick
	poolState, err := c.GetPoolState(ctx)
	if err != nil {
		return nil, err
	}
	
	currentTick := poolState.Tick
	
	// 2. 检查tick范围是否有效
	if currentTick < tickLower || currentTick >= tickUpper {
		return big.NewInt(0), fmt.Errorf("current tick %d not in range [%d, %d)", currentTick, tickLower, tickUpper)
	}
	
	// 3. 计算流动性数量
	// 公式: L = Δx / (1/√P - 1/√P_upper) = Δy / (√P - √P_lower)
	
	// 计算sqrtPrice
	sqrtPrice := poolState.SqrtPriceX96
	sqrtPriceLower := c.tickToSqrtPrice(tickLower)
	sqrtPriceUpper := c.tickToSqrtPrice(tickUpper)
	
	// 计算两种方式得到的流动性，取最小值
	liquidity0 := c.calculateLiquidityForAmount0(amount0, sqrtPrice, sqrtPriceUpper)
	liquidity1 := c.calculateLiquidityForAmount1(amount1, sqrtPrice, sqrtPriceLower)
	
	// 取最小值以确保有足够的两种代币
	liquidity := liquidity0
	if liquidity1.Cmp(liquidity) < 0 {
		liquidity = liquidity1
	}
	
	return liquidity, nil
}

// CalculateExchangeRate 计算兑换率（基于流动性份额）
func (c *UniswapV3Calculator) CalculateExchangeRate(ctx context.Context, amount0, amount1 *big.Int) (*big.Float, error) {
	// 获取池子总流动性
	poolState, err := c.GetPoolState(ctx)
	if err != nil {
		return nil, err
	}
	
	totalLiquidity := poolState.Liquidity
	
	// 计算添加的流动性
	// 简化：假设在当前价格附近添加流动性
	addedLiquidity := c.calculateLiquidityFromAmounts(amount0, amount1, poolState.SqrtPriceX96)
	
	if totalLiquidity.Cmp(big.NewInt(0)) == 0 {
		return big.NewFloat(1.0), nil
	}
	
	// 流动性份额比例
	totalLiquidityFloat := new(big.Float).SetInt(totalLiquidity)
	addedLiquidityFloat := new(big.Float).SetInt(addedLiquidity)
	
	// 份额比例 = 添加的流动性 / 总流动性
	shareRatio := new(big.Float).Quo(addedLiquidityFloat, totalLiquidityFloat)
	
	return shareRatio, nil
}

// 私有方法
func (c *UniswapV3Calculator) calculatePriceFromSqrtPriceX96(sqrtPriceX96 *big.Int) *big.Float {
	// 价格 = (sqrtPriceX96 / 2^96)^2
	// 1. 转换为浮点数
	sqrtPrice := new(big.Float).SetInt(sqrtPriceX96)
	
	// 2. 除以2^96
	two96 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(2), big.NewInt(96), nil))
	sqrtPriceNormalized := new(big.Float).Quo(sqrtPrice, two96)
	
	// 3. 平方得到价格
	price := new(big.Float).Mul(sqrtPriceNormalized, sqrtPriceNormalized)
	
	return price
}

func (c *UniswapV3Calculator) tickToSqrtPrice(tick int24) *big.Int {
	// tick到sqrtPrice的转换公式
	// sqrtPrice = 1.0001^(tick/2) * 2^96
	
	// 简化实现
	base := big.NewFloat(1.0001)
	tickFloat := new(big.Float).SetInt64(int64(tick))
	
	// 计算1.0001^(tick/2)
	exponent := new(big.Float).Quo(tickFloat, big.NewFloat(2))
	power := new(big.Float)
	power.Exp(base, exponent, nil)
	
	// 乘以2^96
	two96 := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(2), big.NewInt(96), nil))
	result := new(big.Float).Mul(power, two96)
	
	// 转换为big.Int
	resultInt := new(big.Int)
	result.Int(resultInt)
	
	return resultInt
}

func (c *UniswapV3Calculator) calculateLiquidityForAmount0(amount0, sqrtPrice, sqrtPriceUpper *big.Int) *big.Int {
	// L = amount0 * (sqrtPrice * sqrtPriceUpper) / (sqrtPriceUpper - sqrtPrice)
	
	numerator := new(big.Int).Mul(amount0, new(big.Int).Mul(sqrtPrice, sqrtPriceUpper))
	denominator := new(big.Int).Sub(sqrtPriceUpper, sqrtPrice)
	
	if denominator.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0)
	}
	
	return new(big.Int).Div(numerator, denominator)
}

func (c *UniswapV3Calculator) calculateLiquidityForAmount1(amount1, sqrtPrice, sqrtPriceLower *big.Int) *big.Int {
	// L = amount1 / (sqrtPrice - sqrtPriceLower)
	
	denominator := new(big.Int).Sub(sqrtPrice, sqrtPriceLower)
	if denominator.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0)
	}
	
	return new(big.Int).Div(amount1, denominator)
}

func (c *UniswapV3Calculator) calculateLiquidityFromAmounts(amount0, amount1, sqrtPrice *big.Int) *big.Int {
	// 简化计算：在当前价格下，流动性 ≈ √(amount0 * amount1)
	
	product := new(big.Int).Mul(amount0, amount1)
	if product.Cmp(big.NewInt(0)) == 0 {
		return big.NewInt(0)
	}
	
	// 计算平方根
	liquidity := new(big.Int).Sqrt(product)
	
	return liquidity
}

// PoolState 池子状态
type PoolState struct {
	SqrtPriceX96 *big.Int
	Tick         int24
	Liquidity    *big.Int
	Token0       common.Address
	Token1       common.Address
	Fee          *big.Int
	Price        *big.Float
}

// 示例：Uniswap V3 USDC/ETH池
func ExampleUniswapV3() {
	ctx := context.Background()
	
	// Uniswap V3 USDC/ETH 0.05%池
	poolAddress := "0x8ad599c3A0ff1De082011EFDDc58f1908eb6e6D8"
	
	calculator, err := NewUniswapV3Calculator(
		"https://eth-mainnet.g.alchemy.com/v2/demo",
		poolAddress,
	)
	if err != nil {
		fmt.Printf("Failed to create calculator: %v\n", err)
		return
	}
	
	// 获取池子状态
	poolState, err := calculator.GetPoolState(ctx)
	if err != nil {
		fmt.Printf("Failed to get pool state: %v\n", err)
		return
	}
	
	fmt.Printf("Uniswap V3 Pool State:\n")
	fmt.Printf("  Token0: %s\n", poolState.Token0.Hex())
	fmt.Printf("  Token1: %s\n", poolState.Token1.Hex())
	fmt.Printf("  Current Tick: %d\n", poolState.Tick)
	fmt.Printf("  Liquidity: %s\n", poolState.Liquidity.String())
	fmt.Printf("  Fee: %s bps\n", poolState.Fee.String())
	fmt.Printf("  Price (ETH/USDC): %s\n", poolState.Price.String())
	
	// 计算添加流动性
	// 假设添加：10,000 USDC 和对应价值的ETH
	usdcAmount := new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e6)) // 10,000 USDC (6 decimals)
	
	// 计算对应的ETH数量
	ethAmount := c.calculateEquivalentETH(usdcAmount, poolState.Price)
	
	fmt.Printf("\n添加流动性计算:\n")
	fmt.Printf("  USDC Amount: %s (%.2f USDC)\n", usdcAmount.String(), float64(usdcAmount.Int64())/1e6)
	fmt.Printf("  ETH Amount: %s (%.6f ETH)\n", ethAmount.String(), float64(ethAmount.Int64())/1e18)
	
	// 计算LP代币（流动性份额）
	// 假设tick范围：当前tick ± 100
	tickLower := poolState.Tick - 100
	tickUpper := poolState.Tick + 100
	
	lpTokens, err := calculator.CalculateLPTokens(ctx, usdcAmount, ethAmount, tickLower, tickUpper)
	if err != nil {
		fmt.Printf("Failed to calculate LP tokens: %v\n", err)
		return
	}
	
	fmt.Printf("  LP Tokens (Liquidity): %s\n", lpTokens.String())
	
	// 计算兑换率（流动性份额比例）
	exchangeRate, err := calculator.CalculateExchangeRate(ctx, usdcAmount, ethAmount)
	if err != nil {
		fmt.Printf("Failed to calculate exchange rate: %v\n", err)
		return
	}
	
	fmt.Printf("  Exchange Rate (Share of Pool): %s\n", exchangeRate.String())
	
	// 解释
	fmt.Println("\n解释:")
	fmt.Println("1. Uniswap V3使用集中流动性")
	fmt.Println("2. LP代币代表特定价格区间的流动性份额")
	fmt.Println("3. 兑换率 = 添加的流动性 / 总流动性")
	fmt.Println("4. 流动性提供者按份额收取交易费用")
}

func (c *UniswapV3Calculator) calculateEquivalentETH(usdcAmount *big.Int, price *big.Float) *big.Int {
	// USDC数量转换为浮点数
	usdcFloat := new(big.Float).SetInt(usdcAmount)
	
	// 除以价格得到ETH数量
	ethFloat := new(big.Float).Quo(usdcFloat, price)
	
	// 转换为wei (18 decimals)
	ethFloat = new(big.Float).Mul(ethFloat, big.NewFloat(1e18))
	
	// 转换为big.Int
	ethInt := new(big.Int)
	ethFloat.Int(ethInt)
	
	return ethInt
}

// Uniswap V3原理说明
func ExplainUniswapV3() {
	fmt.Println("=== Uniswap V3汇率计算原理 ===")
	fmt.Println()
	fmt.Println("1. 核心概念:")
	fmt.Println("   - 集中流动性: 流动性集中在特定价格区间")
	fmt.Println("   - Tick: 价格离散化单位，每个tick对应特定价格")
	fmt.Println("   - 流动性(L):