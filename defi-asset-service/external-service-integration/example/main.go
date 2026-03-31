package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	
	"defi-asset-service/external-service-integration/client"
	"defi-asset-service/external-service-integration/circuit_breaker"
	"defi-asset-service/external-service-integration/rate_limit"
	"defi-asset-service/external-service-integration/retry"
)

func main() {
	ctx := context.Background()
	
	// 示例1: 服务A客户端使用
	exampleServiceAClient(ctx)
	
	// 示例2: 服务B客户端使用
	exampleServiceBClient(ctx)
	
	// 示例3: 独立组件使用
	exampleStandaloneComponents(ctx)
}

func exampleServiceAClient(ctx context.Context) {
	fmt.Println("=== 服务A客户端示例 ===")
	
	// 创建服务A配置
	config := client.DefaultServiceAConfig()
	config.BaseURL = "https://api.service-a.com/v1"
	config.APIKey = "your-api-key-here"
	
	// 创建服务A客户端
	serviceAClient, err := client.NewServiceAClient(config)
	if err != nil {
		log.Fatalf("创建服务A客户端失败: %v", err)
	}
	defer serviceAClient.Close()
	
	// 执行健康检查
	if err := serviceAClient.HealthCheck(ctx); err != nil {
		log.Printf("服务A健康检查失败: %v", err)
	} else {
		fmt.Println("服务A健康检查通过")
	}
	
	// 获取用户资产
	address := "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae"
	chainID := 1
	
	assets, err := serviceAClient.GetUserAssets(ctx, address, chainID)
	if err != nil {
		log.Printf("获取用户资产失败: %v", err)
		return
	}
	
	fmt.Printf("用户 %s 总资产: %s USD\n", address, assets.TotalValueUSD)
	fmt.Printf("资产数量: %d\n", len(assets.Assets))
	
	// 显示前3个资产
	for i, asset := range assets.Assets {
		if i >= 3 {
			break
		}
		fmt.Printf("  %s: %s (价值: %s USD)\n", 
			asset.TokenSymbol, asset.Balance, asset.ValueUSD)
	}
	
	// 获取统计信息
	stats := serviceAClient.GetStats()
	fmt.Printf("服务A客户端统计: %+v\n", stats)
}

func exampleServiceBClient(ctx context.Context) {
	fmt.Println("\n=== 服务B客户端示例 ===")
	
	// 初始化数据库连接
	dsn := "user:password@tcp(localhost:3306)/defi_asset_service?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("数据库连接失败: %v", err)
		return
	}
	
	// 初始化Redis连接
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	
	// 测试Redis连接
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Redis连接失败: %v", err)
		return
	}
	
	// 创建服务B配置
	config := client.DefaultServiceBConfig()
	config.BaseURL = "https://api.service-b.com/v1"
	config.APIKey = "your-api-key-here"
	config.CacheTTL = 5 * time.Minute // 缩短缓存时间用于测试
	
	// 创建服务B客户端
	serviceBClient, err := client.NewServiceBClient(config, db, redisClient)
	if err != nil {
		log.Fatalf("创建服务B客户端失败: %v", err)
	}
	defer serviceBClient.Close()
	
	// 执行健康检查
	if err := serviceBClient.HealthCheck(ctx); err != nil {
		log.Printf("服务B健康检查失败: %v", err)
	} else {
		fmt.Println("服务B健康检查通过")
	}
	
	// 获取用户仓位（带缓存）
	address := "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae"
	chainID := 1
	
	positions, err := serviceBClient.GetUserPositions(ctx, address, chainID, false)
	if err != nil {
		log.Printf("获取用户仓位失败: %v", err)
		return
	}
	
	fmt.Printf("用户 %s 总仓位价值: %s USD\n", address, positions.TotalValueUSD)
	fmt.Printf("仓位数量: %d\n", len(positions.Positions))
	fmt.Printf("是否来自缓存: %v\n", positions.Cached)
	
	// 显示前3个仓位
	for i, position := range positions.Positions {
		if i >= 3 {
			break
		}
		fmt.Printf("  %s - %s: %s (APY: %s%%)\n", 
			position.ProtocolID, position.TokenSymbol, position.Amount, position.APY)
	}
	
	// 强制刷新获取最新数据
	fmt.Println("\n强制刷新获取最新数据...")
	positions, err = serviceBClient.GetUserPositions(ctx, address, chainID, true)
	if err != nil {
		log.Printf("强制刷新获取仓位失败: %v", err)
		return
	}
	
	fmt.Printf("刷新后是否来自缓存: %v\n", positions.Cached)
}

