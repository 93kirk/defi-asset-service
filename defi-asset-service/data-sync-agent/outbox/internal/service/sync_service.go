package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/openclaw/defi-asset-service/data-sync-agent/internal/config"
	"github.com/openclaw/defi-asset-service/data-sync-agent/internal/models"
	"github.com/openclaw/defi-asset-service/data-sync-agent/internal/parser"
	"github.com/openclaw/defi-asset-service/data-sync-agent/internal/scraper"
)

// SyncService 同步服务
type SyncService struct {
	db           *gorm.DB
	config       *config.Config
	logger       *slog.Logger
	scraper      *scraper.DeBankScraper
	parser       *parser.ProtocolParser
	mu           sync.RWMutex
	stats        *SyncStats
}

// SyncStats 同步统计
type SyncStats struct {
	Total   int `json:"total"`
	Created int `json:"created"`
	Updated int `json:"updated"`
	Failed  int `json:"failed"`
}

// NewSyncService 创建新的同步服务
func NewSyncService(db *gorm.DB, cfg *config.Config, logger *slog.Logger) (*SyncService, error) {
	// 创建DeBank抓取器
	scraperConfig := &scraper.DeBankConfig{
		BaseURL:     cfg.External.Debank.BaseURL,
		Timeout:     cfg.External.Debank.Timeout,
		UserAgent:   cfg.External.Debank.UserAgent,
		MaxRetries:  cfg.External.Debank.MaxRetries,
		RetryDelay:  cfg.External.Debank.RetryDelay,
		RateLimit:   cfg.External.Debank.RateLimit,
	}

	debankScraper, err := scraper.NewDeBankScraper(scraperConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("创建DeBank抓取器失败: %w", err)
	}

	// 创建协议解析器
	protocolParser := parser.NewProtocolParser(logger)

	return &SyncService{
		db:      db,
		config:  cfg,
		logger:  logger,
		scraper: debankScraper,
		parser:  protocolParser,
		stats:   &SyncStats{},
	}, nil
}

// SyncProtocolMetadata 同步协议元数据
func (s *SyncService) SyncProtocolMetadata(ctx context.Context, fullSync bool) (*SyncStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("开始同步协议元数据", "full_sync", fullSync)
	startTime := time.Now()

	// 重置统计
	s.stats = &SyncStats{}

	// 抓取协议列表
	scrapedProtocols, err := s.scraper.FetchProtocolList(ctx)
	if err != nil {
		return nil, fmt.Errorf("抓取协议列表失败: %w", err)
	}

	s.logger.Info("抓取到协议列表", "count", len(scrapedProtocols))
	s.stats.Total = len(scrapedProtocols)

	// 批量解析协议信息
	scrapedInfos := make([]*scraper.ProtocolInfo, len(scrapedProtocols))
	for i := range scrapedProtocols {
		scrapedInfos[i] = &scrapedProtocols[i]
	}

	parsedProtocols, parseErrors := s.parser.ParseBatchProtocols(ctx, scrapedInfos)
	s.stats.Failed += len(parseErrors)

	// 记录解析错误
	for _, err := range parseErrors {
		s.logger.Warn("协议解析错误", "error", err)
	}

	// 并发处理协议同步
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, s.config.Sync.ProtocolMetadata.Concurrency)
	results := make(chan *syncResult, len(parsedProtocols))

	for _, protocol := range parsedProtocols {
		wg.Add(1)
		go func(p *models.Protocol) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := s.syncSingleProtocol(ctx, p, fullSync)
			results <- result
		}(protocol)
	}

	// 等待所有goroutine完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	for result := range results {
		if result.err != nil {
			s.logger.Warn("协议同步失败", 
				"protocol_id", result.protocolID, 
				"error", result.err)
			s.stats.Failed++
		} else {
			if result.created {
				s.stats.Created++
			} else {
				s.stats.Updated++
			}
		}
	}

	// 清理过时的协议（如果是全量同步）
	if fullSync {
		if err := s.cleanupStaleProtocols(ctx, parsedProtocols); err != nil {
			s.logger.Warn("清理过时协议失败", "error", err)
		}
	}

	duration := time.Since(startTime)
	s.logger.Info("协议元数据同步完成",
		"duration", duration,
		"total", s.stats.Total,
		"created", s.stats.Created,
		"updated", s.stats.Updated,
		"failed", s.stats.Failed)

	return s.stats, nil
}

