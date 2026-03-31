package repository

import (
	"context"
	"fmt"
	"time"

	"defi-asset-service/internal/model"

	"gorm.io/gorm"
)

// ProtocolRepository 协议仓库接口
type ProtocolRepository interface {
	// 协议相关
	CreateProtocol(ctx context.Context, protocol *model.Protocol) error
	GetProtocolByID(ctx context.Context, protocolID string) (*model.Protocol, error)
	GetProtocolByInternalID(ctx context.Context, id uint64) (*model.Protocol, error)
	UpdateProtocol(ctx context.Context, protocol *model.Protocol) error
	DeleteProtocol(ctx context.Context, protocolID string) error
	ListProtocols(ctx context.Context, query model.ProtocolQuery) ([]model.Protocol, int64, error)
	BatchCreateOrUpdateProtocols(ctx context.Context, protocols []model.Protocol) error
	
	// 协议代币相关
	CreateProtocolToken(ctx context.Context, token *model.ProtocolToken) error
	GetProtocolToken(ctx context.Context, protocolID string, chainID int, tokenAddress string) (*model.ProtocolToken, error)
	GetProtocolTokens(ctx context.Context, protocolID string, query model.TokenQuery) ([]model.ProtocolToken, error)
	UpdateProtocolToken(ctx context.Context, token *model.ProtocolToken) error
	BatchCreateOrUpdateProtocolTokens(ctx context.Context, tokens []model.ProtocolToken) error
	DeleteProtocolTokens(ctx context.Context, protocolID string, chainID int) error
	
	// 同步记录相关
	CreateSyncRecord(ctx context.Context, record *model.SyncRecord) error
	UpdateSyncRecord(ctx context.Context, record *model.SyncRecord) error
	GetSyncRecord(ctx context.Context, syncID string) (*model.SyncRecord, error)
	GetLatestSyncRecord(ctx context.Context, syncType, syncSource string) (*model.SyncRecord, error)
	ListSyncRecords(ctx context.Context, syncType, syncSource, status string, limit, offset int) ([]model.SyncRecord, error)
	
	// 统计相关
	GetProtocolStatistics(ctx context.Context, protocolID string) (*model.ProtocolStatistics, error)
	GetActiveProtocolCount(ctx context.Context) (int64, error)
	GetProtocolsByCategory(ctx context.Context, category string) ([]model.Protocol, error)
}

// protocolRepository 协议仓库实现
type protocolRepository struct {
	db *gorm.DB
}

// NewProtocolRepository 创建协议仓库实例
func NewProtocolRepository(db *gorm.DB) ProtocolRepository {
	return &protocolRepository{db: db}
}

// CreateProtocol 创建协议
func (r *protocolRepository) CreateProtocol(ctx context.Context, protocol *model.Protocol) error {
	return r.db.WithContext(ctx).Create(protocol).Error
}

// GetProtocolByID 根据协议ID获取协议
func (r *protocolRepository) GetProtocolByID(ctx context.Context, protocolID string) (*model.Protocol, error) {
	var protocol model.Protocol
	err := r.db.WithContext(ctx).
		Where("protocol_id = ?", protocolID).
		First(&protocol).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &protocol, nil
}

