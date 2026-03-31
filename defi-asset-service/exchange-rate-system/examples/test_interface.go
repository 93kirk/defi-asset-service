package examples

import (
	"context"
	"fmt"
	"math/big"
	"time"
)

// TestInterface 测试接口
type TestInterface struct {
	rpcURL string
}

// NewTestInterface 创建测试接口
func NewTestInterface(rpcURL string) *TestInterface {
	return &TestInterface{
		rpcURL: rpcURL,
	}
}

// TestLidoExchange 测试Lido汇率计算
func (t *TestInterface) TestLidoExchange() {
	fmt.Println("=== 测试 Lido (ETH → stETH) 汇率计算 ===")
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// 创建Lido计算器
	lidoCalc, err := NewLidoExchangeCalculator(t.rpcURL)
	if err != nil {
		fmt.Printf("创建Lido计算器失败: %v\n", err)
		return
	}
	
	// 计算兑换率
	exchangeRate, err := lidoCalc.CalculateExchangeRate(ctx)
	if err != nil {
		fmt.Printf("计算兑换率失败: %v\n", err)
		return
	}
	
	fmt.Printf("当前兑换率: 1 ETH = %s stETH\n", exchangeRate.String())
	
	// 计算10 ETH对应的stETH
	ethAmount := big.NewFloat(10.0)
	stETHAmount, err := lidoCalc.CalculateStETHAmount(ctx, ethAmount)
	if err != nil {
		fmt.Printf("计算stETH数量失败: %v\n", err)
		return
	}
	
	fmt.Printf("10 ETH = %s stETH\n", stETHAmount.String())
	
	// 解释
	fmt.Println("\n计算原理:")
	fmt.Println("1. 查询stETH合约的totalAssets()获取池子总ETH")
	fmt.Println("2. 查询stETH合约的totalSupply()获取stETH总供应量")
	fmt.Println("3. 兑换率 = totalAssets / totalSupply")
	fmt.Println("4. 兑换率 > 1.0 表示有质押奖励")
}

// TestAaveExchange 测试Aave汇率计算
func (t *TestInterface) TestAaveExchange() {
	fmt.Println("\n=== 测试 Aave V3 (USDC → aUSDC) 汇率计算 ===")
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Aave V3 USDC aToken地址
	aUSDCAddress := "0x98C23E9d8f34FEFb1B7BD6a91B7FF122F4e16F5c"
	
	aaveCalc, err := NewAaveExchangeCalculator(t.rpcURL, aUSDCAddress)
	if err != nil {
		fmt.Printf("创建Aave计算器失败: %v\n", err)
		return
	}
	
	// 方法1: exchangeRateStored
	exchangeRate, err := aaveCalc.CalculateExchangeRateStored(ctx)
	if err != nil {
		fmt.Printf("计算exchangeRateStored失败: %v\n", err)
	} else {
		fmt.Printf("exchangeRateStored: 1 USDC = %s aUSDC\n", exchangeRate.String())
	}
	
	// 方法2: 标准化收入
	normalizedIncome, err := aaveCalc.CalculateNormalizedIncome(ctx)
	if err != nil {
		fmt.Printf("计算标准化收入失败: %v\n", err)
	} else {
		fmt.Printf("标准化收入: %s\n", normalizedIncome.String())
	}
	
	// 计算10,000 USDC对应的aUSDC
	usdcAmount := big.NewFloat(10000.0)
	aTokenAmount, err := aaveCalc.CalculateATokenAmount(ctx, usdcAmount, 6)
	if err != nil {
		fmt.Printf("计算aToken数量失败: %v\n", err)
		return
	}
	
	fmt.Printf("10,000 USDC = %s aUSDC\n", aTokenAmount.String())
	
	// 解释
	fmt.Println("\n计算原理:")
	fmt.Println("1. Aave使用复利模型计算利息")
	fmt.Println("2. exchangeRateStored: 上一次交互时的汇率")
	fmt.Println("3. 标准化收入: 考虑区块高度的复利累积")
	fmt.Println("4. 实际汇率随时间增长（利息累积）")
}

// TestUniswapV3Exchange 测试Uniswap V3汇率计算
func (t *TestInterface) TestUniswapV3Exchange() {
	fmt.Println("\n=== 测试 Uniswap V3 (添加流动性) 汇率计算 ===")
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Uniswap V3 USDC/ETH 0.05%池
	poolAddress := "0x8ad599c3A0ff1De082011EFDDc58f1908eb6e6D8"
	
	uniCalc, err := NewUniswapV3Calculator(t.rpcURL, poolAddress)
	if err != nil {
		fmt.Printf("创建Uniswap计算器失败: %v\n", err)
		return
	}
	
	// 获取池子状态
	poolState, err := uniCalc.GetPoolState(ctx)
	if err != nil {
		fmt.Printf("获取池子状态失败: %v\n", err)
		return
	}
	
	fmt.Printf("池子状态:\n")
	fmt.Printf("  Token0: %s (USDC)\n", poolState.Token0.Hex())
	fmt.Printf("  Token1: %s (WETH)\n", poolState.Token1.Hex())
	fmt.Printf("  当前价格: 1 ETH = %s USDC\n", poolState.Price.String())
	
	// 计算添加流动性
	usdcAmount := new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e6)) // 10,000 USDC
	ethAmount := uniCalc.calculateEquivalentETH(usdcAmount, poolState.Price)
	
	fmt.Printf("\n添加流动性:\n")
	fmt.Printf("  USDC: %s (%.2f USDC)\n", usdcAmount.String(), float64(usdcAmount.Int64())/1e6)
	fmt.Printf("  ETH: %s (%.6f ETH)\n", ethAmount.String(), float64(ethAmount.Int64())/1e18)
	
	// 计算流动性份额
	tickLower := poolState.Tick - 100
	tickUpper := poolState.Tick + 100
	
	liquidity, err := uniCalc.CalculateLPTokens(ctx, usdcAmount, ethAmount, tickLower, tickUpper)
	if err != nil {
		fmt.Printf("计算流动性失败: %v\n", err)
		return
	}
	
	fmt.Printf("  获得的流动性: %s\n", liquidity.String())
	
	// 计算份额比例
	shareRatio, err := uniCalc.CalculateExchangeRate(ctx, usdcAmount, ethAmount)
	if err != nil {
		fmt.Printf("计算份额比例失败: %v\n", err)
		return
	}
	
	fmt.Printf("  池子份额: %s\n", shareRatio.String())
	
	// 解释
	fmt.Println("\n计算原理:")
	fmt.Println("1. Uniswap V3使用集中流动性模型")
	fmt.Println("2. 流动性提供者在特定价格区间提供流动性")
	fmt.Println("3. 流动性数量基于恒定乘积公式计算")
	fmt.Println("4. 份额比例 = 添加的流动性 / 总流动性")
}

