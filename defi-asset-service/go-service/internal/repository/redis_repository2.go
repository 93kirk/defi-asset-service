package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

// GetFromDelayedQueue 从延迟队列获取（续）
func (r *redisRepository) GetFromDelayedQueue(ctx context.Context, zset string, maxScore int64, count int64) ([]redis.Z, error) {
	fullZSet := r.buildKey("zset", zset)
	
	return r.client.ZRangeByScoreWithScores(ctx, fullZSet, &redis.ZRangeBy{
		Min:    "0",
		Max:    fmt.Sprintf("%d", maxScore),
		Offset: 0,
		Count:  count,
	}).Result()
}

// RemoveFromDelayedQueue 从延迟队列移除
func (r *redisRepository) RemoveFromDelayedQueue(ctx context.Context, zset string, values ...interface{}) error {
	fullZSet := r.buildKey("zset", zset)
	
	var members []interface{}
	for _, value := range values {
		valueBytes, err := json.Marshal(value)
		if err != nil {
			return err
		}
		members = append(members, valueBytes)
	}
	
	return r.client.ZRem(ctx, fullZSet, members...).Err()
}

// AcquireLock 获取分布式锁
func (r *redisRepository) AcquireLock(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	fullKey := r.buildKey("lock", key)
	
	result, err := r.client.SetNX(ctx, fullKey, value, ttl).Result()
	if err != nil {
		return false, err
	}
	
	return result, nil
}

// ReleaseLock 释放分布式锁
func (r *redisRepository) ReleaseLock(ctx context.Context, key string, value string) error {
	fullKey := r.buildKey("lock", key)
	
	// 使用Lua脚本确保原子性：只有锁的持有者才能释放锁
	luaScript := `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end
`
	
	script := redis.NewScript(luaScript)
	_, err := script.Run(ctx, r.client, []string{fullKey}, value).Result()
	return err
}

// RenewLock 续期分布式锁
func (r *redisRepository) RenewLock(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	fullKey := r.buildKey("lock", key)
	
	// 使用Lua脚本确保原子性：只有锁的持有者才能续期
	luaScript := `
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("expire", KEYS[1], ARGV[2])
else
    return 0
end
`
	
	script := redis.NewScript(luaScript)
	result, err := script.Run(ctx, r.client, []string{fullKey}, value, int(ttl.Seconds())).Result()
	if err != nil {
		return false, err
	}
	
	return result.(int64) == 1, nil
}

// IncrementRateLimit 增加限流计数
func (r *redisRepository) IncrementRateLimit(ctx context.Context, key string, window time.Duration) (int64, error) {
	fullKey := r.buildKey("rate_limit", key)
	
	// 使用Lua脚本实现滑动窗口限流
	luaScript := `
local current_time = redis.call('TIME')
local current_timestamp = tonumber(current_time[1]) * 1000 + math.floor(tonumber(current_time[2]) / 1000)
local window_ms = tonumber(ARGV[2])

-- 移除过期的时间戳
redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, current_timestamp - window_ms)

-- 添加当前时间戳
redis.call('ZADD', KEYS[1], current_timestamp, current_timestamp)

-- 设置过期时间
redis.call('EXPIRE', KEYS[1], math.ceil(window_ms / 1000))

-- 返回当前计数
return redis.call('ZCARD', KEYS[1])
`
	
	script := redis.NewScript(luaScript)
	result, err := script.Run(ctx, r.client, []string{fullKey}, key, int(window.Milliseconds())).Result()
	if err != nil {
		return 0, err
	}
	
	return result.(int64), nil
}

// GetRateLimit 获取限流计数
func (r *redisRepository) GetRateLimit(ctx context.Context, key string) (int64, error) {
	fullKey := r.buildKey("rate_limit", key)
	return r.client.ZCard(ctx, fullKey).Result()
}

// SetRequestTrace 设置请求追踪
func (r *redisRepository) SetRequestTrace(ctx context.Context, requestID string, data map[string]interface{}, ttl time.Duration) error {
	key := r.buildKey("request", requestID)
	
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	
	return r.client.Set(ctx, key, dataBytes, ttl).Err()
}

