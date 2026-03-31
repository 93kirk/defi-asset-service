package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"defi-asset-service/internal/config"
	"defi-asset-service/internal/model"
	"defi-asset-service/internal/repository"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// ServiceBClient 服务B客户端接口
type ServiceBClient interface {
	GetUserPositions(ctx context.Context, address string, chainID int) ([]model.PositionResponse, error)
	BatchGetUserPositions(ctx context.Context, addresses []string, chainID int) (map[string][]model.PositionResponse, error)
}

// ServiceBService 服务B服务接口
type ServiceBService interface {
	GetUserPositions(ctx context.Context, address string, chainID int, protocolID, positionType string, refresh bool) ([]model.PositionResponse, error)
	BatchGetUserPositions(ctx context.Context, addresses []string, chainID int) (map[string][]model.PositionResponse, error)
	ProcessPositionUpdate(ctx context.Context, message *model.PositionUpdateMessage) error
	StoreUserPositions(ctx context.Context, userID uint64, positions []model.PositionResponse) error
	GetCachedUserPositions(ctx context.Context, address string, chainID int, protocolID string) (*model.CacheData, error)
}

// serviceBClient 服务B客户端实现
type serviceBClient struct {
	config     *config.ExternalServiceConfig
	httpClient *http.Client
	logger     *logrus.Logger
}

// serviceBService 服务B服务实现
type serviceBService struct {
	client       ServiceBClient
	userRepo     repository.UserRepository
	redisRepo    repository.RedisRepository
	cacheConfig  *config.CacheConfig
	businessConfig *config.BusinessConfig
	logger       *logrus.Logger
}

// NewServiceBClient 创建服务B客户端
func NewServiceBClient(cfg *config.ExternalServiceConfig, logger *logrus.Logger) ServiceBClient {
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.Timeout) * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &serviceBClient{
		config:     cfg,
		httpClient: httpClient,
		logger:     logger,
	}
}

// NewServiceBService 创建服务B服务
func NewServiceBService(
	client ServiceBClient,
	userRepo repository.UserRepository,
	redisRepo repository.RedisRepository,
	cacheConfig *config.CacheConfig,
	businessConfig *config.BusinessConfig,
	logger *logrus.Logger,
) ServiceBService {
	return &serviceBService{
		client:         client,
		userRepo:       userRepo,
		redisRepo:      redisRepo,
		cacheConfig:    cacheConfig,
		businessConfig: businessConfig,
		logger:         logger,
	}
}

// GetUserPositions 获取用户仓位（服务B）
func (c *serviceBClient) GetUserPositions(ctx context.Context, address string, chainID int) ([]model.PositionResponse, error) {
	// 构建请求URL
	url := fmt.Sprintf("%s/users/%s/positions", c.config.BaseURL, address)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// 添加查询参数
	q := req.URL.Query()
	q.Add("chain_id", fmt.Sprintf("%d", chainID))
	req.URL.RawQuery = q.Encode()
	
	// 添加认证头
	if c.config.APIKey != "" {
		req.Header.Add("X-API-Key", c.config.APIKey)
	}
	
	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service B returned status %d", resp.StatusCode)
	}
	
	// 解析响应
	var apiResp model.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// 检查API响应码
	if apiResp.Code != model.CodeSuccess {
		return nil, fmt.Errorf("service B error: %s", apiResp.Message)
	}
	
	// 提取仓位数据
	var result struct {
		Address   string                  `json:"address"`
		ChainID   int                     `json:"chain_id"`
		Positions []model.PositionResponse `json:"positions"`
	}
	
	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}
	
	if err := json.Unmarshal(dataBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal positions: %w", err)
	}
	
	return result.Positions, nil
}

// BatchGetUserPositions 批量获取用户仓位
func (c *serviceBClient) BatchGetUserPositions(ctx context.Context, addresses []string, chainID int) (map[string][]model.PositionResponse, error) {
	// 服务B可能不支持批量查询，这里实现为串行查询
	// 在实际项目中，如果服务B支持批量API，应该使用批量API
	
	result := make(map[string][]model.PositionResponse)
	
	for _, address := range addresses {
		positions, err := c.GetUserPositions(ctx, address, chainID)
		if err != nil {
			c.logger.WithError(err).Warnf("Failed to get positions for address %s", address)
			continue
		}
		result[address] = positions
		
		// 添加小延迟避免速率限制
		time.Sleep(100 * time.Millisecond)
	}
	
	return result, nil
}