func exampleStandaloneComponents(ctx context.Context) {
	fmt.Println("\n=== 独立组件使用示例 ===")
	
	// 1. 重试管理器示例
	fmt.Println("1. 重试管理器示例:")
	retryConfig := retry.DefaultRetryConfig()
	retryManager := retry.NewRetryManager(retryConfig)
	
	attempt := 0
	err := retryManager.Execute(ctx, func(ctx context.Context) error {
		attempt++
		fmt.Printf("  第 %d 次尝试\n", attempt)
		if attempt < 3 {
			return fmt.Errorf("模拟失败")
		}
		return nil
	})
	
	if err != nil {
		fmt.Printf("  重试失败: %v\n", err)
	} else {
		fmt.Println("  重试成功")
	}
	
	// 2. 限流器示例
	fmt.Println("\n2. 限流器示例:")
	rateLimitConfig := rate_limit.DefaultRateLimitConfig()
	rateLimitConfig.RequestsPerSecond = 2.0 // 限制为每秒2个请求
	limiter := rate_limit.NewTokenBucketRateLimiter(rateLimitConfig)
	
	for i := 1; i <= 5; i++ {
		if limiter.Allow() {
			fmt.Printf("  请求 %d: 允许\n", i)
		} else {
			fmt.Printf("  请求 %d: 拒绝\n", i)
		}
		time.Sleep(200 * time.Millisecond)
	}
	
	// 3. 熔断器示例
	fmt.Println("\n3. 熔断器示例:")
	circuitBreakerConfig := circuit_breaker.DefaultCircuitBreakerConfig()
	circuitBreakerConfig.FailureThreshold = 2 // 降低阈值便于测试
	breaker := circuit_breaker.NewCircuitBreaker(circuitBreakerConfig)
	
	fmt.Printf("  初始状态: %s\n", breaker.GetState())
	
	// 模拟失败
	for i := 1; i <= 3; i++ {
		err := breaker.Execute(ctx, func() error {
			return fmt.Errorf("模拟服务失败")
		})
		fmt.Printf("  执行 %d: 状态=%s, 错误=%v\n", 
			i, breaker.GetState(), err)
		time.Sleep(100 * time.Millisecond)
	}
	
	// 等待熔断器进入半开状态
	fmt.Println("  等待熔断器恢复...")
	time.Sleep(35 * time.Second)
	fmt.Printf("  当前状态: %s\n", breaker.GetState())
	
	// 模拟成功
	err = breaker.Execute(ctx, func() error {
		return nil
	})
	fmt.Printf("  恢复执行: 状态=%s, 错误=%v\n", breaker.GetState(), err)
	
	// 获取统计信息
	stats := breaker.GetStats()
	fmt.Printf("  熔断器统计: 总请求=%d, 失败=%d, 失败率=%.2f%%\n",
		stats.TotalRequests, stats.FailedRequests, stats.FailureRate)
}

// 批量处理示例
func exampleBatchProcessing(ctx context.Context) {
	fmt.Println("\n=== 批量处理示例 ===")
	
	// 创建服务A配置
	config := client.DefaultServiceAConfig()
	config.BaseURL = "https://api.service-a.com/v1"
	
	serviceAClient, err := client.NewServiceAClient(config)
	if err != nil {
		log.Fatalf("创建服务A客户端失败: %v", err)
	}
	defer serviceAClient.Close()
	
	// 批量查询多个用户
	addresses := []string{
		"0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
		"0x742d35Cc6634C0532925a3b844Bc9e90F1A904Af",
		"0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ag",
	}
	chainID := 1
	
	results, err := serviceAClient.BatchGetUserAssets(ctx, addresses, chainID)
	if err != nil {
		log.Printf("批量查询失败: %v", err)
		return
	}
	
	fmt.Printf("批量查询完成，成功获取 %d 个用户数据\n", len(results))
	for address, response := range results {
		fmt.Printf("  %s: %s USD (%d 个资产)\n", 
			address, response.TotalValueUSD, len(response.Assets))
	}
}

// 错误处理示例
func exampleErrorHandling(ctx context.Context) {
	fmt.Println("\n=== 错误处理示例 ===")
	
	// 创建配置（使用无效的URL）
	config := client.DefaultServiceAConfig()
	config.BaseURL = "https://invalid-service-url.com"
	config.Timeout = 2 * time.Second
	config.RetryConfig.MaxAttempts = 2
	
	serviceAClient, err := client.NewServiceAClient(config)
	if err != nil {
		log.Fatalf("创建客户端失败: %v", err)
	}
	defer serviceAClient.Close()
	
	// 尝试查询（应该会失败）
	address := "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae"
	_, err = serviceAClient.GetUserAssets(ctx, address, 1)
	
	if err != nil {
		fmt.Printf("查询失败（预期中）: %v\n", err)
		
		// 检查错误类型
		var retryableErr *retry.RetryableError
		var nonRetryableErr *retry.NonRetryableError
		
		if errors.As(err, &retryableErr) {
			fmt.Println("  错误类型: 可重试错误")
		} else if errors.As(err, &nonRetryableErr) {
			fmt.Println("  错误类型: 不可重试错误")
		} else {
			fmt.Println("  错误类型: 其他错误")
		}
		
		// 获取统计信息
		stats := serviceAClient.GetStats()
		circuitBreakerStats := stats["circuit_breaker"].(map[string]interface{})
		fmt.Printf("  熔断器状态: %s\n", circuitBreakerStats["state"])
		fmt.Printf("  失败率: %s\n", circuitBreakerStats["failure_rate"])
	}
}

// errors包导入
import (
	"errors"
)