// GetRequestTrace 获取请求追踪
func (r *redisRepository) GetRequestTrace(ctx context.Context, requestID string) (map[string]interface{}, error) {
	key := r.buildKey("request", requestID)
	
	dataBytes, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	
	var data map[string]interface{}
	if err := json.Unmarshal(dataBytes, &data); err != nil {
		return nil, err
	}
	
	return data, nil
}

// UpdateCacheStats 更新缓存统计
func (r *redisRepository) UpdateCacheStats(ctx context.Context, cacheKey, cacheType, entityID string, hit bool, ttl int) error {
	statsKey := r.buildKey("cache_stats", cacheType, entityID)
	
	now := time.Now()
	expiresAt := now.Add(time.Duration(ttl) * time.Second)
	
	// 使用Hash存储缓存状态
	data := map[string]interface{}{
		"cache_key":      cacheKey,
		"cache_type":     cacheType,
		"entity_id":      entityID,
		"last_cached_at": now.Format(time.RFC3339),
		"expires_at":     expiresAt.Format(time.RFC3339),
		"ttl_seconds":    ttl,
	}
	
	// 更新命中/未命中计数
	field := "miss_count"
	if hit {
		field = "hit_count"
	}
	
	// 使用Pipeline批量操作
	pipe := r.client.Pipeline()
	
	// 设置缓存状态
	pipe.HSet(ctx, statsKey, data)
	
	// 增加计数
	pipe.HIncrBy(ctx, statsKey, field, 1)
	
	// 设置过期时间（稍长于缓存TTL）
	pipe.Expire(ctx, statsKey, time.Duration(ttl+3600)*time.Second)
	
	_, err := pipe.Exec(ctx)
	return err
}

// GetCacheStats 获取缓存统计
func (r *redisRepository) GetCacheStats(ctx context.Context, cacheType string, limit int) ([]map[string]interface{}, error) {
	pattern := r.buildKey("cache_stats", cacheType, "*")
	keys, err := r.GetKeysByPattern(ctx, pattern)
	if err != nil {
		return nil, err
	}
	
	if limit > 0 && len(keys) > limit {
		keys = keys[:limit]
	}
	
	var stats []map[string]interface{}
	
	for _, key := range keys {
		data, err := r.client.HGetAll(ctx, key).Result()
		if err != nil {
			continue
		}
		
		if len(data) > 0 {
			stats = append(stats, data)
		}
	}
	
	return stats, nil
}

// CleanupExpiredCache 清理过期缓存
func (r *redisRepository) CleanupExpiredCache(ctx context.Context) error {
	// 清理过期的缓存状态
	pattern := r.buildKey("cache_stats", "*", "*")
	keys, err := r.GetKeysByPattern(ctx, pattern)
	if err != nil {
		return err
	}
	
	for _, key := range keys {
		// 检查是否过期
		ttl, err := r.client.TTL(ctx, key).Result()
		if err != nil {
			continue
		}
		
		// 如果TTL为负（已过期），删除键
		if ttl < 0 {
			r.client.Del(ctx, key)
		}
	}
	
	return nil
}

// getCacheData 获取缓存数据（内部方法）
func (r *redisRepository) getCacheData(ctx context.Context, key string) (*model.CacheData, error) {
	dataBytes, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	
	var cacheData model.CacheData
	if err := json.Unmarshal(dataBytes, &cacheData); err != nil {
		return nil, err
	}
	
	// 检查是否过期
	if cacheData.ExpiresAt > 0 && time.Now().Unix() > cacheData.ExpiresAt {
		// 异步删除过期缓存
		go func() {
			r.client.Del(context.Background(), key)
		}()
		return nil, nil
	}
	
	return &cacheData, nil
}

// setCacheData 设置缓存数据（内部方法）
func (r *redisRepository) setCacheData(ctx context.Context, key string, data interface{}, ttl int) error {
	cacheData := model.CacheData{
		Data:      data,
		CachedAt:  time.Now().Unix(),
		TTL:       ttl,
		ExpiresAt: time.Now().Add(time.Duration(ttl) * time.Second).Unix(),
	}
	
	dataBytes, err := json.Marshal(cacheData)
	if err != nil {
		return err
	}
	
	return r.client.Set(ctx, key, dataBytes, time.Duration(ttl)*time.Second).Err()
}