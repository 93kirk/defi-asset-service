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

	"github.com/sirupsen/logrus"
)

// ServiceAClient 服务A客户端接口
type ServiceAClient interface {
	GetUserAssets(ctx context.Context, address string, chainID int) ([]model.AssetResponse, error)
	BatchGetUserAssets(ctx context.Context, addresses []string, chainID int) (map[string][]model.AssetResponse, error)
	GetTokenPrice(ctx context.Context, tokenAddress string) (float64, error)
	BatchGetTokenPrices(ctx context.Context, tokenAddresses []string) (map[string]float64, error)
}

// ServiceAService 服务A服务接口
type ServiceAService interface {
	GetUserAssets(ctx context.Context, address string, chainID int, protocolID, tokenAddress string) ([]model.AssetResponse, error)
	BatchGetUserAssets(ctx context.Context, addresses []string, chainID int) (map[string][]model.AssetResponse, error)
	GetTokenPrice(ctx context.Context, tokenAddress string) (float64, error)
	BatchGetTokenPrices(ctx context.Context, tokenAddresses []string) (map[string]float64, error)
	StoreUserAssets(ctx context.Context, userID uint64, assets []model.AssetResponse) error
}

// serviceAClient 服务A客户端实现
type serviceAClient struct {
	config     *config.ExternalServiceConfig
	httpClient *http.Client
	logger     *logrus.Logger
}

// serviceAService 服务A服务实现
type serviceAService struct {
	client       ServiceAClient
	userRepo     repository.UserRepository
	redisRepo    repository.RedisRepository
	cacheConfig  *config.CacheConfig
	logger       *logrus.Logger
}

// NewServiceAClient 创建服务A客户端
func NewServiceAClient(cfg *config.ExternalServiceConfig, logger *logrus.Logger) ServiceAClient {
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.Timeout) * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &serviceAClient{
		config:     cfg,
		httpClient: httpClient,
		logger:     logger,
	}
}

// NewServiceAService 创建服务A服务
func NewServiceAService(
	client ServiceAClient,
	userRepo repository.UserRepository,
	redisRepo repository.RedisRepository,
	cacheConfig *config.CacheConfig,
	logger *logrus.Logger,
) ServiceAService {
	return &serviceAService{
		client:      client,
		userRepo:    userRepo,
		redisRepo:   redisRepo,
		cacheConfig: cacheConfig,
		logger:      logger,
	}
}

// GetUserAssets 获取用户资产（服务A）
func (c *serviceAClient) GetUserAssets(ctx context.Context, address string, chainID int) ([]model.AssetResponse, error) {
	// 构建请求URL
	url := fmt.Sprintf("%s/users/%s/assets", c.config.BaseURL, address)
	
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
		return nil, fmt.Errorf("service A returned status %d", resp.StatusCode)
	}
	
	// 解析响应
	var apiResp model.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// 检查API响应码
	if apiResp.Code != model.CodeSuccess {
		return nil, fmt.Errorf("service A error: %s", apiResp.Message)
	}
	
	// 提取资产数据
	var result struct {
		Address string                `json:"address"`
		ChainID int                   `json:"chain_id"`
		Assets  []model.AssetResponse `json:"assets"`
	}
	
	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}
	
	if err := json.Unmarshal(dataBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal assets: %w", err)
	}
	
	return result.Assets, nil
}

// BatchGetUserAssets 批量获取用户资产
func (c *serviceAClient) BatchGetUserAssets(ctx context.Context, addresses []string, chainID int) (map[string][]model.AssetResponse, error) {
	// 服务A可能不支持批量查询，这里实现为串行查询
	// 在实际项目中，如果服务A支持批量API，应该使用批量API
	
	result := make(map[string][]model.AssetResponse)
	
	for _, address := range addresses {
		assets, err := c.GetUserAssets(ctx, address, chainID)
		if err != nil {
			c.logger.WithError(err).Warnf("Failed to get assets for address %s", address)
			continue
		}
		result[address] = assets
		
		// 添加小延迟避免速率限制
		time.Sleep(100 * time.Millisecond)
	}
	
	return result, nil
}

