package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// RateCache 汇率缓存接口
type RateCache interface {
	// Get 获取缓存值
	Get(key string) (interface{}, bool)
	
	// Set 设置缓存值
	Set(key string, value interface{}, ttl time.Duration)
	
	// Delete 删除缓存值
	Delete(key string)
	
	// Clear 清空缓存
	Clear()
	
	// HealthCheck 健康检查
	HealthCheck() map[string]interface{}
}

// MemoryCache 内存缓存实现
type MemoryCache struct {
	items map[string]cacheItem
	mu    sync.RWMutex
	size  int
	ttl   time.Duration
}

type cacheItem struct {
	value      interface{}
	expiration time.Time
}

// NewMemoryCache 创建新的内存缓存
func NewMemoryCache(size int, ttl time.Duration) *MemoryCache {
	return &MemoryCache{
		items: make(map[string]cacheItem),
		size:  size,
		ttl:   ttl,
	}
}

func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	item, found := c.items[key]
	if !found {
		return nil, false
	}
	
	// 检查是否过期
	if time.Now().After(item.expiration) {
		c.mu.RUnlock()
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		c.mu.RLock()
		return nil, false
	}
	
	return item.value, true
}

func (c *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// 如果达到最大大小，删除最旧的项目
	if len(c.items) >= c.size {
		c.evictOldest()
	}
	
	expiration := time.Now().Add(ttl)
	c.items[key] = cacheItem{
		value:      value,
		expiration: expiration,
	}
}

func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.items, key)
}

func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]cacheItem)
}

func (c *MemoryCache) HealthCheck() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return map[string]interface{}{
		"type":        "memory",
		"size":        len(c.items),
		"capacity":    c.size,
		"default_ttl": c.ttl.String(),
		"status":      "healthy",
	}
}

func (c *MemoryCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time
	
	for key, item := range c.items {
		if oldestKey == "" || item.expiration.Before(oldestTime) {
			oldestKey = key
			oldestTime = item.expiration
		}
	}
	
	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

// RedisCache Redis缓存实现
type RedisCache struct {
	client *redis.Client
	ctx    context.Context
	prefix string
}

// NewRedisCache 创建新的Redis缓存
func NewRedisCache(url, password string, db int) (*RedisCache, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		// 如果不是URL格式，使用传统参数
		opt = &redis.Options{
			Addr:     url,
			Password: password,
			DB:       db,
		}
	}
	
	client := redis.NewClient(opt)
	ctx := context.Background()
	
	// 测试连接
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	
	return &RedisCache{
		client: client,
		ctx:    ctx,
		prefix: "exchange_rate:",
	}, nil
}

func (c *RedisCache) Get(key string) (interface{}, bool) {
	fullKey := c.prefix + key
	
	data, err := c.client.Get(c.ctx, fullKey).Result()
	if err == redis.Nil {
		return nil, false
	}
	if err != nil {
		return nil, false
	}
	
	// 尝试解析为JSON
	var value interface{}
	if err := json.Unmarshal([]byte(data), &value); err != nil {
		// 如果不是JSON，返回原始字符串
		return data, true
	}
	
	return value, true
}

func (c *RedisCache) Set(key string, value interface{}, ttl time.Duration) {
	fullKey := c.prefix + key
	
	// 序列化为JSON
	data, err := json.Marshal(value)
	if err != nil {
		// 如果无法序列化，转换为字符串
		data = []byte(fmt.Sprintf("%v", value))
	}
	
	c.client.Set(c.ctx, fullKey, data, ttl)
}

func (c *RedisCache) Delete(key string) {
	fullKey := c.prefix + key
	c.client.Del(c.ctx, fullKey)
}

func (c *RedisCache) Clear() {
	// 删除所有以prefix开头的键
	iter := c.client.Scan(c.ctx, 0, c.prefix+"*", 0).Iterator()
	for iter.Next(c.ctx) {
		c.client.Del(c.ctx, iter.Val())
	}
}

func (c *RedisCache) HealthCheck() map[string]interface{} {
	// 检查Redis连接
	if err := c.client.Ping(c.ctx).Err(); err != nil {
		return map[string]interface{}{
			"type":   "redis",
			"status": "unhealthy",
			"error":  err.Error(),
		}
	}
	
	// 获取Redis信息
	info, err := c.client.Info(c.ctx).Result()
	if err != nil {
		info = "unavailable"
	}
	
	return map[string]interface{}{
		"type":        "redis",
		"status":      "healthy",
		"connected":   true,
		"prefix":      c.prefix,
		"info":        info[:100], // 只取前100字符
	}
}

// MultiLevelCache 多级缓存（内存 + Redis）
type MultiLevelCache struct {
	memoryCache *MemoryCache
	redisCache  *RedisCache
	useRedis    bool
}

// NewMultiLevelCache 创建新的多级缓存
func NewMultiLevelCache(memorySize int, memoryTTL time.Duration, redisURL, redisPassword string, redisDB int) (*MultiLevelCache, error) {
	memoryCache := NewMemoryCache(memorySize, memoryTTL)
	
	var redisCache *RedisCache
	var useRedis bool
	
	if redisURL != "" {
		rc, err := NewRedisCache(redisURL, redisPassword, redisDB)
		if err == nil {
			redisCache = rc
			useRedis = true
		}
	}
	
	return &MultiLevelCache{
		memoryCache: memoryCache,
		redisCache:  redisCache,
		useRedis:    useRedis,
	}, nil
}

func (c *MultiLevelCache) Get(key string) (interface{}, bool) {
	// 首先尝试内存缓存
	if value, found := c.memoryCache.Get(key); found {
		return value, true
	}
	
	// 然后尝试Redis缓存
	if c.useRedis {
		if value, found := c.redisCache.Get(key); found {
			// 将Redis中的值存回内存缓存
			c.memoryCache.Set(key, value, c.memoryCache.ttl)
			return value, true
		}
	}
	
	return nil, false
}

func (c *MultiLevelCache) Set(key string, value interface{}, ttl time.Duration) {
	// 设置内存缓存
	c.memoryCache.Set(key, value, ttl)
	
	// 设置Redis缓存
	if c.useRedis {
		c.redisCache.Set(key, value, ttl)
	}
}

func (c *MultiLevelCache) Delete(key string) {
	c.memoryCache.Delete(key)
	if c.useRedis {
		c.redisCache.Delete(key)
	}
}

func (c *MultiLevelCache) Clear() {
	c.memoryCache.Clear()
	if c.useRedis {
		c.redisCache.Clear()
	}
}

func (c *MultiLevelCache) HealthCheck() map[string]interface{} {
	memoryHealth := c.memoryCache.HealthCheck()
	
	var redisHealth map[string]interface{}
	if c.useRedis {
		redisHealth = c.redisCache.HealthCheck()
	} else {
		redisHealth = map[string]interface{}{
			"type":   "redis",
			"status": "disabled",
		}
	}
	
	return map[string]interface{}{
		"type":   "multi_level",
		"status": "healthy",
		"memory": memoryHealth,
		"redis":  redisHealth,
	}
}