package service

import (
	"context"
	"strconv"
	"strings"
	"time"

	"defi-asset-service/internal/model"
	"defi-asset-service/internal/repository"
)

// StoreUserAssets 存储用户资产到数据库（续）
func (s *serviceAService) StoreUserAssets(ctx context.Context, userID uint64, assets []model.AssetResponse) error {
	if len(assets) == 0 {
		return nil
	}
	
	// 转换资产响应为数据库模型
	var userAssets []model.UserAsset
	now := time.Now()
	
	for _, asset := range assets {
		userAsset := model.UserAsset{
			UserID:        userID,
			ChainID:       1, // 默认链ID，实际应从asset中获取
			TokenAddress:  asset.TokenAddress,
			TokenSymbol:   asset.TokenSymbol,
			TokenName:     asset.TokenName,
			TokenDecimals: asset.TokenDecimals,
			BalanceRaw:    asset.BalanceRaw,
			BalanceDecimal: parseBalance(asset.Balance),
			PriceUSD:      asset.PriceUSD,
			ValueUSD:      asset.ValueUSD,
			ProtocolID:    asset.ProtocolID,
			AssetType:     asset.AssetType,
			Source:        "service_a",
			QueriedAt:     now,
			CreatedAt:     now,
		}
		userAssets = append(userAssets, userAsset)
	}
	
	// 批量存储到数据库
	if err := s.userRepo.BatchCreateUserAssets(ctx, userAssets); err != nil {
		return fmt.Errorf("failed to batch create user assets: %w", err)
	}
	
	// 更新用户总资产价值
	totalValue := calculateTotalAssetValue(assets)
	if err := s.userRepo.UpdateUserAssets(ctx, userID, totalValue); err != nil {
		s.logger.WithError(err).Warn("Failed to update user total assets")
	}
	
	return nil
}

// filterAssets 过滤资产
func (s *serviceAService) filterAssets(assets []model.AssetResponse, protocolID, tokenAddress string) []model.AssetResponse {
	if protocolID == "" && tokenAddress == "" {
		return assets
	}
	
	var filtered []model.AssetResponse
	for _, asset := range assets {
		// 协议过滤
		if protocolID != "" && asset.ProtocolID != protocolID {
			continue
		}
		
		// 代币地址过滤
		if tokenAddress != "" && !strings.EqualFold(asset.TokenAddress, tokenAddress) {
			continue
		}
		
		filtered = append(filtered, asset)
	}
	
	return filtered
}

// parseBalance 解析余额字符串为浮点数
func parseBalance(balanceStr string) float64 {
	if balanceStr == "" {
		return 0
	}
	
	balance, err := strconv.ParseFloat(balanceStr, 64)
	if err != nil {
		return 0
	}
	
	return balance
}

// calculateTotalAssetValue 计算总资产价值
func calculateTotalAssetValue(assets []model.AssetResponse) float64 {
	var total float64
	for _, asset := range assets {
		total += asset.ValueUSD
	}
	return total
}

// 导入必要的包
import (
	"fmt"
)