package adapter

import (
	"context"
	"sync"
	"time"

	"defi-asset-service/exchange-rate-system/internal/models"
)

// AdapterRegistry 适配器注册表
type AdapterRegistry struct {
	adapters map[string]ProtocolAdapter
	mu       sync.RWMutex
}

// NewAdapterRegistry 创建新的适配器注册表
func NewAdapterRegistry() *AdapterRegistry {
	return &AdapterRegistry{
		adapters: make(map[string]ProtocolAdapter),
	}
}

// Register 注册适配器
func (r *AdapterRegistry) Register(protocolID string, adapter ProtocolAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adapters[protocolID] = adapter
}

// GetAdapter 获取适配器
func (r *AdapterRegistry) GetAdapter(protocolID string) (ProtocolAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	adapter, exists := r.adapters[protocolID]
	if exists {
		return adapter, true
	}
	
	// 尝试通过协议ID模式匹配
	for id, adapter := range r.adapters {
		if isProtocolMatch(protocolID, id) {
			return adapter, true
		}
	}
	
	return nil, false
}

// GetAllAdapters 获取所有适配器
func (r *AdapterRegistry) GetAllAdapters() map[string]ProtocolAdapter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	result := make(map[string]ProtocolAdapter)
	for k, v := range r.adapters {
		result[k] = v
	}
	return result
}

// RemoveAdapter 移除适配器
func (r *AdapterRegistry) RemoveAdapter(protocolID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.adapters, protocolID)
}

// HealthCheck 健康检查所有适配器
func (r *AdapterRegistry) HealthCheck(ctx context.Context) map[string]error {
	r.mu.RLock()
	adapters := make(map[string]ProtocolAdapter)
	for k, v := range r.adapters {
		adapters[k] = v
	}
	r.mu.RUnlock()
	
	results := make(map[string]error)
	for protocolID, adapter := range adapters {
		if err := adapter.HealthCheck(ctx); err != nil {
			results[protocolID] = err
		}
	}
	
	return results
}

// isProtocolMatch 检查协议ID是否匹配
func isProtocolMatch(requestID, adapterID string) bool {
	// 简单匹配逻辑，可以根据需要扩展
	if requestID == adapterID {
		return true
	}
	
	// 处理带链前缀的协议ID
	// 例如: eth_lido 匹配 lido
	if len(requestID) > 4 && requestID[3] == '_' {
		chainPrefix := requestID[:3]
		baseID := requestID[4:]
		if baseID == adapterID {
			return true
		}
	}
	
	return false
}

// AdapterFactory 适配器工厂
type AdapterFactory struct {
	registry *AdapterRegistry
}

// NewAdapterFactory 创建适配器工厂
func NewAdapterFactory() *AdapterFactory {
	return &AdapterFactory{
		registry: NewAdapterRegistry(),
	}
}

// CreateAdapter 创建适配器
func (f *AdapterFactory) CreateAdapter(protocolID string, protocolData map[string]interface{}) (ProtocolAdapter, error) {
	// 从协议数据中提取信息
	name, _ := protocolData["name"].(string)
	chain, _ := protocolData["chain"].(string)
	poolStats, _ := protocolData["pool_stats"].([]interface{})
	
	// 确定协议类型
	category := models.GetCategoryFromProtocolID(protocolID)
	
	baseAdapter := BaseAdapter{
		ProtocolID:   protocolID,
		ProtocolName: name,
		ProtocolType: category,
		Chain:        chain,
		LastUpdate:   time.Now(),
	}
	
	var adapter ProtocolAdapter
	
	switch category {
	case models.CategoryLiquidStaking:
		adapter = &LiquidStakingAdapter{
			BaseAdapter: baseAdapter,
			ReceiptToken:    getReceiptToken(protocolID),
			UnderlyingToken: getUnderlyingToken(protocolID, chain),
		}
		
	case models.CategoryLending:
		adapter = &LendingAdapter{
			BaseAdapter: baseAdapter,
			LendingPoolAddress: getLendingPoolAddress(protocolID, chain),
			InterestRateModel:  "variable",
		}
		
	case models.CategoryAMM:
		adapter = &AMMAdapter{
			BaseAdapter: baseAdapter,
			PoolAddress: getPoolAddress(protocolID, chain),
			PoolType:    getPoolType(protocolID),
		}
		
	case models.CategoryYieldAggregator:
		adapter = &YieldAggregatorAdapter{
			BaseAdapter: baseAdapter,
			VaultAddress: getVaultAddress(protocolID, chain),
			Strategy:     getStrategy(protocolID),
		}
		
	case models.CategoryLSDRewards, models.CategoryRestaking:
		adapter = &LSDRewardsAdapter{
			BaseAdapter: baseAdapter,
			StakingLayer: getStakingLayer(protocolID),
			RewardsLayer: getRewardsLayer(protocolID),
			LSDToken:     getLSDToken(protocolID),
		}
		
	default:
		// 通用适配器
		adapter = &GenericAdapter{
			BaseAdapter: baseAdapter,
			RateCalculationFunc: createGenericRateFunction(protocolID, poolStats),
		}
	}
	
	// 注册适配器
	f.registry.Register(protocolID, adapter)
	
	return adapter, nil
}