// GetProtocolByInternalID 根据内部ID获取协议
func (r *protocolRepository) GetProtocolByInternalID(ctx context.Context, id uint64) (*model.Protocol, error) {
	var protocol model.Protocol
	err := r.db.WithContext(ctx).First(&protocol, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &protocol, nil
}

// UpdateProtocol 更新协议
func (r *protocolRepository) UpdateProtocol(ctx context.Context, protocol *model.Protocol) error {
	return r.db.WithContext(ctx).Save(protocol).Error
}

// DeleteProtocol 删除协议
func (r *protocolRepository) DeleteProtocol(ctx context.Context, protocolID string) error {
	return r.db.WithContext(ctx).
		Where("protocol_id = ?", protocolID).
		Delete(&model.Protocol{}).Error
}

// ListProtocols 列出协议
func (r *protocolRepository) ListProtocols(ctx context.Context, query model.ProtocolQuery) ([]model.Protocol, int64, error) {
	var protocols []model.Protocol
	var total int64
	
	db := r.db.WithContext(ctx).Model(&model.Protocol{})
	
	// 构建查询条件
	if query.Category != "" {
		db = db.Where("category = ?", query.Category)
	}
	
	if query.ChainID > 0 {
		db = db.Where("JSON_CONTAINS(supported_chains, ?)", query.ChainID)
	}
	
	if query.IsActive {
		db = db.Where("is_active = ?", true)
	}
	
	// 计算总数
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	
	// 查询数据
	err := db.
		Order("tvl_usd DESC").
		Limit(query.PageSize).
		Offset((query.Page - 1) * query.PageSize).
		Find(&protocols).Error
	
	return protocols, total, err
}

// BatchCreateOrUpdateProtocols 批量创建或更新协议
func (r *protocolRepository) BatchCreateOrUpdateProtocols(ctx context.Context, protocols []model.Protocol) error {
	if len(protocols) == 0 {
		return nil
	}
	
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, protocol := range protocols {
			// 检查协议是否存在
			var existing model.Protocol
			err := tx.Where("protocol_id = ?", protocol.ProtocolID).First(&existing).Error
			
			if err == gorm.ErrRecordNotFound {
				// 创建新协议
				if err := tx.Create(&protocol).Error; err != nil {
					return err
				}
			} else if err == nil {
				// 更新现有协议
				protocol.ID = existing.ID
				protocol.CreatedAt = existing.CreatedAt
				protocol.SyncVersion = existing.SyncVersion + 1
				if err := tx.Save(&protocol).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}
		return nil
	})
}

// CreateProtocolToken 创建协议代币
func (r *protocolRepository) CreateProtocolToken(ctx context.Context, token *model.ProtocolToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

// GetProtocolToken 获取协议代币
func (r *protocolRepository) GetProtocolToken(ctx context.Context, protocolID string, chainID int, tokenAddress string) (*model.ProtocolToken, error) {
	var token model.ProtocolToken
	err := r.db.WithContext(ctx).
		Where("protocol_id = ? AND chain_id = ? AND token_address = ?", protocolID, chainID, tokenAddress).
		First(&token).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &token, nil
}

// GetProtocolTokens 获取协议代币列表
func (r *protocolRepository) GetProtocolTokens(ctx context.Context, protocolID string, query model.TokenQuery) ([]model.ProtocolToken, error) {
	var tokens []model.ProtocolToken
	db := r.db.WithContext(ctx).
		Where("protocol_id = ?", protocolID)
	
	if query.ChainID > 0 {
		db = db.Where("chain_id = ?", query.ChainID)
	}
	
	if query.IsCollateral {
		db = db.Where("is_collateral = ?", true)
	}
	
	if query.IsBorrowable {
		db = db.Where("is_borrowable = ?", true)
	}
	
	err := db.
		Order("tvl_usd DESC").
		Find(&tokens).Error
	
	return tokens, err
}

// UpdateProtocolToken 更新协议代币
func (r *protocolRepository) UpdateProtocolToken(ctx context.Context, token *model.ProtocolToken) error {
	return r.db.WithContext(ctx).Save(token).Error
}

// BatchCreateOrUpdateProtocolTokens 批量创建或更新协议代币
func (r *protocolRepository) BatchCreateOrUpdateProtocolTokens(ctx context.Context, tokens []model.ProtocolToken) error {
	if len(tokens) == 0 {
		return nil
	}
	
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, token := range tokens {
			// 检查代币是否存在
			var existing model.ProtocolToken
			err := tx.
				Where("protocol_id = ? AND chain_id = ? AND token_address = ?", 
					token.ProtocolID, token.ChainID, token.TokenAddress).
				First(&existing).Error
			
			if err == gorm.ErrRecordNotFound {
				// 创建新代币
				if err := tx.Create(&token).Error; err != nil {
					return err
				}
			} else if err == nil {
				// 更新现有代币
				token.ID = existing.ID
				token.CreatedAt = existing.CreatedAt
				if err := tx.Save(&token).Error; err != nil {
					return err
				}
			} else {
				return err
			}
		}
		return nil
	})
}

// DeleteProtocolTokens 删除协议代币
func (r *protocolRepository) DeleteProtocolTokens(ctx context.Context, protocolID string, chainID int) error {
	db := r.db.WithContext(ctx).
		Where("protocol_id = ?", protocolID)
	
	if chainID > 0 {
		db = db.Where("chain_id = ?", chainID)
	}
	
	return db.Delete(&model.ProtocolToken{}).Error
}

