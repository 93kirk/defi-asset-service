package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"defi-asset-service/internal/model"

	"github.com/go-redis/redis/v8"
)

// RedisRepository Redis仓库接口
type RedisRepository interface {
	// 键操作
	Exists(ctx context.Context, key string) (bool, error)
	Delete(ctx context.Context, key string) error
	DeleteByPattern(ctx context.Context, pattern string) error
	GetKeysByPattern(ctx context.Context, pattern string) ([]string, error)
	
	// 用户仓位缓存
	GetPositionCache(ctx context.Context, userAddress, protocolID string) (*model.CacheData, error)
	SetPositionCache(ctx context.Context, userAddress, protocolID string, data interface{}, ttl int) error
	DeletePositionCache(ctx context.Context, userAddress, protocolID string) error
	GetUserPositionsCache(ctx context.Context, userAddress string) (*model.CacheData, error)
	SetUserPositionsCache(ctx context.Context, userAddress string, data interface{}, ttl int) error
	
	// 协议缓存
	GetProtocolCache(ctx context.Context, protocolID string) (*model.CacheData, error)
	SetProtocolCache(ctx context.Context, protocolID string, data interface{}, ttl int, version int) error
	DeleteProtocolCache(ctx context.Context, protocolID string) error
	GetProtocolListCache(ctx context.Context, category string, page int) (*model.CacheData, error)
	SetProtocolListCache(ctx context.Context, category string, page int, data interface{}, ttl int) error
	
	// 价格缓存
	GetPriceCache(ctx context.Context, tokenAddress string) (*model.PriceData, error)
	SetPriceCache(ctx context.Context, tokenAddress string, price float64, source string, ttl int) error
	BatchGetPriceCache(ctx context.Context, tokenAddresses []string) (map[string]*model.PriceData, error)
	BatchSetPriceCache(ctx context.Context, prices map[string]*model.PriceData) error
	
	// APY缓存
	GetApyCache(ctx context.Context, protocolID, tokenAddress string) (*model.ApyData, error)
	SetApyCache(ctx context.Context, protocolID, tokenAddress string, supplyApy, borrowApy float64, ttl int) error
	
	// 空值缓存（防缓存穿透）
	GetEmptyCache(ctx context.Context, cacheKey string) (bool, error)
	SetEmptyCache(ctx context.Context, cacheKey string, ttl int) error
	
	// 布隆过滤器
	BloomAdd(ctx context.Context, key string, value string) error
	BloomExists(ctx context.Context, key string, value string) (bool, error)
	
	// 队列操作
	AddToStream(ctx context.Context, stream string, values map[string]interface{}) (string, error)
	ReadFromStream(ctx context.Context, stream, group, consumer string, count int, block time.Duration) ([]redis.XStream, error)
	AckMessage(ctx context.Context, stream, group string, ids ...string) error
	AddToDelayedQueue(ctx context.Context, zset string, score int64, value interface{}) error
	GetFromDelayedQueue(ctx context.Context, zset string, maxScore int64, count int64) ([]redis.Z, error)
	RemoveFromDelayedQueue(ctx context.Context, zset string, values ...interface{}) error
	
	// 分布式锁
	AcquireLock(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key string, value string) error
	RenewLock(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	
	// 限流器
	IncrementRateLimit(ctx context.Context, key string, window time.Duration) (int64, error)
	GetRateLimit(ctx context.Context, key string) (int64, error)
	
	// 请求追踪
	SetRequestTrace(ctx context.Context, requestID string, data map[string]interface{}, ttl time.Duration) error
	GetRequestTrace(ctx context.Context, requestID string) (map[string]interface{}, error)
	
	// 缓存状态
	UpdateCacheStats(ctx context.Context, cacheKey, cacheType, entityID string, hit bool, ttl int) error
	GetCacheStats(ctx context.Context, cacheType string, limit int) ([]map[string]interface{}, error)
	CleanupExpiredCache(ctx context.Context) error
}

// redisRepository Redis仓库实现
type redisRepository struct {
	client *redis.Client
	prefix string
}

// NewRedisRepository 创建Redis仓库实例
func NewRedisRepository(client *redis.Client, prefix string) RedisRepository {
	return &redisRepository{
		client: client,
		prefix: prefix,
	}
}

// buildKey 构建完整的Redis键
func (r *redisRepository) buildKey(parts ...string) string {
	key := r.prefix
	for _, part := range parts {
		if part != "" {
			key += ":" + part
		}
	}
	return key
}

// Exists 检查键是否存在
func (r *redisRepository) Exists(ctx context.Context, key string) (bool, error) {
	result, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

// Delete 删除键
func (r *redisRepository) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

// DeleteByPattern 按模式删除键
func (r *redisRepository) DeleteByPattern(ctx context.Context, pattern string) error {
	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
	var keys []string
	
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	
	if err := iter.Err(); err != nil {
		return err
	}
	
	if len(keys) > 0 {
		return r.client.Del(ctx, keys...).Err()
	}
	
	return nil
}

// GetKeysByPattern 按模式获取键
func (r *redisRepository) GetKeysByPattern(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	iter := r.client.Scan(ctx, 0, pattern, 0).Iterator()
	
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	
	return keys, iter.Err()
}

// GetPositionCache 获取仓位缓存
func (r *redisRepository) GetPositionCache(ctx context.Context, userAddress, protocolID string) (*model.CacheData, error) {
	key := r.buildKey("position", userAddress, protocolID)
	return r.getCacheData(ctx, key)
}

// SetPositionCache 设置仓位缓存
func (r *redisRepository) SetPositionCache(ctx context.Context, userAddress, protocolID string, data interface{}, ttl int) error {
	key := r.buildKey("position", userAddress, protocolID)
	return r.setCacheData(ctx, key, data, ttl)
}

// DeletePositionCache 删除仓位缓存
func (r *redisRepository) DeletePositionCache(ctx context.Context, userAddress, protocolID string) error {
	key := r.buildKey("position", userAddress, protocolID)
	return r.Delete(ctx, key)
}

// GetUserPositionsCache 获取用户所有仓位缓存
func (r *redisRepository) GetUserPositionsCache(ctx context.Context, userAddress string) (*model.CacheData, error) {
	key := r.buildKey("positions", userAddress)
	return r.getCacheData(ctx, key)
}

// SetUserPositionsCache 设置用户所有仓位缓存
func (r *redisRepository) SetUserPositionsCache(ctx context.Context, userAddress string, data interface{}, ttl int) error {
	key := r.buildKey("positions", userAddress)
	return r.setCacheData(ctx, key, data, ttl)
}

// GetProtocolCache 获取协议缓存
func (r *redisRepository) GetProtocolCache(ctx context.Context, protocolID string) (*model.CacheData, error) {
	key := r.buildKey("protocol", protocolID)
	return r.getCacheData(ctx, key)
}

// SetProtocolCache 设置协议缓存
func (r *redisRepository) SetProtocolCache(ctx context.Context, protocolID string, data interface{}, ttl int, version int) error {
	key := r.buildKey("protocol", protocolID)
	
	cacheData := model.CacheData{
		Data:      data,
		CachedAt:  time.Now().Unix(),
		TTL:       ttl,
		Version:   version,
		ExpiresAt: time.Now().Add(time.Duration(ttl) * time.Second).Unix(),
	}
	
	dataBytes, err := json.Marshal(cacheData)
	if err != nil {
		return err
	}
	
	return r.client.Set(ctx, key, dataBytes, time.Duration(ttl)*time.Second).Err()
}

// DeleteProtocolCache 删除协议缓存
func (r *redisRepository) DeleteProtocolCache(ctx context.Context, protocolID string) error {
	key := r.buildKey("protocol", protocolID)
	return r.Delete(ctx, key)
}

// GetProtocolListCache 获取协议列表缓存
func (r *redisRepository) GetProtocolListCache(ctx context.Context, category string, page int) (*model.CacheData, error) {
	key := r.buildKey("protocols", "list", category, fmt.Sprintf("%d", page))
	return r.getCacheData(ctx, key)
}

// SetProtocolListCache 设置协议列表缓存
func (r *redisRepository) SetProtocolListCache(ctx context.Context, category string, page int, data interface{}, ttl int) error {
	key := r.buildKey("protocols", "list", category, fmt.Sprintf("%d", page))
	return r.setCacheData(ctx, key, data, ttl)
}

// GetPriceCache 获取价格缓存
func (r *redisRepository) GetPriceCache(ctx context.Context, tokenAddress string) (*model.PriceData, error) {
	key := r.buildKey("price", tokenAddress)
	
	data, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	
	if len(data) == 0 {
		return nil, nil
	}
	
	priceData := &model.PriceData{}
	if priceStr, ok := data["price"]; ok {
		fmt.Sscanf(priceStr, "%f", &priceData.Price)
	}
	
	if source, ok := data["source"]; ok {
		priceData.Source = source
	}
	
	if updatedAtStr, ok := data["updated_at"]; ok {
		fmt.Sscanf(updatedAtStr, "%d", &priceData.UpdatedAt)
	}
	
	if ttlStr, ok := data["ttl"]; ok {
		fmt.Sscanf(ttlStr, "%d", &priceData.TTL)
	}
	
	return priceData, nil
}

// SetPriceCache 设置价格缓存
func (r *redisRepository) SetPriceCache(ctx context.Context, tokenAddress string, price float64, source string, ttl int) error {
	key := r.buildKey("price", tokenAddress)
	
	data := map[string]interface{}{
		"price":       fmt.Sprintf("%f", price),
		"source":      source,
		"updated_at":  fmt.Sprintf("%d", time.Now().Unix()),
		"ttl":         fmt.Sprintf("%d", ttl),
	}
	
	err := r.client.HSet(ctx, key, data).Err()
	if err != nil {
		return err
	}
	
	return r.client.Expire(ctx, key, time.Duration(ttl)*time.Second).Err()
}

// BatchGetPriceCache 批量获取价格缓存
func (r *redisRepository) BatchGetPriceCache(ctx context.Context, tokenAddresses []string) (map[string]*model.PriceData, error) {
	result := make(map[string]*model.PriceData)
	
	for _, tokenAddress := range tokenAddresses {
		priceData, err := r.GetPriceCache(ctx, tokenAddress)
		if err != nil {
			return nil, err
		}
		if priceData != nil {
			result[tokenAddress] = priceData
		}
	}
	
	return result, nil
}

// BatchSetPriceCache 批量设置价格缓存
func (r *redisRepository) BatchSetPriceCache(ctx context.Context, prices map[string]*model.PriceData) error {
	for tokenAddress, priceData := range prices {
		err := r.SetPriceCache(ctx, tokenAddress, priceData.Price, priceData.Source, priceData.TTL)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetApyCache 获取APY缓存
func (r *redisRepository) GetApyCache(ctx context.Context, protocolID, tokenAddress string) (*model.ApyData, error) {
	key := r.buildKey("apy", protocolID, tokenAddress)
	
	data, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	
	if len(data) == 0 {
		return nil, nil
	}
	
	apyData := &model.ApyData{}
	if supplyApyStr, ok := data["supply_apy"]; ok {
		fmt.Sscanf(supplyApyStr, "%f", &apyData.SupplyApy)
	}
	
	if borrowApyStr, ok := data["borrow_apy"]; ok {
		fmt.Sscanf(borrowApyStr, "%f", &apyData.BorrowApy)
	}
	
	if updatedAtStr, ok := data["updated_at"]; ok {
		fmt.Sscanf(updatedAtStr, "%d", &apyData.UpdatedAt)
	}
	
	if ttlStr, ok := data["ttl"]; ok {
		fmt.Sscanf(ttlStr, "%d", &apyData.TTL)
	}
	
	return apyData, nil
}

// SetApyCache 设置APY缓存
func (r *redisRepository) SetApyCache(ctx context.Context, protocolID, tokenAddress string, supplyApy, borrowApy float64, ttl int) error {
	key := r.buildKey("apy", protocolID, tokenAddress)
	
	data := map[string]interface{}{
		"supply_apy":  fmt.Sprintf("%f", supplyApy),
		"borrow_apy":  fmt.Sprintf("%f", borrowApy),
		"updated_at":  fmt.Sprintf("%d", time.Now().Unix()),
		"ttl":         fmt.Sprintf("%d", ttl),
	}
	
	err := r.client.HSet(ctx, key, data).Err()
	if err != nil {
		return err
	}
	
	return r.client.Expire(ctx, key, time.Duration(ttl)*time.Second).Err()
}

// GetEmptyCache 获取空值缓存
func (r *redisRepository) GetEmptyCache(ctx context.Context, cacheKey string) (bool, error) {
	key := r.buildKey("empty", cacheKey)
	exists, err := r.Exists(ctx, key)
	return exists, err
}

// SetEmptyCache 设置空值缓存
func (r *redisRepository) SetEmptyCache(ctx context.Context, cacheKey string, ttl int) error {
	key := r.buildKey("empty", cacheKey)
	return r.client.Set(ctx, key, "1", time.Duration(ttl)*time.Second).Err()
}

// BloomAdd 添加到布隆过滤器
func (r *redisRepository) BloomAdd(ctx context.Context, key string, value string) error {
	fullKey := r.buildKey("bloom", key)
	return r.client.Do(ctx, "BF.ADD", fullKey, value).Err()
}

// BloomExists 检查布隆过滤器
func (r *redisRepository) BloomExists(ctx context.Context, key string, value string) (bool, error) {
	fullKey := r.buildKey("bloom", key)
	result, err := r.client.Do(ctx, "BF.EXISTS", fullKey, value).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

// AddToStream 添加到Stream
func (r *redisRepository) AddToStream(ctx context.Context, stream string, values map[string]interface{}) (string, error) {
	fullStream := r.buildKey("stream", stream)
	return r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: fullStream,
		Values: values,
	}).Result()
}

// ReadFromStream 从Stream读取
func (r *redisRepository) ReadFromStream(ctx context.Context, stream, group, consumer string, count int, block time.Duration) ([]redis.XStream, error) {
	fullStream := r.buildKey("stream", stream)
	return r.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{fullStream, ">"},
		Count:    int64(count),
		Block:    block,
	}).Result()
}

// AckMessage 确认消息
func (r *redisRepository) AckMessage(ctx context.Context, stream, group string, ids ...string) error {
	fullStream := r.buildKey("stream", stream)
	return r.client.XAck(ctx, fullStream, group, ids...).Err()
}

// AddToDelayedQueue 添加到延迟队列
func (r *redisRepository) AddToDelayedQueue(ctx context.Context, zset string, score int64, value interface{}) error {
	fullZSet := r.buildKey("zset", zset)
	
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return err
	}
	
	return r.client.ZAdd(ctx, fullZSet, &redis.Z{
		Score:  float64(score),
		Member: valueBytes,
	}).Err()
}

// GetFromDelayedQueue 从延迟队列获取
func (r *redisRepository) GetFromDelayedQueue(ctx context.Context, zset string, maxScore int64, count int64) ([]redis.Z, error) {
	fullZSet := r.build