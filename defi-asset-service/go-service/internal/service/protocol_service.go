package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"defi-asset-service/internal/config"
	"defi-asset-service/internal/model"
	"defi-asset-service/internal/repository"

	"github.com/sirupsen/logrus"
)

// ProtocolClient 协议客户端接口
type ProtocolClient interface {
	FetchProtocols(ctx context.Context) ([]model.Protocol, error)
	FetchProtocolDetails(ctx context.Context, protocolID string) (*model.Protocol, error)
	FetchProtocolTokens(ctx context.Context, protocolID string, chainID int) ([]model.ProtocolToken, error)
}

// ProtocolService 协议服务接口
type ProtocolService interface {
	GetProtocols(ctx context.Context, query model.ProtocolQuery) (*model.ProtocolListResponse, error)
	GetProtocol(ctx context.Context, protocolID string) (*model.ProtocolResponse, error)
	GetProtocolTokens(ctx context.Context, protocolID string, query model.TokenQuery) ([]model.TokenResponse, error)
	SyncProtocols(ctx context.Context, forceFullSync bool, protocolIDs []string) (*model.SyncResponse, error)
	GetSyncStatus(ctx context.Context, syncID string) (*model.SyncStatusResponse, error)
	UpdateProtocolCache(ctx context.Context, protocolID string) error
	CleanupProtocolCache(ctx context.Context) error
}

// protocolClient 协议客户端实现（从DeBank获取数据）
type protocolClient struct {
	config     *config.ExternalServiceConfig
	httpClient *http.Client
	logger     *logrus.Logger
}

// protocolService 协议服务实现
type protocolService struct {
	client        ProtocolClient
	protocolRepo  repository.ProtocolRepository
	redisRepo     repository.RedisRepository
	cacheConfig   *config.CacheConfig
	cronConfig    *config.CronConfig
	logger        *logrus.Logger
}

// NewProtocolClient 创建协议客户端
func NewProtocolClient(cfg *config.ExternalServiceConfig, logger *logrus.Logger) ProtocolClient {
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.Timeout) * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &protocolClient{
		config:     cfg,
		httpClient: httpClient,
		logger:     logger,
	}
}

// NewProtocolService 创建协议服务
func NewProtocolService(
	client ProtocolClient,
	protocolRepo repository.ProtocolRepository,
	redisRepo repository.RedisRepository,
	cacheConfig *config.CacheConfig,
	cronConfig *config.CronConfig,
	logger *logrus.Logger,
) ProtocolService {
	return &protocolService{
		client:       client,
		protocolRepo: protocolRepo,
		redisRepo:    redisRepo,
		cacheConfig:  cacheConfig,
		cronConfig:   cronConfig,
		logger:       logger,
	}
}

