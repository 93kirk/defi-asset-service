package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"defi-asset-service/internal/model"
)

// filterPositions 过滤仓位（续）
func (s *serviceBService) filterPositions(positions []model.PositionResponse, protocolID, positionType string) []model.PositionResponse {
	if protocolID == "" && positionType == "" {
		return positions
	}
	
	var filtered []model.PositionResponse
	for _, position := range positions {
		// 协议过滤
		if protocolID != "" && position.ProtocolID != protocolID {
			continue
		}
		
		// 仓位类型过滤
		if positionType != "" && position.PositionType != positionType {
			continue
		}
		
		filtered = append(filtered, position)
	}
	
	return filtered
}

// validatePositionUpdate 验证仓位更新消息
func (s *serviceBService) validatePositionUpdate(message *model.PositionUpdateMessage) error {
	if message.EventID == "" {
		return fmt.Errorf("event_id is required")
	}
	
	if message.UserAddress == "" {
		return fmt.Errorf("user_address is required")
	}
	
	if message.ProtocolID == "" {
		return fmt.Errorf("protocol_id is required")
	}
	
	if message.PositionData == nil {
		return fmt.Errorf("position_data is required")
	}
	
	return nil
}

// parsePositionData 解析仓位数据
func (s *serviceBService) parsePositionData(userID uint64, message *model.PositionUpdateMessage) (*model.UserPosition, error) {
	// 从position_data中提取必要字段
	data := message.PositionData
	
	// 生成仓位ID
	positionID := generatePositionIDFromData(data)
	
	// 解析金额
	amountRaw, _ := data["amount_raw"].(string)
	amount, _ := data["amount"].(string)
	
	// 解析数值字段
	valueUSD, _ := parseFloat(data["value_usd"])
	priceUSD, _ := parseFloat(data["price_usd"])
	apy, _ := parseFloat(data["apy"])
	healthFactor, _ := parseFloat(data["health_factor"])
	liquidationThreshold, _ := parseFloat(data["liquidation_threshold"])
	collateralFactor, _ := parseFloat(data["collateral_factor"])
	
	position := &model.UserPosition{
		UserID:             userID,
		ProtocolID:         message.ProtocolID,
		PositionID:         positionID,
		PositionType:       getString(data, "position_type", "unknown"),
		TokenAddress:       getString(data, "token_address", ""),
		TokenSymbol:        getString(data, "token_symbol", ""),
		AmountRaw:          amountRaw,
		AmountDecimal:      parseAmount(amount),
		PriceUSD:           priceUSD,
		ValueUSD:           valueUSD,
		Apy:                apy,
		HealthFactor:       healthFactor,
		LiquidationThreshold: liquidationThreshold,
		CollateralFactor:   collateralFactor,
		PositionData:       data,
		IsActive:           true,
		LastUpdatedBy:      "queue_update",
		LastUpdatedAt:      time.Now(),
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	
	return position, nil
}

// shouldUpdatePosition 检查是否需要更新仓位
func (s *serviceBService) shouldUpdatePosition(existing, new *model.UserPosition) bool {
	// 检查价值变化是否超过阈值
	valueChange := abs(new.ValueUSD - existing.ValueUSD)
	valueChangePercent := valueChange / existing.ValueUSD
	
	if valueChangePercent > s.businessConfig.PositionRefreshThreshold {
		return true
	}
	
	// 检查其他重要字段是否有变化
	if new.PositionType != existing.PositionType {
		return true
	}
	
	if new.AmountDecimal != existing.AmountDecimal {
		return true
	}
	
	if new.Apy != existing.Apy {
		return true
	}
	
	if new.HealthFactor != existing.HealthFactor {
		return true
	}
	
	// 如果距离上次更新时间超过1小时，也更新
	if time.Since(existing.LastUpdatedAt) > time.Hour {
		return true
	}
	
	return false
}

// updatePositionCache 更新仓位缓存
func (s *serviceBService) updatePositionCache(ctx context.Context, userAddress string, position *model.UserPosition) {
	// 获取现有缓存
	cacheData, err := s.redisRepo.GetPositionCache(ctx, userAddress, position.ProtocolID)
	if err != nil || cacheData == nil {
		return
	}
	
	// 更新缓存中的仓位数据
	if data, ok := cacheData.Data.(map[string]interface{}); ok {
		if positions, ok := data["positions"].([]model.PositionResponse); ok {
			// 查找并更新对应的仓位
			updated := false
			for i, pos := range positions {
				if pos.PositionID == position.PositionID {
					// 更新仓位
					positions[i] = convertToPositionResponse(position)
					updated = true
					break
				}
			}
			
			// 如果没找到，添加新仓位
			if !updated {
				positions = append(positions, convertToPositionResponse(position))
			}
			
			// 重新计算总价值
			data["positions"] = positions
			data["total_value"] = calculateTotalPositionValue(positions)
			
			// 更新缓存
			s.redisRepo.SetPositionCache(ctx, userAddress, position.ProtocolID, data, s.cacheConfig.PositionTTL)
		}
	}
}

// updateUserTotalValue 更新用户总资产价值
func (s *serviceBService) updateUserTotalValue(ctx context.Context, userID uint64) {
	// 获取用户所有仓位总价值
	positions, err := s.userRepo.GetActiveUserPositions(ctx, userID)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to get user positions for total value update")
		return
	}
	
	var totalValue float64
	for _, position := range positions {
		totalValue += position.ValueUSD
	}
	
	// 更新用户表
	if err := s.userRepo.UpdateUserAssets(ctx, userID, totalValue); err != nil {
		s.logger.WithError(err).Warn("Failed to update user total assets")
	}
}

