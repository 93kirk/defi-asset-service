package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	
	"defi-asset-service/exchange-rate-system/examples"
	"github.com/ethereum/go-ethereum/common"
)

func main() {
	ctx := context.Background()
	
	fmt.Println("=== 测试Sablier流支付协议 ===")
	
	// 创建Sablier计算器
	calculator, err := examples.NewSablierExchangeCalculator("https://eth-mainnet.g.alchemy.com/v2/demo")
	if err != nil {
		log.Fatalf("创建计算器失败: %v", err)
	}
	
	// 测试1: 计算流支付价值
	streamID := big.NewInt(123456789)
	paidAmount, remainingAmount, err := calculator.CalculateStreamValue(ctx, streamID)
	if err != nil {
		log.Printf("计算流价值失败: %v", err)
	} else {
		fmt.Printf("1. 流支付 #%s 价值分析:\n", streamID.String())
		fmt.Printf("   已支付金额: %.6f USDC\n", paidAmount)
		fmt.Printf("   剩余金额: %.6f USDC\n", remainingAmount)
		fmt.Printf("   总金额: %.6f USDC\n", new(big.Float).Add(paidAmount, remainingAmount))
	}
	
	// 测试2: 计算流支付汇率
	exchangeRate, err := calculator.CalculateStreamExchangeRate(ctx, streamID)
	if err != nil {
		log.Printf("计算汇率失败: %v", err)
	} else {
		fmt.Printf("2. 流支付汇率:\n")
		fmt.Printf("   当前完成比例: %.2f%%\n", new(big.Float).Mul(exchangeRate, big.NewFloat(100)))
		fmt.Printf("   汇率解释: 0.0=未开始, 0.5=进行中, 1.0=已完成\n")
	}
	
	// 测试3: 计算流动性价值
	liquidityValue, err := calculator.CalculateStreamLiquidityValue(ctx, streamID, 0.15) // 15%折扣
	if err != nil {
		log.Printf("计算流动性价值失败: %v", err)
	} else {
		fmt.Printf("3. 流动性价值分析:\n")
		fmt.Printf("   剩余金额: %.6f USDC\n", remainingAmount)
		fmt.Printf("   流动性折扣: 15%%\n")
		fmt.Printf("   流动性价值: %.6f USDC\n", liquidityValue)
		fmt.Printf("   折扣金额: %.6f USDC\n", new(big.Float).Sub(remainingAmount, liquidityValue))
	}
	
	// 测试4: 模拟创建新流
	streamParams := examples.StreamParams{
		Sender:        common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e0F3B5F2b1F"),
		Recipient:     common.HexToAddress("0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984"),
		TokenAddress:  common.HexToAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"), // USDC
		TotalAmount:   big.NewInt(5000000000), // 5000 USDC (6 decimals)
		DurationHours: 720, // 30天
	}
	
	simulation, err := calculator.CreateStreamSimulation(ctx, streamParams)
	if err != nil {
		log.Printf("创建流模拟失败: %v", err)
	} else {
		fmt.Printf("4. 新流支付模拟:\n")
		fmt.Printf("   流ID: %s\n", simulation.StreamID.String())
		fmt.Printf("   发送方: %s\n", simulation.Sender.Hex())
		fmt.Printf("   接收方: %s\n", simulation.Recipient.Hex())
		fmt.Printf("   代币: USDC (%s)\n", simulation.TokenAddress.Hex())
		fmt.Printf("   总金额: %.2f USDC\n", new(big.Float).SetInt(simulation.TotalAmount))
		fmt.Printf("   持续时间: %d小时 (%.1f天)\n", streamParams.DurationHours, float64(streamParams.DurationHours)/24)
		fmt.Printf("   每秒支付: %.10f USDC\n", simulation.RatePerSecond)
		fmt.Printf("   每小时支付: %.6f USDC\n", new(big.Float).Mul(simulation.RatePerSecond, big.NewFloat(3600)))
		fmt.Printf("   每日支付: %.6f USDC\n", new(big.Float).Mul(simulation.RatePerSecond, big.NewFloat(86400)))
	}
	
	// 协议总结
	fmt.Println("\n=== Sablier协议总结 ===")
	fmt.Println("协议类型: 实时流支付协议")
	fmt.Println("TVL: $3.91B")
	fmt.Println("核心概念:")
	fmt.Println("  1. 流支付: 资金按秒实时流动，而非一次性支付")
	fmt.Println("  2. 时间加权价值: 流支付的价值随时间线性增加")
	fmt.Println("  3. 流动性: 未支付部分可折价转让（流动性折价）")
	fmt.Println("技术特点:")
	fmt.Println("  - 支持多种ERC20代币")
	fmt.Println("  - 可撤销流支付（发送方）")
	fmt.Println("  - 可转让流支付权益")
	fmt.Println("  - 实时余额查询")
	fmt.Println("汇率计算原理:")
	fmt.Println("  流支付汇率 = 已支付金额 / 总金额")
	fmt.Println("  范围: 0.0 (未开始) → 1.0 (已完成)")
	fmt.Println("  反映流支付的完成进度")
	
	fmt.Println("\n✅ Sablier协议实现完成!")
}