package repository

import (
	"context"
	"fmt"
	"time"

	"defi-asset-service/internal/model"

	"gorm.io/gorm"
)

// UserRepository 用户仓库接口
type UserRepository interface {
	// 用户相关
	CreateUser(ctx context.Context, user *model.User) error
	GetUserByAddress(ctx context.Context, address string, chainID int) (*model.User, error)
	GetUserByID(ctx context.Context, id uint64) (*model.User, error)
	UpdateUser(ctx context.Context, user *model.User) error
	UpdateUserAssets(ctx context.Context, userID uint64, totalAssetsUSD float64) error
	ListUsers(ctx context.Context, limit, offset int) ([]model.User, error)
	CountUsers(ctx context.Context) (int64, error)
	
	// 用户资产相关
	CreateUserAsset(ctx context.Context, asset *model.UserAsset) error
	BatchCreateUserAssets(ctx context.Context, assets []model.UserAsset) error
	GetUserAssets(ctx context.Context, userID uint64, chainID int, protocolID string) ([]model.UserAsset, error)
	GetLatestUserAssets(ctx context.Context, userID uint64, limit int) ([]model.UserAsset, error)
	DeleteOldUserAssets(ctx context.Context, before time.Time) error
	
	// 用户仓位相关
	CreateUserPosition(ctx context.Context, position *model.UserPosition) error
	UpdateUserPosition(ctx context.Context, position *model.UserPosition) error
	GetUserPosition(ctx context.Context, userID uint64, protocolID, positionID string) (*model.UserPosition, error)
	GetUserPositions(ctx context.Context, userID uint64, protocolID, positionType string) ([]model.UserPosition, error)
	GetActiveUserPositions(ctx context.Context, userID uint64) ([]model.UserPosition, error)
	BatchUpdateUserPositions(ctx context.Context, positions []model.UserPosition) error
	DeactivateOldPositions(ctx context.Context, userID uint64, updatedBefore time.Time) error
	
	// 汇总查询
	GetUserAssetSummary(ctx context.Context, userID uint64) (*model.UserAssetSummary, error)
	GetUserTotalValue(ctx context.Context, userID uint64) (float64, error)
}

// userRepository 用户仓库实现
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository 创建用户仓库实例
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// CreateUser 创建用户
func (r *userRepository) CreateUser(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// GetUserByAddress 根据地址获取用户
func (r *userRepository) GetUserByAddress(ctx context.Context, address string, chainID int) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).
		Where("address = ? AND chain_id = ?", address, chainID).
		First(&user).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// GetUserByID 根据ID获取用户
func (r *userRepository) GetUserByID(ctx context.Context, id uint64) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).First(&user, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// UpdateUser 更新用户
func (r *userRepository) UpdateUser(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// UpdateUserAssets 更新用户资产总额
func (r *userRepository) UpdateUserAssets(ctx context.Context, userID uint64, totalAssetsUSD float64) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"total_assets_usd": totalAssetsUSD,
			"last_updated_at":  time.Now(),
		}).Error
}

// ListUsers 列出用户
func (r *userRepository) ListUsers(ctx context.Context, limit, offset int) ([]model.User, error) {
	var users []model.User
	err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&users).Error
	return users, err
}

// CountUsers 统计用户数量
func (r *userRepository) CountUsers(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Count(&count).Error
	return count, err
}

// CreateUserAsset 创建用户资产
func (r *userRepository) CreateUserAsset(ctx context.Context, asset *model.UserAsset) error {
	return r.db.WithContext(ctx).Create(asset).Error
}

// BatchCreateUserAssets 批量创建用户资产
func (r *userRepository) BatchCreateUserAssets(ctx context.Context, assets []model.UserAsset) error {
	if len(assets) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).CreateInBatches(assets, 100).Error
}

// GetUserAssets 获取用户资产
func (r *userRepository) GetUserAssets(ctx context.Context, userID uint64, chainID int, protocolID string) ([]model.UserAsset, error) {
	var assets []model.UserAsset
	query := r.db.WithContext(ctx).
		Where("user_id = ?", userID)
	
	if chainID > 0 {
		query = query.Where("chain_id = ?", chainID)
	}
	
	if protocolID != "" {
		query = query.Where("protocol_id = ?", protocolID)
	}
	
	err := query.
		Order("value_usd DESC").
		Find(&assets).Error
	return assets, err
}

// GetLatestUserAssets 获取最新用户资产
func (r *userRepository) GetLatestUserAssets(ctx context.Context, userID uint64, limit int) ([]model.UserAsset, error) {
	var assets []model.UserAsset
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("queried_at DESC").
		Limit(limit).
		Find(&assets).Error
	return assets, err
}

// DeleteOldUserAssets 删除旧的用户资产
func (r *userRepository) DeleteOldUserAssets(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).
		Where("queried_at < ?", before).
		Delete(&model.UserAsset{}).Error
}

// CreateUserPosition 创建用户仓位
func (r *userRepository) CreateUserPosition(ctx context.Context, position *model.UserPosition) error {
	return r.db.WithContext(ctx).Create(position).Error
}

// UpdateUserPosition 更新用户仓位
func (r *userRepository) UpdateUserPosition(ctx context.Context, position *model.UserPosition) error {
	return r.db.WithContext(ctx).Save(position).Error
}

