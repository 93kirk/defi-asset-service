package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"

	"defi-asset-service/queue-system/models"
)

// PositionProcessor 仓位处理器
type PositionProcessor struct {
	db          *sql.DB
	redisClient *redis.Client
	logger      *log.Logger
}

// NewPositionProcessor 创建新的仓位处理器
func NewPositionProcessor(db *sql.DB, redisClient *redis.Client, logger *log.Logger) *PositionProcessor {
	return &PositionProcessor{
		db:          db,
		redisClient: redisClient,
		logger:      logger,
	}
}

// ProcessPositionUpdate 处理仓位更新
func (p *PositionProcessor) ProcessPositionUpdate(ctx context.Context, msg *models.PositionUpdateMessage) error {
	// 开始数据库事务
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()
	
	// 1. 更新或创建用户记录
	if err := p.upsertUser(tx, msg); err != nil {
		return fmt.Errorf("failed to upsert user: %w", err)
	}
	
	// 2. 更新仓位记录
	if err := p.upsertPosition(tx, msg); err != nil {
		return fmt.Errorf("failed to upsert position: %w", err)
	}
	
	// 3. 更新用户总资产
	if err := p.updateUserTotalAssets(tx, msg); err != nil {
		return fmt.Errorf("failed to update user total assets: %w", err)
	}
	
	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	
	// 4. 更新Redis缓存
	if err := p.updateCache(ctx, msg); err != nil {
		// 缓存更新失败不影响主流程，但需要记录日志
		p.logger.Printf("Failed to update cache for user %s: %v", msg.UserAddress, err)
	}
	
	return nil
}

// upsertUser 更新或创建用户记录
func (p *PositionProcessor) upsertUser(tx *sql.Tx, msg *models.PositionUpdateMessage) error {
	query := `
		INSERT INTO users (address, chain_id, last_updated_at, created_at, updated_at)
		VALUES (?, ?, NOW(), NOW(), NOW())
		ON DUPLICATE KEY UPDATE
		last_updated_at = NOW(),
		updated_at = NOW()
	`
	
	_, err := tx.Exec(query, msg.UserAddress, msg.ChainID)
	if err != nil {
		return fmt.Errorf("failed to upsert user: %w", err)
	}
	
	return nil
}

// upsertPosition 更新或创建仓位记录
func (p *PositionProcessor) upsertPosition(tx *sql.Tx, msg *models.PositionUpdateMessage) error {
	// 首先确保协议存在
	if err := p.ensureProtocolExists(tx, msg.ProtocolID); err != nil {
		p.logger.Printf("Warning: protocol %s may not exist: %v", msg.ProtocolID, err)
	}
	
	// 转换元数据为JSON
	metadataJSON, err := json.Marshal(msg.Position.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	// 更新仓位记录
	query := `
		INSERT INTO user_positions 
		(user_address, protocol_id, chain_id, token_address, token_symbol, amount, amount_usd, apy, risk_level, metadata, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
		ON DUPLICATE KEY UPDATE
		token_symbol = VALUES(token_symbol),
		amount = VALUES(amount),
		amount_usd = VALUES(amount_usd),
		apy = VALUES(apy),
		risk_level = VALUES(risk_level),
		metadata = VALUES(metadata),
		updated_at = NOW()
	`
	
	_, err = tx.Exec(query,
		msg.UserAddress,
		msg.ProtocolID,
		msg.ChainID,
		msg.Position.TokenAddress,
		msg.Position.TokenSymbol,
		msg.Position.Amount,
		msg.Position.AmountUSD,
		msg.Position.APY,
		msg.Position.RiskLevel,
		metadataJSON,
	)
	
	if err != nil {
		return fmt.Errorf("failed to upsert position: %w", err)
	}
	
	return nil
}

// ensureProtocolExists 确保协议存在
func (p *PositionProcessor) ensureProtocolExists(tx *sql.Tx, protocolID string) error {
	// 检查协议是否存在
	var count int
	err := tx.QueryRow("SELECT COUNT(*) FROM protocols WHERE protocol_id = ?", protocolID).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check protocol existence: %w", err)
	}
	
	if count == 0 {
		// 协议不存在，创建默认记录
		query := `
			INSERT INTO protocols (protocol_id, name, category, is_active, created_at, updated_at)
			VALUES (?, ?, 'unknown', TRUE, NOW(), NOW())
			ON DUPLICATE KEY UPDATE updated_at = NOW()
		`
		
		_, err := tx.Exec(query, protocolID, protocolID)
		if err != nil {
			return fmt.Errorf("failed to create default protocol: %w", err)
		}
		
		p.logger.Printf("Created default protocol record for %s", protocolID)
	}
	
	return nil
}