// SyncProtocolTokens 同步协议代币
func (s *SyncService) SyncProtocolTokens(ctx context.Context) (*SyncStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("开始同步协议代币")
	startTime := time.Now()

	// 重置统计
	s.stats = &SyncStats{}

	// 获取所有活跃协议
	var protocols []models.Protocol
	if err := s.db.Where("is_active = ?", true).Find(&protocols).Error; err != nil {
		return nil, fmt.Errorf("获取活跃协议失败: %w", err)
	}

	s.stats.Total = len(protocols)

	// 并发处理协议代币同步
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, s.config.Sync.ProtocolTokens.Concurrency)
	results := make(chan *syncResult, len(protocols))

	for _, protocol := range protocols {
		wg.Add(1)
		go func(p models.Protocol) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := s.syncProtocolTokens(ctx, p.ProtocolID)
			results <- result
		}(protocol)
	}

	// 等待所有goroutine完成
	go func() {
		wg.Wait()
		close(results)
	}()

	// 收集结果
	for result := range results {
		if result.err != nil {
			s.logger.Warn("协议代币同步失败", 
				"protocol_id", result.protocolID, 
				"error", result.err)
			s.stats.Failed++
		} else {
			s.stats.Updated++ // 代币同步总是更新操作
		}
	}

	duration := time.Since(startTime)
	s.logger.Info("协议代币同步完成",
		"duration", duration,
		"total", s.stats.Total,
		"updated", s.stats.Updated,
		"failed", s.stats.Failed)

	return s.stats, nil
}

// IncrementalSyncProtocol 增量同步单个协议
func (s *SyncService) IncrementalSyncProtocol(ctx context.Context, protocolID string) (*SyncStats, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("开始增量同步协议", "protocol_id", protocolID)
	startTime := time.Now()

	// 重置统计
	s.stats = &SyncStats{Total: 1}

	// 抓取协议详情
	scrapedInfo, err := s.scraper.FetchProtocolDetail(ctx, protocolID)
	if err != nil {
		s.stats.Failed = 1
		return s.stats, fmt.Errorf("抓取协议详情失败: %w", err)
	}

	// 解析协议信息
	parsedProtocol, err := s.parser.ParseProtocolInfo(ctx, scrapedInfo)
	if err != nil {
		s.stats.Failed = 1
		return s.stats, fmt.Errorf("解析协议信息失败: %w", err)
	}

	// 同步单个协议
	result := s.syncSingleProtocol(ctx, parsedProtocol, false)
	if result.err != nil {
		s.stats.Failed = 1
		return s.stats, result.err
	}

	if result.created {
		s.stats.Created = 1
	} else {
		s.stats.Updated = 1
	}

	duration := time.Since(startTime)
	s.logger.Info("协议增量同步完成",
		"protocol_id", protocolID,
		"duration", duration,
		"created", result.created,
		"updated", !result.created)

	return s.stats, nil
}

// syncSingleProtocol 同步单个协议
func (s *SyncService) syncSingleProtocol(ctx context.Context, protocol *models.Protocol, fullSync bool) *syncResult {
	result := &syncResult{
		protocolID: protocol.ProtocolID,
	}

	// 检查协议是否已存在
	var existingProtocol models.Protocol
	err := s.db.Where("protocol_id = ?", protocol.ProtocolID).First(&existingProtocol).Error

	if err == gorm.ErrRecordNotFound {
		// 创建新协议
		if err := s.db.Create(protocol).Error; err != nil {
			result.err = fmt.Errorf("创建协议失败: %w", err)
			return result
		}
		result.created = true
		s.logger.Info("创建新协议", "protocol_id", protocol.ProtocolID, "name", protocol.Name)
	} else if err != nil {
		result.err = fmt.Errorf("查询协议失败: %w", err)
		return result
	} else {
		// 更新现有协议
		// 提取变更
		changes := s.parser.ExtractProtocolChanges(&existingProtocol, protocol)
		
		// 如果有变更，则更新
		if len(changes) > 0 || fullSync {
			// 更新协议
			protocol.ID = existingProtocol.ID
			protocol.CreatedAt = existingProtocol.CreatedAt
			
			if err := s.db.Save(protocol).Error; err != nil {
				result.err = fmt.Errorf("更新协议失败: %w", err)
				return result
			}
			
			s.logger.Info("更新协议", 
				"protocol_id", protocol.ProtocolID, 
				"changes", len(changes))
		} else {
			// 没有变更，只更新最后同步时间
			if err := s.db.Model(&existingProtocol).
				Update("last_synced_at", time.Now()).Error; err != nil {
				result.err = fmt.Errorf("更新最后同步时间失败: %w", err)
				return result
			}
			s.logger.Debug("协议无变更，跳过更新", "protocol_id", protocol.ProtocolID)
		}
		result.created = false
	}

	return result
}