// CreateSyncRecord 创建同步记录
func (r *protocolRepository) CreateSyncRecord(ctx context.Context, record *model.SyncRecord) error {
	return r.db.WithContext(ctx).Create(record).Error
}

// UpdateSyncRecord 更新同步记录
func (r *protocolRepository) UpdateSyncRecord(ctx context.Context, record *model.SyncRecord) error {
	return r.db.WithContext(ctx).Save(record).Error
}

// GetSyncRecord 获取同步记录
func (r *protocolRepository) GetSyncRecord(ctx context.Context, syncID string) (*model.SyncRecord, error) {
	var record model.SyncRecord
	err := r.db.WithContext(ctx).
		Where("id = ?", syncID).
		First(&record).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

// GetLatestSyncRecord 获取最新同步记录
func (r *protocolRepository) GetLatestSyncRecord(ctx context.Context, syncType, syncSource string) (*model.SyncRecord, error) {
	var record model.SyncRecord
	db := r.db.WithContext(ctx).
		Where("sync_type = ?", syncType)
	
	if syncSource != "" {
		db = db.Where("sync_source = ?", syncSource)
	}
	
	err := db.
		Order("started_at DESC").
		First(&record).Error
	
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

// ListSyncRecords 列出同步记录
func (r *protocolRepository) ListSyncRecords(ctx context.Context, syncType, syncSource, status string, limit, offset int) ([]model.SyncRecord, error) {
	var records []model.SyncRecord
	db := r.db.WithContext(ctx)
	
	if syncType != "" {
		db = db.Where("sync_type = ?", syncType)
	}
	
	if syncSource != "" {
		db = db.Where("sync_source = ?", syncSource)
	}
	
	if status != "" {
		db = db.Where("status = ?", status)
	}
	
	err := db.
		Order("started_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&records).Error
	
	return records, err
}

// GetProtocolStatistics 获取协议统计
func (r *protocolRepository) GetProtocolStatistics(ctx context.Context, protocolID string) (*model.ProtocolStatistics, error) {
	var stats model.ProtocolStatistics
	
	// 使用原生SQL查询视图
	err := r.db.WithContext(ctx).
		Table("protocol_statistics").
		Where("protocol_id = ?", protocolID).
		First(&stats).Error
	
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 视图可能不存在，手动计算
			return r.calculateProtocolStatistics(ctx, protocolID)
		}
		return nil, err
	}
	
	return &stats, nil
}

// calculateProtocolStatistics 计算协议统计
func (r *protocolRepository) calculateProtocolStatistics(ctx context.Context, protocolID string) (*model.ProtocolStatistics, error) {
	var stats model.ProtocolStatistics
	
	// 获取用户统计
	var userStats struct {
		UserCount     int64
		PositionCount int64
		TotalValue    float64
		AvgApy        float64
		LastUpdatedAt time.Time
	}
	
	err := r.db.WithContext(ctx).
		Model(&model.UserPosition{}).
		Select("COUNT(DISTINCT user_id) as user_count, COUNT(*) as position_count, SUM(value_usd) as total_value, AVG(apy) as avg_apy, MAX(last_updated_at) as last_updated_at").
		Where("protocol_id = ? AND is_active = ?", protocolID, true).
		Scan(&userStats).Error
	if err != nil {
		return nil, err
	}
	
	stats.UserCount = int(userStats.UserCount)
	stats.PositionCount = int(userStats.PositionCount)
	stats.TotalPositionValue = userStats.TotalValue
	stats.AvgApy = userStats.AvgApy
	if !userStats.LastUpdatedAt.IsZero() {
		stats.LastUpdatedAt = userStats.LastUpdatedAt.Format(time.RFC3339)
	}
	
	return &stats, nil
}

// GetActiveProtocolCount 获取活跃协议数量
func (r *protocolRepository) GetActiveProtocolCount(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.Protocol{}).
		Where("is_active = ?", true).
		Count(&count).Error
	return count, err
}

// GetProtocolsByCategory 根据类别获取协议
func (r *protocolRepository) GetProtocolsByCategory(ctx context.Context, category string) ([]model.Protocol, error) {
	var protocols []model.Protocol
	err := r.db.WithContext(ctx).
		Where("category = ? AND is_active = ?", category, true).
		Order("tvl_usd DESC").
		Find(&protocols).Error
	return protocols, err
}