// updateUserTotalAssets 更新用户总资产
func (p *PositionProcessor) updateUserTotalAssets(tx *sql.Tx, msg *models.PositionUpdateMessage) error {
	// 计算用户所有仓位的总价值
	query := `
		UPDATE users u
		SET total_assets_usd = (
			SELECT COALESCE(SUM(amount_usd), 0)
			FROM user_positions up
			WHERE up.user_address = u.address
			AND up.chain_id = u.chain_id
		),
		last_updated_at = NOW(),
		updated_at = NOW()
		WHERE u.address = ?
		AND u.chain_id = ?
	`
	
	_, err := tx.Exec(query, msg.UserAddress, msg.ChainID)
	if err != nil {
		return fmt.Errorf("failed to update user total assets: %w", err)
	}
	
	return nil
}

// updateCache 更新Redis缓存
func (p *PositionProcessor) updateCache(ctx context.Context, msg *models.PositionUpdateMessage) error {
	// 1. 更新单个协议仓位缓存
	if err := p.updatePositionCache(ctx, msg); err != nil {
		return fmt.Errorf("failed to update position cache: %w", err)
	}
	
	// 2. 清除用户所有仓位缓存
	if err := p.clearUserPositionsCache(ctx, msg); err != nil {
		return fmt.Errorf("failed to clear user positions cache: %w", err)
	}
	
	// 3. 更新布隆过滤器
	if err := p.updateBloomFilter(ctx, msg); err != nil {
		// 布隆过滤器更新失败不影响主流程
		p.logger.Printf("Failed to update bloom filter: %v", err)
	}
	
	return nil
}

// updatePositionCache 更新单个协议仓位缓存
func (p *PositionProcessor) updatePositionCache(ctx context.Context, msg *models.PositionUpdateMessage) error {
	cacheKey := fmt.Sprintf("defi:position:%s:%s", msg.UserAddress, msg.ProtocolID)
	
	// 准备缓存数据
	positionData := map[string]interface{}{
		"token_address": msg.Position.TokenAddress,
		"token_symbol":  msg.Position.TokenSymbol,
		"amount":        msg.Position.Amount,
		"amount_usd":    msg.Position.AmountUSD,
		"apy":           msg.Position.APY,
		"risk_level":    msg.Position.RiskLevel,
		"updated_at":    time.Now().Unix(),
	}
	
	positionJSON, err := json.Marshal(positionData)
	if err != nil {
		return fmt.Errorf("failed to marshal position data: %w", err)
	}
	
	cacheData := map[string]interface{}{
		"data":       string(positionJSON),
		"cached_at":  fmt.Sprintf("%d", time.Now().Unix()),
		"ttl":        "600",
		"protocol":   msg.ProtocolID,
		"chain_id":   msg.ChainID,
	}
	
	// 使用Hash存储
	if err := p.redisClient.HSet(ctx, cacheKey, cacheData).Err(); err != nil {
		return fmt.Errorf("failed to set position cache: %w", err)
	}
	
	// 设置过期时间
	if err := p.redisClient.Expire(ctx, cacheKey, 600*time.Second).Err(); err != nil {
		return fmt.Errorf("failed to set cache expiration: %w", err)
	}
	
	return nil
}

// clearUserPositionsCache 清除用户所有仓位缓存
func (p *PositionProcessor) clearUserPositionsCache(ctx context.Context, msg *models.PositionUpdateMessage) error {
	// 清除用户所有仓位缓存
	userPositionsKey := fmt.Sprintf("defi:positions:%s", msg.UserAddress)
	if err := p.redisClient.Del(ctx, userPositionsKey).Err(); err != nil {
		return fmt.Errorf("failed to delete user positions cache: %w", err)
	}
	
	// 清除按协议分类的缓存
	pattern := fmt.Sprintf("defi:positions:%s:*", msg.UserAddress)
	iter := p.redisClient.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := p.redisClient.Del(ctx, iter.Val()).Err(); err != nil {
			p.logger.Printf("Failed to delete cache key %s: %v", iter.Val(), err)
		}
	}
	
	if err := iter.Err(); err != nil {
		return fmt.Errorf("failed to scan cache keys: %w", err)
	}
	
	return nil
}