// FetchProtocols 从DeBank获取协议列表
func (c *protocolClient) FetchProtocols(ctx context.Context) ([]model.Protocol, error) {
	// 构建请求URL
	url := fmt.Sprintf("%s/protocol/all", c.config.BaseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
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
		return nil, fmt.Errorf("debank returned status %d", resp.StatusCode)
	}
	
	// 解析响应
	var apiResp struct {
		Code    int                    `json:"code"`
		Message string                 `json:"message"`
		Data    []*DebankProtocol      `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// 检查API响应码
	if apiResp.Code != 0 {
		return nil, fmt.Errorf("debank error: %s", apiResp.Message)
	}
	
	// 转换为内部协议模型
	var protocols []model.Protocol
	for _, debankProtocol := range apiResp.Data {
		protocol, err := c.convertDebankProtocol(debankProtocol)
		if err != nil {
			c.logger.WithError(err).Warnf("Failed to convert protocol %s", debankProtocol.ID)
			continue
		}
		protocols = append(protocols, *protocol)
	}
	
	return protocols, nil
}

// FetchProtocolDetails 获取协议详情
func (c *protocolClient) FetchProtocolDetails(ctx context.Context, protocolID string) (*model.Protocol, error) {
	// 构建请求URL
	url := fmt.Sprintf("%s/protocol", c.config.BaseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// 添加查询参数
	q := req.URL.Query()
	q.Add("id", protocolID)
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
		return nil, fmt.Errorf("debank returned status %d", resp.StatusCode)
	}
	
	// 解析响应
	var apiResp struct {
		Code    int               `json:"code"`
		Message string            `json:"message"`
		Data    *DebankProtocol   `json:"data"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// 检查API响应码
	if apiResp.Code != 0 {
		return nil, fmt.Errorf("debank error: %s", apiResp.Message)
	}
	
	// 转换为内部协议模型
	return c.convertDebankProtocol(apiResp.Data)
}

// FetchProtocolTokens 获取协议代币
func (c *protocolClient) FetchProtocolTokens(ctx context.Context, protocolID string, chainID int) ([]model.ProtocolToken, error) {
	// 注意：DeBank API可能不直接提供协议代币列表
	// 这里需要根据实际情况实现
	// 暂时返回空列表
	return []model.ProtocolToken{}, nil
}

// GetProtocols 获取协议列表（带缓存）
func (s *protocolService) GetProtocols(ctx context.Context, query model.ProtocolQuery) (*model.ProtocolListResponse, error) {
	// 1. 检查Redis缓存
	cacheKey := fmt.Sprintf("protocols:list:%s:%d", query.Category, query.Page)
	if cacheData, err := s.redisRepo.GetProtocolListCache(ctx, query.Category, query.Page); err == nil && cacheData != nil {
		if data, ok := cacheData.Data.(*model.ProtocolListResponse); ok {
			// 更新缓存统计
			s.redisRepo.UpdateCacheStats(ctx, cacheKey, "protocol_list", query.Category, true, s.cacheConfig.ProtocolTTL)
			return data, nil
		}
	}
	
	// 2. 从数据库获取协议列表
	protocols, total, err := s.protocolRepo.ListProtocols(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list protocols: %w", err)
	}
	
	// 3. 转换为响应格式
	protocolResponses := make([]model.ProtocolResponse, len(protocols))
	for i, protocol := range protocols {
		protocolResponses[i] = s.convertToProtocolResponse(&protocol)
	}
	
	// 4. 构建响应
	response := &model.ProtocolListResponse{
		Protocols: protocolResponses,
		Pagination: model.Pagination{
			Page:       query.Page,
			PageSize:   query.PageSize,
			Total:      int(total),
			TotalPages: (int(total) + query.PageSize - 1) / query.PageSize,
		},
	}
	
	// 5. 存储到Redis缓存
	if err := s.redisRepo.SetProtocolListCache(ctx, query.Category, query.Page, response, s.cacheConfig.ProtocolTTL); err != nil {
		s.logger.WithError(err).Warn("Failed to cache protocol list")
	}
	
	// 6. 更新缓存统计
	s.redisRepo.UpdateCacheStats(ctx, cacheKey, "protocol_list", query.Category, false, s.cacheConfig.ProtocolTTL)
	
	return response, nil
}

// GetProtocol 获取协议详情（带缓存）
func (s *protocolService) GetProtocol(ctx context.Context, protocolID string) (*model.ProtocolResponse, error) {
	// 1. 检查Redis缓存
	cacheKey := fmt.Sprintf("protocol:%s", protocolID)
	if cacheData, err := s.redisRepo.GetProtocolCache(ctx, protocolID); err == nil && cacheData != nil {
		if data, ok := cacheData.Data.(*model.ProtocolResponse); ok {
			// 检查版本
			if cacheData.Version > 0 {
				// 更新缓存统计
				s.redisRepo.UpdateCacheStats(ctx, cacheKey, "protocol", protocolID, true, s.cacheConfig.ProtocolTTL)
				return data, nil
			}
		}
	}
	
	// 2. 从数据库获取协议
	protocol, err := s.protocolRepo.GetProtocolByID(ctx, protocolID)
	if err != nil {
		return nil, fmt.Errorf("failed to get protocol: %w", err)
	}
	
	if protocol == nil {
		// 检查是否有空值缓存
		emptyKey := fmt.Sprintf("protocol:%s", protocolID)
		if exists, err := s.redisRepo.GetEmptyCache(ctx, emptyKey); err == nil && exists {
			return nil, fmt.Errorf("protocol not found (cached empty)")
		}
		
		// 设置空值缓存
		s.redisRepo.SetEmptyCache(ctx, emptyKey, s.cacheConfig.EmptyTTL)
		return nil, fmt.Errorf("protocol not found")
	}
	
	// 3. 获取协议代币
	tokens, err := s.protocolRepo.GetProtocolTokens(ctx, protocolID, model.TokenQuery{})
	if err != nil {
		s.logger.WithError(err).Warnf("Failed to get tokens for protocol %s", protocolID)
	}
	
	// 4. 获取协议统计
	stats, err := s.protocolRepo.GetProtocolStatistics(ctx, protocolID)
	if err != nil {
		s.logger.WithError(err).Warnf("Failed to get statistics for protocol %s", protocolID)
	}
	
	// 5. 转换为响应格式
	response := s.convertToProtocolResponse(protocol)
	response.Tokens = s.convertToTokenResponses(tokens)
	if stats != nil {
		response.Statistics = *stats
	}
	
	// 6. 存储到Redis缓存
	if err := s.redisRepo.SetProtocolCache(ctx, protocolID, response, s.cacheConfig.ProtocolTTL, int(protocol.SyncVersion)); err != nil {
		s.logger.WithError(err).Warn("Failed to cache protocol")
	}
	
	// 7. 更新缓存统计
	s.redisRepo.UpdateCacheStats(ctx, cacheKey, "protocol", protocolID, false, s.cacheConfig.ProtocolTTL)
	
	return &response, nil
}

// GetProtocolTokens 获取协议代币列表
func (s *protocolService) GetProtocolTokens(ctx context.Context, protocolID string, query model.TokenQuery) ([]model.TokenResponse, error) {
	// 从数据库获取协议代币
	tokens, err := s.protocolRepo.GetProtocolTokens(ctx, protocolID, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get protocol tokens: %w", err)
	}
	
	// 转换为响应格式
	return s.convertToTokenResponses(tokens), nil
}

// SyncProtocols 同步协议数据
func (s *protocolService) SyncProtocols(ctx context.Context, forceFullSync bool, protocolIDs []string) (*model.SyncResponse, error) {
	// 1. 创建同步记录
	syncID := fmt.Sprintf("sync_%d", time.Now().Unix())
	syncRecord := &model.SyncRecord{
		SyncType:   "protocol_metadata",
		SyncSource: "debank",
		TargetID:   strings.Join(protocolIDs, ","),
		Status:     "pending",
		StartedAt:  time.Now(),
		CreatedAt:  time.Now(),
	}
	
	if err := s.protocolRepo.CreateSyncRecord(ctx, syncRecord); err != nil {
		return nil, fmt.Errorf("failed to create sync record: %w", err)
	}
	
	// 2. 异步执行同步
	go s.executeProtocolSync(context.Background(), syncRecord.ID, forceFullSync, protocolIDs)
	
	// 3. 返回同步响应
	return &model.SyncResponse{
		SyncID:        syncID,
		Status:        "started",
		EstimatedTime: 300, // 预估5分钟
		StartedAt:     time.Now().Format(time.RFC3339),
	}, nil
}

// GetSyncStatus 获取同步状态
func (s *protocolService) GetSyncStatus(ctx context.Context, syncID string) (*model.SyncStatusResponse, error) {
	// 从数据库获取同步记录
	record, err := s.protocolRepo.GetSyncRecord(ctx, syncID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync record: %w", err)
	}
	
	if record == nil {
		return nil, fmt.Errorf("sync record not found")
	}
	
	// 计算进度
	progress := "0%"
	if record.TotalCount > 0 {
		progress = fmt.Sprintf("%.2f%%", float64(record.SuccessCount+record.FailedCount)/float64(record.TotalCount)*100)
	}
	
	// 预估完成时间
	var estimatedFinishAt string
	if record.Status == "processing" && record.TotalCount > 0 {
		processedCount := record.SuccessCount + record.FailedCount
		if processedCount > 0 {
			elapsedTime := time.Since(record.StartedAt)
			timePerItem := elapsedTime / time.Duration(processedCount)
			remainingItems := record.TotalCount - processedCount
			estimatedFinishTime := time.Now().Add(timePerItem * time.Duration(remainingItems))
			estimatedFinishAt = estimatedFinishTime.Format(time.RFC3339)
		}
	}
	
	return &model.SyncStatusResponse{
		SyncID:        syncID,
		SyncType:      record.SyncType,
		SyncSource:    record.SyncSource,
		Status:        record.Status,
		TotalCount:    record.TotalCount,
		SuccessCount:  record.SuccessCount,
		FailedCount:   record.FailedCount,
		Progress:      progress,
		StartedAt:     record.StartedAt.Format(time.RFC3339),
		EstimatedFinishAt: estimatedFinishAt,
	}, nil
}

// UpdateProtocolCache 更新协议缓存
func (s *protocolService) UpdateProtocolCache(ctx context.Context, protocolID string) error {
	// 获取协议数据
	protocol, err := s.protocolRepo.GetProtocolByID(ctx, protocolID)
	if err != nil {
		return fmt.Errorf("failed to get protocol: %w", err)
	}
	
	if protocol == nil {
		// 删除缓存
		s.redisRepo.DeleteProtocolCache(ctx, protocolID)
		return nil
	}
	
	// 转换为响应格式并更新缓存
	response := s.convertToProtocolResponse(protocol)
	if err := s.redisRepo.SetProtocolCache(ctx, protocolID, &response, s.cacheConfig.ProtocolTTL, int(protocol.SyncVersion)); err != nil {
		return fmt.Errorf("failed to update protocol cache: %w", err)
	}
	
	return nil
}

// CleanupProtocolCache 清理协议缓存
func (s *protocolService) CleanupProtocolCache(ctx context.Context) error {
	// 清理过期的协议缓存
	pattern := fmt.Sprintf("%s:protocol:*", s.redisRepo.(*redisRepository).prefix)
	keys, err := s.redisRepo.GetKeysByPattern(ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to get cache keys: %w", err)
	}
	
	for _, key := range keys {
		// 检查缓存是否过期
		cacheData, err := s.redisRepo.(*redisRepository).getCacheData(ctx, key)
		if err != nil || cacheData == nil {
			continue
		}
		
		// 如果过期，删除缓存
		if cacheData.ExpiresAt > 0 && time.Now().Unix() > cacheData.ExpiresAt {
			s.redisRepo.Delete(ctx, key)
		}
	}
	
	return nil
}

// executeProtocolSync 执行协议同步（异步）
func (s *protocolService) executeProtocolSync(ctx context.Context, syncRecordID uint64, forceFullSync bool, protocolIDs []