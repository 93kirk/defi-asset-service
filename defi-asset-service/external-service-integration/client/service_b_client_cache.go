package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// getFromCache 从缓存获取数据
func (c *ServiceBClient) getFromCache(ctx context.Context, address string, chainID int) (*UserPositionsResponse, error) {
	cacheKey := fmt.Sprintf("%s:%s:%d", c.config.CachePrefix, address, chainID)
	
	// 从Redis获取数据
	data, err := c.redisClient.Get(ctx, cacheKey).Result()
	if err == redis.Nil {
		return nil, nil // 缓存未命中
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get from cache: %w", err)
	}
	
	// 解析数据
	var response UserPositionsResponse
	if err := json.Unmarshal([]byte(data), &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached data: %w", err)
	}
	
	// 设置缓存相关字段
	response.Cached = true
	ttl, err := c.redisClient.TTL(ctx, cacheKey).Result()
	if err == nil && ttl > 0 {
		response.CacheExpiresAt = time.Now().Add(ttl).Format(time.RFC3339)
	}
	
	log.Debug().
		Str("address", address).
		Int("chain_id", chainID).
		Msg("cache hit for user positions")
	
	return &response, nil
}

// saveToCache 保存数据到缓存
func (c *ServiceBClient) saveToCache(ctx context.Context, address string, chainID int, response *UserPositionsResponse) error {
	cacheKey := fmt.Sprintf("%s:%s:%d", c.config.CachePrefix, address, chainID)
	
	// 序列化数据
	data, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal data for cache: %w", err)
	}
	
	// 保存到Redis
	if err := c.redisClient.Set(ctx, cacheKey, data, c.config.CacheTTL).Err(); err != nil {
		return fmt.Errorf("failed to save to cache: %w", err)
	}
	
	log.Debug().
		Str("address", address).
		Int("chain_id", chainID).
		Dur("ttl", c.config.CacheTTL).
		Msg("cached user positions")
	
	return nil
}

// clearPositionCache 清除仓位相关缓存
func (c *ServiceBClient) clearPositionCache(ctx context.Context, position PositionData) error {
	// 这里可以根据需要清除相关缓存
	// 例如：清除用户的所有仓位缓存
	// cacheKey := fmt.Sprintf("%s:*", c.config.CachePrefix)
	// keys, err := c.redisClient.Keys(ctx, cacheKey).Result()
	// if err != nil {
	//     return err
	// }
	// if len(keys) > 0 {
	//     return c.redisClient.Del(ctx, keys...).Err()
	// }
	
	return nil
}

// getFromDatabase 从数据库获取数据
func (c *ServiceBClient) getFromDatabase(ctx context.Context, address string, chainID int) (*UserPositionsResponse, error) {
	// 查询用户信息
	var user struct {
		ID      uint   `gorm:"column:id"`
		Address string `gorm:"column:address"`
	}
	
	if err := c.db.WithContext(ctx).
		Table("users").
		Select("id, address").
		Where("address = ? AND chain_id = ?", address, chainID).
		First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}
	
	// 查询用户仓位
	var positions []PositionData
	if err := c.db.WithContext(ctx).
		Table("user_positions").
		Where("user_id = ? AND is_active = ?", user.ID, true).
		Order("value_usd DESC").
		Find(&positions).Error; err != nil {
		return nil, fmt.Errorf("failed to query positions: %w", err)
	}
	
	if len(positions) == 0 {
		return nil, nil
	}
	
	// 计算总价值
	var totalValueUSD float64
	for _, position := range positions {
		var value float64
		if _, err := fmt.Sscanf(position.ValueUSD, "%f", &value); err == nil {
			totalValueUSD += value
		}
	}
	
	response := &UserPositionsResponse{
		Address:       address,
		ChainID:       chainID,
		TotalValueUSD: fmt.Sprintf("%.2f", totalValueUSD),
		Positions:     positions,
		Cached:        true,
		LastUpdatedAt: time.Now().Format(time.RFC3339),
	}
	
	log.Debug().
		Str("address", address).
		Int("chain_id", chainID).
		Int("position_count", len(positions)).
		Msg("database hit for user positions")
	
	return response, nil
}