// updateBloomFilter 更新布隆过滤器
func (p *PositionProcessor) updateBloomFilter(ctx context.Context, msg *models.PositionUpdateMessage) error {
	// 检查布隆过滤器是否存在
	filterKey := "defi:bloom:positions"
	
	// 添加用户到布隆过滤器
	if err := p.redisClient.Do(ctx, "BF.ADD", filterKey, msg.UserAddress).Err(); err != nil {
		// 如果命令不存在，可能是Redis版本不支持布隆过滤器
		// 在这种情况下，我们可以使用Set作为回退方案
		fallbackKey := fmt.Sprintf("defi:users:has_positions:%s", msg.UserAddress)
		if err := p.redisClient.Set(ctx, fallbackKey, "1", 24*time.Hour).Err(); err != nil {
			return fmt.Errorf("failed to update bloom filter fallback: %w", err)
		}
	}
	
	return nil
}

// GetUserPositions 获取用户所有仓位（从缓存或数据库）
func (p *PositionProcessor) GetUserPositions(ctx context.Context, userAddress string, chainID int) ([]models.PositionData, error) {
	// 首先尝试从缓存获取
	cacheKey := fmt.Sprintf("defi:positions:%s", userAddress)
	cachedData, err := p.redisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		// 缓存命中，解析数据
		var positions []models.PositionData
		if err := json.Unmarshal([]byte(cachedData), &positions); err == nil {
			return positions, nil
		}
	}
	
	// 缓存未命中或解析失败，从数据库获取
	positions, err := p.getUserPositionsFromDB(ctx, userAddress, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user positions from DB: %w", err)
	}
	
	// 更新缓存
	positionsJSON, err := json.Marshal(positions)
	if err == nil {
		if err := p.redisClient.Set(ctx, cacheKey, positionsJSON, 300*time.Second).Err(); err != nil {
			p.logger.Printf("Failed to cache user positions: %v", err)
		}
	}
	
	return positions, nil
}

// getUserPositionsFromDB 从数据库获取用户所有仓位
func (p *PositionProcessor) getUserPositionsFromDB(ctx context.Context, userAddress string, chainID int) ([]models.PositionData, error) {
	query := `
		SELECT 
			token_address,
			token_symbol,
			amount,
			amount_usd,
			apy,
			risk_level,
			metadata
		FROM user_positions
		WHERE user_address = ?
		AND chain_id = ?
		ORDER BY amount_usd DESC
	`
	
	rows, err := p.db.QueryContext(ctx, query, userAddress, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user positions: %w", err)
	}
	defer rows.Close()
	
	var positions []models.PositionData
	
	for rows.Next() {
		var position models.PositionData
		var metadataJSON []byte
		
		err := rows.Scan(
			&position.TokenAddress,
			&position.TokenSymbol,
			&position.Amount,
			&position.AmountUSD,
			&position.APY,
			&position.RiskLevel,
			&metadataJSON,
		)
		
		if err != nil {
			return nil, fmt.Errorf("failed to scan position row: %w", err)
		}
		
		// 解析元数据
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &position.Metadata); err != nil {
				p.logger.Printf("Failed to unmarshal metadata: %v", err)
			}
		}
		
		positions = append(positions, position)
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}
	
	return positions, nil
}

// CleanupOldPositions 清理旧的仓位数据
func (p *PositionProcessor) CleanupOldPositions(ctx context.Context, maxAge time.Duration) (int64, error) {
	// 删除超过指定时间的仓位记录
	cutoffTime := time.Now().Add(-maxAge)
	
	query := `
		DELETE FROM user_positions
		WHERE updated_at < ?
	`
	
	result, err := p.db.ExecContext(ctx, query, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old positions: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	// 清理对应的缓存
	if rowsAffected > 0 {
		p.logger.Printf("Cleaned up %d old position records older than %v", rowsAffected, maxAge)
	}
	
	return rowsAffected, nil
}