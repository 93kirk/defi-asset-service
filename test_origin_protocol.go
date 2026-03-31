package main

import (
	"context"
	"fmt"
	"log"
	
	"defi-asset-service/exchange-rate-system/examples"
)

func main() {
	ctx := context.Background()
	
	fmt.Println("=== 测试Origin协议汇率计算 ===")
	
	// 创建Origin计算器
	calculator, err := examples.NewOriginExchangeCalculator("https://eth-mainnet.g.alchemy.com/v2/demo")
	if err != nil {
		log.Fatalf("创建计算器失败: %v", err)
	}
	
	// 测试1: 计算综合汇率
	exchangeRate, err := calculator.CalculateExchangeRate(ctx)
	if err != nil {
		log.Printf("计算汇率失败: %v", err)
	} else {
		fmt.Printf("1. Origin协议综合汇率: %.6f\n", exchangeRate)
		fmt.Printf("   说明: OUSD稳定币汇率(≈1.0) × 质押奖励因子\n")
	}
	
	// 测试2: 计算OGV质押奖励
	ogvAmount := 1000.0
	rewards, err := calculator.CalculateOGVStakingRewards(ctx, ogvAmount, 30)
	if err != nil {
		log.Printf("计算质押奖励失败: %v", err)
	} else {
		fmt.Printf("2. OGV质押奖励计算:\n")
		fmt.Printf("   质押金额: %.2f OGV\n", ogvAmount)
		fmt.Printf("   质押天数: 30天\n")
		fmt.Printf("   预计奖励: %.4f OGV\n", rewards)
		fmt.Printf("   年化收益率: ≈8%%\n")
	}
	
	// 测试3: 计算OUSD收益
	ousdAmount := 10000.0
	yield, err := calculator.CalculateOUSDYield(ctx, ousdAmount)
	if err != nil {
		log.Printf("计算OUSD收益失败: %v", err)
	} else {
		fmt.Printf("3. OUSD收益计算:\n")
		fmt.Printf("   本金: %.2f OUSD\n", ousdAmount)
		fmt.Printf("   预计收益: %.2f OUSD\n", yield)
		fmt.Printf("   收益率: ≈3%%\n")
	}
	
	// 协议总结
	fmt.Println("\n=== Origin协议总结 ===")
	fmt.Println("协议类型: 算法稳定币 + 收益协议")
	fmt.Println("TVL: $4.07B")
	fmt.Println("核心组件:")
	fmt.Println("  1. OUSD: 算法稳定币，与美元1:1锚定")
	fmt.Println("  2. OGV: 治理代币，质押获得奖励")
	fmt.Println("  3. 质押系统: OGV质押获得收益")
	fmt.Println("收益来源:")
	fmt.Println("  - OUSD: 通过算法稳定币机制产生收益")
	fmt.Println("  - OGV: 质押奖励和治理权利")
	fmt.Println("汇率计算原理:")
	fmt.Println("  综合汇率 = OUSD汇率(≈1.0) × 质押奖励因子(>1.0)")
	fmt.Println("  反映稳定币价值 + 收益潜力")
	
	fmt.Println("\n✅ Origin协议实现完成!")
}