// GetUserPositions 获取用户仓位（带缓存）
func (s *serviceBService) GetUserPositions(ctx context.Context, address string, chainID int, protocolID, positionType string, refresh bool) ([]model.PositionResponse, error) {
	// 1. 如果不是强制刷新，先检查Redis缓存
	if !refresh {
		cacheData, err := s.GetCachedUserPositions(ctx, address, chainID, protocolID)
		if err == nil && cacheData != nil {
			// 缓存命中，直接返回
			if positions, ok := cacheData.Data.([]model.PositionResponse); ok {
				// 应用过滤
				filteredPositions := s.filterPositions(positions, protocolID, positionType)
				return filteredPositions, nil
			}
		}
	}
	
	// 2. 从服务B获取仓位数据
	positions, err := s.client.GetUserPositions(ctx, address, chainID)
	if err != nil {
		// 检查是否有空值缓存（防缓存穿透）
		emptyKey := fmt.Sprintf("position:%s:%s", address, protocolID)
		if exists, err := s.redisRepo.GetEmptyCache(ctx, emptyKey); err == nil && exists {
			return nil, fmt.Errorf("positions not available (cached empty)")
		}
		
		// 设置空值缓存
		s.redisRepo.SetEmptyCache(ctx, emptyKey, s.cacheConfig.EmptyTTL)
		return nil, fmt.Errorf("failed to get positions from service B: %w", err)
	}
	
	// 3. 应用过滤
	filteredPositions := s.filterPositions(positions, protocolID, positionType)
	
	// 4. 存储到Redis缓存
	cacheKey := fmt.Sprintf("position:%s:%s", address, protocolID)
	cacheData := map[string]interface{}{
		"positions": filteredPositions,
		"total_value": calculateTotalPositionValue(filteredPositions),
	}
	
	if err := s.redisRepo.SetPositionCache(ctx, address, protocolID, cacheData, s.cacheConfig.PositionTTL); err != nil {
		s.logger.WithError(err).Warn("Failed to cache positions")
	}
	
	// 5. 更新缓存统计
	s.redisRepo.UpdateCacheStats(ctx, cacheKey, "position", address, false, s.cacheConfig.PositionTTL)
	
	// 6. 异步存储到数据库
	go func() {
		storageCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		// 获取或创建用户
		user, err := s.userRepo.GetUserByAddress(storageCtx, address, chainID)
		if err != nil {
			s.logger.WithError(err).Error("Failed to get user for position storage")
			return
		}
		
		if user == nil {
			// 创建新用户
			user = &model.User{
				Address: address,
				ChainID: chainID,
			}
			if err := s.userRepo.CreateUser(storageCtx, user); err != nil {
				s.logger.WithError(err).Error("Failed to create user")
				return
			}
		}
		
		// 存储仓位数据
		if err := s.StoreUserPositions(storageCtx, user.ID, filteredPositions); err != nil {
			s.logger.WithError(err).Error("Failed to store user positions")
		}
	}()
	
	return filteredPositions, nil
}

// BatchGetUserPositions 批量获取用户仓位
func (s *serviceBService) BatchGetUserPositions(ctx context.Context, addresses []string, chainID int) (map[string][]model.PositionResponse, error) {
	// 从服务B批量获取仓位数据
	positionsMap, err := s.client.BatchGetUserPositions(ctx, addresses, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to batch get positions from service B: %w", err)
	}
	
	// 异步存储到数据库和缓存
	go func() {
		storageCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		
		for address, positions := range positionsMap {
			// 获取或创建用户
			user, err := s.userRepo.GetUserByAddress(storageCtx, address, chainID)
			if err != nil {
				s.logger.WithError(err).Errorf("Failed to get user %s for position storage", address)
				continue
			}
			
			if user == nil {
				// 创建新用户
				user = &model.User{
					Address: address,
					ChainID: chainID,
				}
				if err := s.userRepo.CreateUser(storageCtx, user); err != nil {
					s.logger.WithError(err).Errorf("Failed to create user %s", address)
					continue
				}
			}
			
			// 存储仓位数据
			if err := s.StoreUserPositions(storageCtx, user.ID, positions); err != nil {
				s.logger.WithError(err).Errorf("Failed to store positions for user %s", address)
			}
			
			// 存储到Redis缓存
			cacheData := map[string]interface{}{
				"positions": positions,
				"total_value": calculateTotalPositionValue(positions),
			}
			
			if err := s.redisRepo.SetPositionCache(ctx, address, "", cacheData, s.cacheConfig.PositionTTL); err != nil {
				s.logger.WithError(err).Warnf("Failed to cache positions for user %s", address)
			}
		}
	}()
	
	return positionsMap, nil
}