// saveToDatabase 保存数据到数据库
func (c *ServiceBClient) saveToDatabase(ctx context.Context, address string, chainID int, response *UserPositionsResponse) error {
	// 开始事务
	tx := c.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	
	// 获取或创建用户
	var user struct {
		ID      uint   `gorm:"column:id"`
		Address string `gorm:"column:address"`
	}
	
	if err := tx.Table("users").
		Select("id, address").
		Where("address = ? AND chain_id = ?", address, chainID).
		First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 创建新用户
			userRecord := map[string]interface{}{
				"address":           address,
				"chain_id":          chainID,
				"total_assets_usd":  0,
				"last_updated_at":   gorm.Expr("NOW()"),
			}
			
			if err := tx.Table("users").Create(&userRecord).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create user: %w", err)
			}
			
			// 获取新用户的ID
			if err := tx.Table("users").
				Select("id").
				Where("address = ? AND chain_id = ?", address, chainID).
				First(&user).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to get new user ID: %w", err)
			}
		} else {
			tx.Rollback()
			return fmt.Errorf("failed to query user: %w", err)
		}
	}
	
	// 更新用户总资产
	var totalValue float64
	for _, position := range response.Positions {
		var value float64
		if _, err := fmt.Sscanf(position.ValueUSD, "%f", &value); err == nil {
			totalValue += value
		}
	}
	
	if err := tx.Table("users").
		Where("id = ?", user.ID).
		Updates(map[string]interface{}{
			"total_assets_usd": totalValue,
			"last_updated_at":  gorm.Expr("NOW()"),
		}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to update user: %w", err)
	}
	
	// 保存仓位数据
	for _, position := range response.Positions {
		positionRecord := map[string]interface{}{
			"user_id":              user.ID,
			"protocol_id":          position.ProtocolID,
			"position_id":          position.PositionID,
			"position_type":        position.PositionType,
			"token_address":        position.TokenAddress,
			"token_symbol":         position.TokenSymbol,
			"token_name":           position.TokenName,
			"amount_raw":           position.AmountRaw,
			"amount_decimal":       position.Amount,
			"price_usd":            position.PriceUSD,
			"value_usd":            position.ValueUSD,
			"apy":                  position.APY,
			"health_factor":        position.HealthFactor,
			"liquidation_threshold": position.LiquidationThreshold,
			"collateral_factor":    position.CollateralFactor,
			"position_data":        string(position.PositionDataRaw),
			"is_active":            position.IsActive,
			"last_updated_by":      "service_b",
			"last_updated_at":      gorm.Expr("NOW()"),
		}
		
		// 使用ON DUPLICATE KEY UPDATE
		if err := tx.Exec(`
			INSERT INTO user_positions (
				user_id, protocol_id, position_id, position_type,
				token_address, token_symbol, token_name,
				amount_raw, amount_decimal, price_usd, value_usd,
				apy, health_factor, liquidation_threshold, collateral_factor,
				position_data, is_active, last_updated_by, last_updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
			ON DUPLICATE KEY UPDATE
				amount_raw = VALUES(amount_raw),
				amount_decimal = VALUES(amount_decimal),
				price_usd = VALUES(price_usd),
				value_usd = VALUES(value_usd),
				apy = VALUES(apy),
				health_factor = VALUES(health_factor),
				position_data = VALUES(position_data),
				is_active = VALUES(is_active),
				last_updated_by = VALUES(last_updated_by),
				last_updated_at = VALUES(last_updated_at)
		`,
			positionRecord["user_id"],
			positionRecord["protocol_id"],
			positionRecord["position_id"],
			positionRecord["position_type"],
			positionRecord["token_address"],
			positionRecord["token_symbol"],
			positionRecord["token_name"],
			positionRecord["amount_raw"],
			positionRecord["amount_decimal"],
			positionRecord["price_usd"],
			positionRecord["value_usd"],
			positionRecord["apy"],
			positionRecord["health_factor"],
			positionRecord["liquidation_threshold"],
			positionRecord["collateral_factor"],
			positionRecord["position_data"],
			positionRecord["is_active"],
			positionRecord["last_updated_by"],
		).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to save position: %w", err)
		}
	}
	
	// 提交事务
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	log.Debug().
		Str("address", address).
		Int("chain_id", chainID).
		Int("position_count", len(response.Positions)).
		Msg("saved user positions to database")
	
	return nil
}

// updatePositionInDatabase 更新数据库中的仓位数据
func (c *ServiceBClient) updatePositionInDatabase(ctx context.Context, position PositionData) error {
	// 这里需要根据positionData中的信息找到对应的用户和仓位
	// 由于这是一个简化示例，我们假设positionData包含所有必要信息
	
	// 在实际实现中，需要：
	// 1. 根据positionData找到对应的用户
	// 2. 更新或插入仓位数据
	// 3. 更新用户总资产
	
	return c.db.WithContext(ctx).Exec(`
		INSERT INTO user_positions (
			user_id, protocol_id, position_id, position_type,
			token_address, token_symbol, token_name,
			amount_raw, amount_decimal, price_usd, value_usd,
			apy, health_factor, liquidation_threshold, collateral_factor,
			position_data, is_active, last_updated_by, last_updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
		ON DUPLICATE KEY UPDATE
			amount_raw = VALUES(amount_raw),
			amount_decimal = VALUES(amount_decimal),
			price_usd = VALUES(price_usd),
			value_usd = VALUES(value_usd),
			apy = VALUES(apy),
			health_factor = VALUES(health_factor),
			position_data = VALUES(position_data),
			is_active = VALUES(is_active),
			last_updated_by = VALUES(last_updated_by),
			last_updated_at = VALUES(last_updated_at)
	`,
		// 这里需要实际的参数值
	).Error
}

// executeProtectedRequest 执行受保护的请求（熔断器 + 重试）
func (c *ServiceBClient) executeProtectedRequest(ctx context.Context, fn func() error) error {
	// 使用熔断器执行操作
	err := c.circuitBreaker.Execute(ctx, func() error {
		// 使用重试管理器执行操作
		return c.retryManager.Execute(ctx, func(ctx context.Context) error {
			return fn()
		})
	})
	
	return err
}

// getHeaders 获取请求头
func (c *ServiceBClient) getHeaders() map[string]string {
	headers := map[string]string{
		"Accept": "application/json",
	}
	
	if c.config.APIKey != "" {
		headers["Authorization"] = fmt.Sprintf("Bearer %s", c.config.APIKey)
		headers["X-API-Key"] = c.config.APIKey
	}
	
	return headers
}

// Close 关闭客户端
func (c *ServiceBClient) Close() {
	c.httpClient.Close()
	if c.redisClient != nil {
		c.redisClient.Close()
	}
}