// syncProtocolTokens 同步协议代币
func (s *SyncService) syncProtocolTokens(ctx context.Context, protocolID string) *syncResult {
	result := &syncResult{
		protocolID: protocolID,
	}

	// 抓取代币列表
	scrapedTokens, err := s.scraper.FetchProtocolTokens(ctx, protocolID)
	if err != nil {
		result.err = fmt.Errorf("抓取代币列表失败: %w", err)
		return result
	}

	// 批量解析代币信息
	scrapedInfos := make([]*scraper.TokenInfo, len(scrapedTokens))
	for i := range scrapedTokens {
		scrapedInfos[i] = &scrapedTokens[i]
	}

	parsedTokens, parseErrors := s.parser.ParseBatchTokens(ctx, protocolID, scrapedInfos)
	
	// 记录解析错误
	for _, err := range parseErrors {
		s.logger.Warn("代币解析错误", "protocol_id", protocolID, "error", err)
	}

	// 批量同步代币
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			result.err = fmt.Errorf("代币同步发生panic: %v", r)
		}
	}()

	for _, token := range parsedTokens {
		// 检查代币是否已存在
		var existingToken models.ProtocolToken
		err := tx.Where("protocol_id = ? AND token_address = ?", 
			protocolID, token.TokenAddress).First(&existingToken).Error

		if err == gorm.ErrRecordNotFound {
			// 创建新代币
			if err := tx.Create(token).Error; err != nil {
				tx.Rollback()
				result.err = fmt.Errorf("创建代币失败: %w", err)
				return result
			}
		} else if err != nil {
			tx.Rollback()
			result.err = fmt.Errorf("查询代币失败: %w", err)
			return result
		} else {
			// 更新现有代币
			token.ID = existingToken.ID
			token.CreatedAt = existingToken.CreatedAt
			
			if err := tx.Save(token).Error; err != nil {
				tx.Rollback()
				result.err = fmt.Errorf("更新代币失败: %w", err)
				return result
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		result.err = fmt.Errorf("提交代币事务失败: %w", err)
		return result
	}

	s.logger.Info("协议代币同步完成", 
		"protocol_id", protocolID, 
		"token_count", len(parsedTokens))

	return result
}

// cleanupStaleProtocols 清理过时的协议
func (s *SyncService) cleanupStaleProtocols(ctx context.Context, currentProtocols []*models.Protocol) error {
	// 获取当前协议ID列表
	currentIDs := make(map[string]bool)
	for _, p := range currentProtocols {
		currentIDs[p.ProtocolID] = true
	}

	// 查找数据库中不在当前列表中的协议
	var staleProtocols []models.Protocol
	if err := s.db.Where("is_active = ?", true).Find(&staleProtocols).Error; err != nil {
		return fmt.Errorf("查找协议失败: %w", err)
	}

	// 标记过时协议为非活跃
	for _, protocol := range staleProtocols {
		if !currentIDs[protocol.ProtocolID] {
			if err := s.db.Model(&protocol).
				Updates(map[string]interface{}{
					"is_active": false,
					"updated_at": time.Now(),
				}).Error; err != nil {
				s.logger.Warn("标记过时协议失败", 
					"protocol_id", protocol.ProtocolID, 
					"error", err)
			} else {
				s.logger.Info("标记过时协议为非活跃", 
					"protocol_id", protocol.ProtocolID, 
					"name", protocol.Name)
			}
		}
	}

	return nil
}

// GetSyncStats 获取同步统计
func (s *SyncService) GetSyncStats() *SyncStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	return &SyncStats{
		Total:   s.stats.Total,
		Created: s.stats.Created,
		Updated: s.stats.Updated,
		Failed:  s.stats.Failed,
	}
}

// Close 关闭服务
func (s *SyncService) Close() error {
	if s.scraper != nil {
		return s.scraper.Close()
	}
	return nil
}

// syncResult 同步结果
type syncResult struct {
	protocolID string
	created    bool
	err        error
}