// RunAllTests 运行所有测试
func (t *TestInterface) RunAllTests() {
	fmt.Println("开始运行所有汇率计算测试...")
	fmt.Println("=========================================")
	
	t.TestLidoExchange()
	t.TestAaveExchange()
	t.TestUniswapV3Exchange()
	
	fmt.Println("\n=========================================")
	fmt.Println("测试完成!")
}

// 实时汇率查询接口
func (t *TestInterface) QueryRealTimeRates() {
	fmt.Println("=== 实时汇率查询 ===")
	
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	// 1. Lido汇率
	lidoCalc, _ := NewLidoExchangeCalculator(t.rpcURL)
	if lidoCalc != nil {
		rate, err := lidoCalc.CalculateExchangeRate(ctx)
		if err == nil {
			fmt.Printf("Lido: 1 ETH = %s stETH\n", rate.String())
		}
	}
	
	// 2. Aave汇率
	aUSDCAddress := "0x98C23E9d8f34FEFb1B7BD6a91B7FF122F4e16F5c"
	aaveCalc, _ := NewAaveExchangeCalculator(t.rpcURL, aUSDCAddress)
	if aaveCalc != nil {
		rate, err := aaveCalc.CalculateExchangeRateStored(ctx)
		if err == nil {
			fmt.Printf("Aave USDC: 1 USDC = %s aUSDC\n", rate.String())
		}
	}
	
	// 3. Uniswap V3价格
	poolAddress := "0x8ad599c3A0ff1De082011EFDDc58f1908eb6e6D8"
	uniCalc, _ := NewUniswapV3Calculator(t.rpcURL, poolAddress)
	if uniCalc != nil {
		state, err := uniCalc.GetPoolState(ctx)
		if err == nil {
			fmt.Printf("Uniswap V3: 1 ETH = %s USDC\n", state.Price.String())
		}
	}
	
	fmt.Println("\n更新时间:", time.Now().Format("2006-01-02 15:04:05"))
}

// 主函数示例
func main() {
	// 使用公共RPC端点
	rpcURL := "https://eth-mainnet.g.alchemy.com/v2/demo"
	
	// 创建测试接口
	testInterface := NewTestInterface(rpcURL)
	
	// 运行所有测试
	testInterface.RunAllTests()
	
	fmt.Println("\n" + strings.Repeat("=", 50))
	
	// 查询实时汇率
	testInterface.QueryRealTimeRates()
	
	// 显示计算原理
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("汇率计算原理总结:")
	fmt.Println()
	fmt.Println("1. 流动性质押协议 (Lido):")
	fmt.Println("   兑换率 = 池子总资产 / 代币总供应量")
	fmt.Println("   反映质押奖励累积")
	fmt.Println()
	fmt.Println("2. 借贷协议 (Aave):")
	fmt.Println("   兑换率 = exchangeRateStored")
	fmt.Println("   基于复利模型: rate = (1 + supplyRate)^time")
	fmt.Println("   标准化收入考虑区块高度")
	fmt.Println()
	fmt.Println("3. AMM协议 (Uniswap V3):")
	fmt.Println("   流动性基于恒定乘积公式: x * y = k")
	fmt.Println("   集中流动性在特定价格区间")
	fmt.Println("   份额比例 = 添加流动性 / 总流动性")
	fmt.Println()
	fmt.Println("关键点:")
	fmt.Println("- 所有计算都基于链上数据")
	fmt.Println("- 汇率实时更新")
	fmt.Println("- 考虑协议特定的经济模型")
	fmt.Println("- 支持多数据源验证")
}

// 简单的HTTP测试服务器
func StartTestServer(port string) {
	// 这里可以启动一个HTTP服务器提供测试接口
	// 实际实现需要集成到主服务中
	fmt.Printf("测试服务器将在端口 %s 启动\n", port)
	fmt.Println("提供以下测试端点:")
	fmt.Println("  GET /test/lido-rate     - Lido汇率")
	fmt.Println("  GET /test/aave-rate     - Aave汇率")
	fmt.Println("  GET /test/uniswap-price - Uniswap价格")
	fmt.Println("  GET /test/all-rates     - 所有汇率")
	fmt.Println("  POST /test/calculate    - 自定义计算")
}