// deactivateInactivePositions 停用不活跃的仓位
func (s *serviceBService) deactivateInactivePositions(ctx context.Context, userID uint64, activePositionIDs map[string]bool) error {
	// 获取用户所有活跃仓位
	positions, err := s.userRepo.GetActiveUserPositions(ctx, userID)
	if err != nil {
		return err
	}
	
	// 停用不在activePositionIDs中的仓位
	for _, position := range positions {
		if !activePositionIDs[position.PositionID] {
			position.IsActive = false
			position.UpdatedAt = time.Now()
			if err := s.userRepo.UpdateUserPosition(ctx, &position); err != nil {
				s.logger.WithError(err).Warnf("Failed to deactivate position %s", position.PositionID)
			}
		}
	}
	
	return nil
}

// 辅助函数

// generatePositionID 生成仓位ID
func generatePositionID(position model.PositionResponse) string {
	return fmt.Sprintf("%s_%s_%s_%s", 
		position.ProtocolID,
		position.PositionType,
		strings.ToLower(position.TokenSymbol),
		uuid.New().String()[:8])
}

// generatePositionIDFromData 从数据生成仓位ID
func generatePositionIDFromData(data map[string]interface{}) string {
	protocolID, _ := data["protocol_id"].(string)
	positionType, _ := data["position_type"].(string)
	tokenSymbol, _ := data["token_symbol"].(string)
	
	if protocolID == "" || positionType == "" || tokenSymbol == "" {
		return uuid.New().String()
	}
	
	return fmt.Sprintf("%s_%s_%s_%s", 
		protocolID,
		positionType,
		strings.ToLower(tokenSymbol),
		uuid.New().String()[:8])
}

// parseAmount 解析金额字符串为浮点数
func parseAmount(amountStr string) float64 {
	if amountStr == "" {
		return 0
	}
	
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return 0
	}
	
	return amount
}

// parseFloat 解析浮点数
func parseFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

// getString 获取字符串值
func getString(data map[string]interface{}, key, defaultValue string) string {
	if value, ok := data[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return defaultValue
}

// calculateTotalPositionValue 计算总仓位价值
func calculateTotalPositionValue(positions []model.PositionResponse) float64 {
	var total float64
	for _, position := range positions {
		total += position.ValueUSD
	}
	return total
}

// convertToPositionResponse 转换数据库模型为响应模型
func convertToPositionResponse(position *model.UserPosition) model.PositionResponse {
	return model.PositionResponse{
		ProtocolID:          position.ProtocolID,
		ProtocolName:        "", // 需要从协议表获取
		PositionID:          position.PositionID,
		PositionType:        position.PositionType,
		TokenAddress:        position.TokenAddress,
		TokenSymbol:         position.TokenSymbol,
		TokenName:           "", // 需要从代币表获取
		Amount:              fmt.Sprintf("%f", position.AmountDecimal),
		AmountRaw:           position.AmountRaw,
		PriceUSD:            position.PriceUSD,
		ValueUSD:            position.ValueUSD,
		Apy:                 position.Apy,
		HealthFactor:        position.HealthFactor,
		LiquidationThreshold: position.LiquidationThreshold,
		CollateralFactor:    position.CollateralFactor,
		IsActive:            position.IsActive,
		LastUpdatedAt:       position.LastUpdatedAt.Format(time.RFC3339),
	}
}

// abs 绝对值函数
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}