// GetUserPosition 获取用户仓位
func (r *userRepository) GetUserPosition(ctx context.Context, userID uint64, protocolID, positionID string) (*model.UserPosition, error) {
	var position model.UserPosition
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND protocol_id = ? AND position_id = ?", userID, protocolID, positionID).
		First(&position).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &position, nil
}

// GetUserPositions 获取用户仓位列表
func (r *userRepository) GetUserPositions(ctx context.Context, userID uint64, protocolID, positionType string) ([]model.UserPosition, error) {
	var positions []model.UserPosition
	query := r.db.WithContext(ctx).
		Where("user_id = ? AND is_active = ?", userID, true)
	
	if protocolID != "" {
		query = query.Where("protocol_id = ?", protocolID)
	}
	
	if positionType != "" {
		query = query.Where("position_type = ?", positionType)
	}
	
	err := query.
		Order("value_usd DESC").
		Find(&positions).Error
	return positions, err
}

// GetActiveUserPositions 获取活跃用户仓位
func (r *userRepository) GetActiveUserPositions(ctx context.Context, userID uint64) ([]model.UserPosition, error) {
	var positions []model.UserPosition
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND is_active = ?", userID, true).
		Order("last_updated_at DESC").
		Find(&positions).Error
	return positions, err
}

// BatchUpdateUserPositions 批量更新用户仓位
func (r *userRepository) BatchUpdateUserPositions(ctx context.Context, positions []model.UserPosition) error {
	if len(positions) == 0 {
		return nil
	}
	
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, position := range positions {
			if err := tx.Save(&position).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeactivateOldPositions 停用旧的仓位
func (r *userRepository) DeactivateOldPositions(ctx context.Context, userID uint64, updatedBefore time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.UserPosition{}).
		Where("user_id = ? AND is_active = ? AND last_updated_at < ?", userID, true, updatedBefore).
		Update("is_active", false).Error
}

// GetUserAssetSummary 获取用户资产汇总
func (r *userRepository) GetUserAssetSummary(ctx context.Context, userID uint64) (*model.UserAssetSummary, error) {
	var summary model.UserAssetSummary
	
	// 使用原生SQL查询视图
	err := r.db.WithContext(ctx).
		Table("user_asset_summary").
		Where("user_id = ?", userID).
		First(&summary).Error
	
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// 视图可能不存在，手动计算
			return r.calculateUserAssetSummary(ctx, userID)
		}
		return nil, err
	}
	
	return &summary, nil
}

// calculateUserAssetSummary 计算用户资产汇总
func (r *userRepository) calculateUserAssetSummary(ctx context.Context, userID uint64) (*model.UserAssetSummary, error) {
	var summary model.UserAssetSummary
	
	// 获取用户信息
	user, err := r.GetUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	
	summary.UserID = user.ID
	summary.Address = user.Address
	summary.ChainID = user.ChainID
	
	// 获取仓位统计
	var positionStats struct {
		ProtocolCount      int64
		PositionCount      int64
		TotalPositionValue float64
		LastUpdatedAt      time.Time
	}
	
	err = r.db.WithContext(ctx).
		Model(&model.UserPosition{}).
		Select("COUNT(DISTINCT protocol_id) as protocol_count, COUNT(*) as position_count, SUM(value_usd) as total_position_value, MAX(last_updated_at) as last_updated_at").
		Where("user_id = ? AND is_active = ?", userID, true).
		Scan(&positionStats).Error
	if err != nil {
		return nil, err
	}
	
	summary.ProtocolCount = int(positionStats.ProtocolCount)
	summary.PositionCount = int(positionStats.PositionCount)
	summary.TotalPositionValue = positionStats.TotalPositionValue
	if !positionStats.LastUpdatedAt.IsZero() {
		summary.LastUpdatedAt = positionStats.LastUpdatedAt
	}
	
	// 获取资产统计
	var assetStats struct {
		TotalAssetValue float64
		LastQueriedAt   time.Time
	}
	
	err = r.db.WithContext(ctx).
		Model(&model.UserAsset{}).
		Select("SUM(value_usd) as total_asset_value, MAX(queried_at) as last_queried_at").
		Where("user_id = ?", userID).
		Scan(&assetStats).Error
	if err != nil {
		return nil, err
	}
	
	summary.TotalAssetValue = assetStats.TotalAssetValue
	if !assetStats.LastQueriedAt.IsZero() && assetStats.LastQueriedAt.After(summary.LastUpdatedAt) {
		summary.LastUpdatedAt = assetStats.LastQueriedAt
	}
	
	// 计算总价值
	summary.TotalValueUSD = summary.TotalPositionValue + summary.TotalAssetValue
	
	return &summary, nil
}

// GetUserTotalValue 获取用户总资产价值
func (r *userRepository) GetUserTotalValue(ctx context.Context, userID uint64) (float64, error) {
	var totalValue float64
	
	// 获取仓位总价值
	var positionValue float64
	err := r.db.WithContext(ctx).
		Model(&model.UserPosition{}).
		Select("COALESCE(SUM(value_usd), 0)").
		Where("user_id = ? AND is_active = ?", userID, true).
		Scan(&positionValue).Error
	if err != nil {
		return 0, err
	}
	
	// 获取资产总价值
	var assetValue float64
	err = r.db.WithContext(ctx).
		Model(&model.UserAsset{}).
		Select("COALESCE(SUM(value_usd), 0)").
		Where("user_id = ?", userID).
		Scan(&assetValue).Error
	if err != nil {
		return 0, err
	}
	
	totalValue = positionValue + assetValue
	return totalValue, nil
}