// GetTokenPrice 获取代币价格
func (c *serviceAClient) GetTokenPrice(ctx context.Context, tokenAddress string) (float64, error) {
	// 构建请求URL
	url := fmt.Sprintf("%s/tokens/%s/price", c.config.BaseURL, tokenAddress)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}
	
	// 添加认证头
	if c.config.APIKey != "" {
		req.Header.Add("X-API-Key", c.config.APIKey)
	}
	
	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("service A returned status %d", resp.StatusCode)
	}
	
	// 解析响应
	var apiResp model.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// 检查API响应码
	if apiResp.Code != model.CodeSuccess {
		return 0, fmt.Errorf("service A error: %s", apiResp.Message)
	}
	
	// 提取价格数据
	var priceData struct {
		Price float64 `json:"price"`
	}
	
	dataBytes, err := json.Marshal(apiResp.Data)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal data: %w", err)
	}
	
	if err := json.Unmarshal(dataBytes, &priceData); err != nil {
		return 0, fmt.Errorf("failed to unmarshal price: %w", err)
	}
	
	return priceData.Price, nil
}

// BatchGetTokenPrices 批量获取代币价格
func (c *serviceAClient) BatchGetTokenPrices(ctx context.Context, tokenAddresses []string) (map[string]float64, error) {
	// 服务A可能不支持批量查询，这里实现为串行查询
	// 在实际项目中，如果服务A支持批量API，应该使用批量API
	
	result := make(map[string]float64)
	
	for _, tokenAddress := range tokenAddresses {
		price, err := c.GetTokenPrice(ctx, tokenAddress)
		if err != nil {
			c.logger.WithError(err).Warnf("Failed to get price for token %s", tokenAddress)
			continue
		}
		result[tokenAddress] = price
		
		// 添加小延迟避免速率限制
		time.Sleep(50 * time.Millisecond)
	}
	
	return result, nil
}