// CreateAdaptersFromDebankData 从DeBank数据创建适配器
func (f *AdapterFactory) CreateAdaptersFromDebankData(protocols []map[string]interface{}) (map[string]ProtocolAdapter, error) {
	createdAdapters := make(map[string]ProtocolAdapter)
	
	for _, protocol := range protocols {
		protocolID, ok := protocol["id"].(string)
		if !ok || protocolID == "" {
			continue
		}
		
		adapter, err := f.CreateAdapter(protocolID, protocol)
		if err != nil {
			// 记录错误但继续处理其他协议
			continue
		}
		
		createdAdapters[protocolID] = adapter
	}
	
	return createdAdapters, nil
}

// GetRegistry 获取注册表
func (f *AdapterFactory) GetRegistry() *AdapterRegistry {
	return f.registry
}

// 辅助函数
func getReceiptToken(protocolID string) string {
	switch protocolID {
	case "lido":
		return "stETH"
	case "rocketpool":
		return "rETH"
	case "bsc_bnbchain":
		return "stBNB"
	default:
		return "receipt_token"
	}
}

func getUnderlyingToken(protocolID, chain string) string {
	switch chain {
	case "eth":
		return "ETH"
	case "bsc":
		return "BNB"
	case "matic":
		return "MATIC"
	case "arb":
		return "ETH"
	default:
		return "underlying_token"
	}
}

func getLendingPoolAddress(protocolID, chain string) string {
	// 这里应该从配置或数据库中获取
	return "0x" + protocolID[:40]
}

func getPoolAddress(protocolID, chain string) string {
	return "0x" + protocolID[:40]
}

func getPoolType(protocolID string) string {
	switch {
	case contains(protocolID, "uniswap_v3"):
		return "uniswap_v3"
	case contains(protocolID, "curve"):
		return "curve"
	case contains(protocolID, "balancer"):
		return "balancer"
	default:
		return "uniswap_v2"
	}
}

func getVaultAddress(protocolID, chain string) string {
	return "0x" + protocolID[:40]
}

func getStrategy(protocolID string) string {
	return "default_strategy"
}

func getStakingLayer(protocolID string) string {
	if contains(protocolID, "etherfi") {
		return "eigenlayer"
	}
	return "native_staking"
}

func getRewardsLayer(protocolID string) string {
	return "lsd_rewards"
}

func getLSDToken(protocolID string) string {
	switch {
	case contains(protocolID, "etherfi"):
		return "eETH"
	case contains(protocolID, "kelp"):
		return "rsETH"
	case contains(protocolID, "swell"):
		return "swETH"
	default:
		return "lsd_token"
	}
}

func createGenericRateFunction(protocolID string, poolStats []interface{}) func(ctx context.Context, request models.RateCalculationRequest) (float64, error) {
	return func(ctx context.Context, request models.RateCalculationRequest) (float64, error) {
		// 从pool_stats中提取rate字段
		for _, stat := range poolStats {
			if statMap, ok := stat.(map[string]interface{}); ok {
				if rate, ok := statMap["rate"].(float64); ok && rate > 0 {
					return rate, nil
				}
			}
		}
		
		// 默认返回1.0
		return 1.0, nil
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || 
		contains(s[1:], substr))))
}