// ProcessPositionUpdate 处理仓位更新消息
func (s *serviceBService) ProcessPositionUpdate(ctx context.Context, message *model.PositionUpdateMessage) error {
	// 1. 验证消息
	if err := s.validatePositionUpdate(message); err != nil {
		return fmt.Errorf("invalid position update message: %w", err)
	}
	
	// 2. 获取用户
	user, err := s.userRepo.GetUserByAddress(ctx, message.UserAddress, s.businessConfig.DefaultChainID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	
	if user == nil {
		// 创建新用户
		user = &model.User{
			Address: message.UserAddress,
			ChainID: s.businessConfig.DefaultChainID,
		}
		if err := s.userRepo.CreateUser(ctx, user); err != nil {
			return fmt.Errorf("failed to create user: %w", err)
		}
	}
	
	// 3. 解析仓位数据
	position, err := s.parsePositionData(user.ID, message)
	if err != nil {
		return fmt.Errorf("failed to parse position data: %w", err)
	}
	
	// 4. 检查仓位是否存在
	existingPosition, err := s.userRepo.GetUserPosition(ctx, user.ID, position.ProtocolID, position.PositionID)
	if err != nil {
		return fmt.Errorf("failed to check existing position: %w", err)
	}
	
	// 5. 创建或更新仓位
	if existingPosition == nil {
		// 创建新仓位
		if err := s.userRepo.CreateUserPosition(ctx, position); err != nil {
			return fmt.Errorf("failed to create position: %w", err)
		}
	} else {
		// 更新现有仓位
		position.ID = existingPosition.ID
		position.CreatedAt = existingPosition.CreatedAt
		position.LastUpdatedBy = "queue_worker"
		
		// 检查是否需要更新（避免频繁更新）
		if s.shouldUpdatePosition(existingPosition, position) {
			if err := s.userRepo.UpdateUserPosition(ctx, position); err != nil {
				return fmt.Errorf("failed to update position: %w", err)
			}
		}
	}
	
	// 6. 更新Redis缓存
	s.updatePositionCache(ctx, message.UserAddress, position)
	
	// 7. 更新用户总资产价值
	s.updateUserTotalValue(ctx, user.ID)
	
	return nil
}

// StoreUserPositions 存储用户仓位到数据库
func (s *serviceBService) StoreUserPositions(ctx context.Context, userID uint64, positions []model.PositionResponse) error {
	if len(positions) == 0 {
		// 如果没有仓位，停用所有现有仓位
		return s.userRepo.DeactivateOldPositions(ctx, userID, time.Now().Add(-24*time.Hour))
	}
	
	// 转换仓位响应为数据库模型
	var userPositions []model.UserPosition
	now := time.Now()
	activePositionIDs := make(map[string]bool)
	
	for _, position := range positions {
		positionID := generatePositionID(position)
		activePositionIDs[positionID] = true
		
		userPosition := model.UserPosition{
			UserID:             userID,
			ProtocolID:         position.ProtocolID,
			PositionID:         positionID,
			PositionType:       position.PositionType,
			TokenAddress:       position.TokenAddress,
			TokenSymbol:        position.TokenSymbol,
			AmountRaw:          position.AmountRaw,
			AmountDecimal:      parseAmount(position.Amount),
			PriceUSD:           position.PriceUSD,
			ValueUSD:           position.ValueUSD,
			Apy:                position.Apy,
			HealthFactor:       position.HealthFactor,
			LiquidationThreshold: position.LiquidationThreshold,
			CollateralFactor:   position.CollateralFactor,
			PositionData:       nil, // 可以存储原始数据
			IsActive:           true,
			LastUpdatedBy:      "service_b",
			LastUpdatedAt:      now,
			CreatedAt:          now,
			UpdatedAt:          now,
		}
		userPositions = append(userPositions, userPosition)
	}
	
	// 批量创建或更新仓位
	if err := s.userRepo.BatchUpdateUserPositions(ctx, userPositions); err != nil {
		return fmt.Errorf("failed to batch update positions: %w", err)
	}
	
	// 停用不再活跃的仓位
	if err := s.deactivateInactivePositions(ctx, userID, activePositionIDs); err != nil {
		s.logger.WithError(err).Warn("Failed to deactivate inactive positions")
	}
	
	// 更新用户总资产价值
	totalValue := calculateTotalPositionValue(positions)
	if err := s.userRepo.UpdateUserAssets(ctx, userID, totalValue); err != nil {
		s.logger.WithError(err).Warn("Failed to update user total assets")
	}
	
	return nil
}

// GetCachedUserPositions 获取缓存的用户仓位
func (s *serviceBService) GetCachedUserPositions(ctx context.Context, address string, chainID int, protocolID string) (*model.CacheData, error) {
	cacheKey := fmt.Sprintf("position:%s:%s", address, protocolID)
	
	// 检查布隆过滤器（如果启用）
	// if exists, err := s.redisRepo.BloomExists(ctx, "positions", address); err == nil && !exists {
	//     return nil, nil
	// }
	
	cacheData, err := s.redisRepo.GetPositionCache(ctx, address, protocolID)
	if err != nil {
		return nil, err
	}
	
	if cacheData != nil {
		// 更新缓存统计
		s.redisRepo.UpdateCacheStats(ctx, cacheKey, "position", address, true, s.cacheConfig.PositionTTL)
	}
	
	return cacheData, nil
}

// filterPositions 过滤仓位
func (s *serviceBService) filterPositions(positions []model.PositionResponse, protocol