// GetUserAssets 获取用户资产（带缓存和过滤）
func (s *serviceAService) GetUserAssets(ctx context.Context, address string, chainID int, protocolID, tokenAddress string) ([]model.AssetResponse, error) {
	// 1. 从服务A获取实时资产数据
	assets, err := s.client.GetUserAssets(ctx, address, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assets from service A: %w", err)
	}
	
	// 2. 应用过滤
	filteredAssets := s.filterAssets(assets, protocolID, tokenAddress)
	
	// 3. 异步存储到数据库（不阻塞响应）
	go func() {
		storageCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		
		// 获取或创建用户
		user, err := s.userRepo.GetUserByAddress(storageCtx, address, chainID)
		if err != nil {
			s.logger.WithError(err).Error("Failed to get user for asset storage")
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
		
		// 存储资产数据
		if err := s.StoreUserAssets(storageCtx, user.ID, filteredAssets); err != nil {
			s.logger.WithError(err).Error("Failed to store user assets")
		}
	}()
	
	return filteredAssets, nil
}

// BatchGetUserAssets 批量获取用户资产
func (s *serviceAService) BatchGetUserAssets(ctx context.Context, addresses []string, chainID int) (map[string][]model.AssetResponse, error) {
	// 从服务A批量获取资产数据
	assetsMap, err := s.client.BatchGetUserAssets(ctx, addresses, chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to batch get assets from service A: %w", err)
	}
	
	// 异步存储到数据库
	go func() {
		storageCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		
		for address, assets := range assetsMap {
			// 获取或创建用户
			user, err := s.userRepo.GetUserByAddress(storageCtx, address, chainID)
			if err != nil {
				s.logger.WithError(err).Errorf("Failed to get user %s for asset storage", address)
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
			
			// 存储资产数据
			if err := s.StoreUserAssets(storageCtx, user.ID, assets); err != nil {
				s.logger.WithError(err).Errorf("Failed to store assets for user %s", address)
			}
		}
	}()
	
	return assetsMap, nil
}

// GetTokenPrice 获取代币价格（带缓存）
func (s *serviceAService) GetTokenPrice(ctx context.Context, tokenAddress string) (float64, error) {
	// 1. 检查Redis缓存
	cacheKey := fmt.Sprintf("price:%s", tokenAddress)
	if priceData, err := s.redisRepo.GetPriceCache(ctx, tokenAddress); err == nil && priceData != nil {
		// 更新缓存统计
		s.redisRepo.UpdateCacheStats(ctx, cacheKey, "price", tokenAddress, true, s.cacheConfig.PriceTTL)
		return priceData.Price, nil
	}
	
	// 2. 从服务A获取价格
	price, err := s.client.GetTokenPrice(ctx, tokenAddress)
	if err != nil {
		// 检查是否有空值缓存（防缓存穿透）
		emptyKey := fmt.Sprintf("price:%s", tokenAddress)
		if exists, err := s.redisRepo.GetEmptyCache(ctx, emptyKey); err == nil && exists {
			return 0, fmt.Errorf("price not available (cached empty)")
		}
		
		// 设置空值缓存
		s.redisRepo.SetEmptyCache(ctx, emptyKey, s.cacheConfig.EmptyTTL)
		return 0, fmt.Errorf("failed to get price from service A: %w", err)
	}
	
	// 3. 存储到Redis缓存
	if err := s.redisRepo.SetPriceCache(ctx, tokenAddress, price, "service_a", s.cacheConfig.PriceTTL); err != nil {
		s.logger.WithError(err).Warn("Failed to cache price")
	}
	
	// 4. 更新缓存统计
	s.redisRepo.UpdateCacheStats(ctx, cacheKey, "price", tokenAddress, false, s.cacheConfig.PriceTTL)
	
	return price, nil
}

// BatchGetTokenPrices 批量获取代币价格（带缓存）
func (s *serviceAService) BatchGetTokenPrices(ctx context.Context, tokenAddresses []string) (map[string]float64, error) {
	result := make(map[string]float64)
	var missingTokens []string
	
	// 1. 批量检查Redis缓存
	cachedPrices, err := s.redisRepo.BatchGetPriceCache(ctx, tokenAddresses)
	if err != nil {
		s.logger.WithError(err).Warn("Failed to batch get price cache")
		// 继续从服务A获取
		missingTokens = tokenAddresses
	} else {
		// 记录缓存命中的价格
		for tokenAddress, priceData := range cachedPrices {
			if priceData != nil {
				result[tokenAddress] = priceData.Price
				// 更新缓存统计
				cacheKey := fmt.Sprintf("price:%s", tokenAddress)
				s.redisRepo.UpdateCacheStats(ctx, cacheKey, "price", tokenAddress, true, s.cacheConfig.PriceTTL)
			} else {
				missingTokens = append(missingTokens, tokenAddress)
			}
		}
	}
	
	// 2. 从服务A获取缺失的价格
	if len(missingTokens) > 0 {
		prices, err := s.client.BatchGetTokenPrices(ctx, missingTokens)
		if err != nil {
			return nil, fmt.Errorf("failed to batch get prices from service A: %w", err)
		}
		
		// 3. 存储到Redis缓存并更新结果
		for tokenAddress, price := range prices {
			result[tokenAddress] = price
			
			// 存储到缓存
			if err := s.redisRepo.SetPriceCache(ctx, tokenAddress, price, "service_a", s.cacheConfig.PriceTTL); err != nil {
				s.logger.WithError(err).Warnf("Failed to cache price for token %s", tokenAddress)
			}
			
			// 更新缓存统计
			cacheKey := fmt.Sprintf("price:%s", tokenAddress)
			s.redisRepo.UpdateCacheStats(ctx, cacheKey, "price", tokenAddress, false, s.cacheConfig.PriceTTL)
		}
	}
	
	return result, nil
}

// StoreUserAssets 存储用户资产到数据库
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
	if err := s.userRepo.BatchCreateUserAssets(ctx, userAssets); err