package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"defi-asset-service/internal/model"
)

// executeProtocolSync 执行协议同步（异步）（续）
func (s *protocolService) executeProtocolSync(ctx context.Context, syncRecordID uint64, forceFullSync bool, protocolIDs []string) {
	// 更新同步记录状态
	syncRecord := &model.SyncRecord{
		ID:     syncRecordID,
		Status: "processing",
	}
	
	if err := s.protocolRepo.UpdateSyncRecord(ctx, syncRecord); err != nil {
		s.logger.WithError(err).Error("Failed to update sync record status")
		return
	}
	
	var protocolsToSync []string
	var totalCount int
	var successCount int
	var failedCount int
	
	// 确定需要同步的协议
	if len(protocolIDs) > 0 {
		// 同步指定协议
		protocolsToSync = protocolIDs
	} else if forceFullSync {
		// 同步所有协议
		// 这里需要实现获取所有协议ID的逻辑
		// 暂时使用空列表，实际应从数据库或DeBank获取
		protocolsToSync = []string{}
	} else {
		// 增量同步：只同步最近更新的协议
		// 这里需要实现获取需要更新的协议ID的逻辑
		protocolsToSync = []string{}
	}
	
	totalCount = len(protocolsToSync)
	
	// 更新总记录数
	syncRecord.TotalCount = totalCount
	if err := s.protocolRepo.UpdateSyncRecord(ctx, syncRecord); err != nil {
		s.logger.WithError(err).Error("Failed to update sync record total count")
	}
	
	// 同步每个协议
	for _, protocolID := range protocolsToSync {
		if err := s.syncSingleProtocol(ctx, protocolID); err != nil {
			s.logger.WithError(err).Errorf("Failed to sync protocol %s", protocolID)
			failedCount++
			
			// 记录错误信息
			syncRecord.ErrorMessage = err.Error()
		} else {
			successCount++
		}
		
		// 更新进度
		syncRecord.SuccessCount = successCount
		syncRecord.FailedCount = failedCount
		if err := s.protocolRepo.UpdateSyncRecord(ctx, syncRecord); err != nil {
			s.logger.WithError(err).Error("Failed to update sync record progress")
		}
		
		// 添加延迟避免速率限制
		time.Sleep(1 * time.Second)
	}
	
	// 更新同步记录状态
	syncRecord.Status = "success"
	if failedCount > 0 {
		syncRecord.Status = "partial_success"
	}
	if successCount == 0 && failedCount > 0 {
		syncRecord.Status = "failed"
	}
	
	syncRecord.FinishedAt = time.Now()
	syncRecord.DurationMs = int(time.Since(syncRecord.StartedAt).Milliseconds())
	
	if err := s.protocolRepo.UpdateSyncRecord(ctx, syncRecord); err != nil {
		s.logger.WithError(err).Error("Failed to update sync record final status")
	}
	
	// 清理相关缓存
	s.cleanupProtocolCacheAfterSync(ctx, protocolsToSync)
}

// syncSingleProtocol 同步单个协议
func (s *protocolService) syncSingleProtocol(ctx context.Context, protocolID string) error {
	// 1. 从DeBank获取协议数据
	protocol, err := s.client.FetchProtocolDetails(ctx, protocolID)
	if err != nil {
		return fmt.Errorf("failed to fetch protocol details: %w", err)
	}
	
	// 2. 存储到数据库
	if err := s.protocolRepo.CreateProtocol(ctx, protocol); err != nil {
		// 如果协议已存在，更新它
		existing, err := s.protocolRepo.GetProtocolByID(ctx, protocolID)
		if err != nil {
			return fmt.Errorf("failed to check existing protocol: %w", err)
		}
		
		if existing != nil {
			protocol.ID = existing.ID
			protocol.CreatedAt = existing.CreatedAt
			protocol.SyncVersion = existing.SyncVersion + 1
			if err := s.protocolRepo.UpdateProtocol(ctx, protocol); err != nil {
				return fmt.Errorf("failed to update protocol: %w", err)
			}
		} else {
			return fmt.Errorf("failed to create protocol: %w", err)
		}
	}
	
	// 3. 获取协议代币数据
	tokens, err := s.client.FetchProtocolTokens(ctx, protocolID, 1) // 默认以太坊主网
	if err != nil {
		s.logger.WithError(err).Warnf("Failed to fetch tokens for protocol %s", protocolID)
	} else if len(tokens) > 0 {
		// 存储代币数据
		if err := s.protocolRepo.BatchCreateOrUpdateProtocolTokens(ctx, tokens); err != nil {
			s.logger.WithError(err).Warnf("Failed to store tokens for protocol %s", protocolID)
		}
	}
	
	// 4. 更新协议最后同步时间
	protocol.LastSyncedAt = time.Now()
	if err := s.protocolRepo.UpdateProtocol(ctx, protocol); err != nil {
		s.logger.WithError(err).Warnf("Failed to update protocol sync time: %s", protocolID)
	}
	
	// 5. 更新缓存
	s.UpdateProtocolCache(ctx, protocolID)
	
	return nil
}

// cleanupProtocolCacheAfterSync 同步后清理缓存
func (s *protocolService) cleanupProtocolCacheAfterSync(ctx context.Context, protocolIDs []string) {
	// 清理协议列表缓存
	categories := []string{"", "lending", "dex", "yield", "derivatives", "insurance"}
	for _, category := range categories {
		for page := 1; page <= 10; page++ { // 清理前10页
			s.redisRepo.Delete(ctx, fmt.Sprintf("%s:protocols:list:%s:%d", 
				s.redisRepo.(*redisRepository).prefix, category, page))
		}
	}
	
	// 清理单个协议缓存
	for _, protocolID := range protocolIDs {
		s.redisRepo.DeleteProtocolCache(ctx, protocolID)
	}
}

// convertDebankProtocol 转换DeBank协议为内部协议模型
func (c *protocolClient) convertDebankProtocol(debankProtocol *DebankProtocol) (*model.Protocol, error) {
	if debankProtocol == nil {
		return nil, fmt.Errorf("debank protocol is nil")
	}
	
	// 解析支持的链
	var supportedChains []int
	if debankProtocol.ChainList != nil {
		for _, chain := range debankProtocol.ChainList {
			if chain.ChainID > 0 {
				supportedChains = append(supportedChains, chain.ChainID)
			}
		}
	}
	
	// 构建元数据
	metadata := map[string]interface{}{
		"debank_id": debankProtocol.ID,
		"tags":      debankProtocol.TagList,
	}
	
	if debankProtocol.PortfolioItemList != nil {
		metadata["portfolio_items"] = debankProtocol.PortfolioItemList
	}
	
	supportedChainsJSON, err := json.Marshal(supportedChains)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal supported chains: %w", err)
	}
	
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	return &model.Protocol{
		ProtocolID:      strings.ToLower(debankProtocol.ID),
		Name:            debankProtocol.Name,
		Description:     debankProtocol.Description,
		Category:        getProtocolCategory(debankProtocol),
		LogoURL:         debankProtocol.LogoURL,
		WebsiteURL:      debankProtocol.SiteURL,
		TwitterURL:      debankProtocol.TwitterURL,
		GithubURL:       debankProtocol.GithubURL,
		TvlUSD:          debankProtocol.TVL,
		RiskLevel:       calculateRiskLevel(debankProtocol),
		IsActive:        true,
		SupportedChains: model.JSON{"chains": supportedChains},
		Metadata:        model.JSON(metadata),
		SyncSource:      "debank",
		SyncVersion:     1,
		LastSyncedAt:    time.Now(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}, nil
}

// convertToProtocolResponse 转换数据库协议为响应格式
func (s *protocolService) convertToProtocolResponse(protocol *model.Protocol) model.ProtocolResponse {
	// 解析支持的链
	var supportedChains []int
	if chains, ok := protocol.SupportedChains["chains"].([]interface{}); ok {
		for _, chain := range chains {
			if chainID, ok := chain.(float64); ok {
				supportedChains = append(supportedChains, int(chainID))
			}
		}
	}
	
	return model.ProtocolResponse{
		ProtocolID:      protocol.ProtocolID,
		Name:            protocol.Name,
		Description:     protocol.Description,
		Category:        protocol.Category,
		LogoURL:         protocol.LogoURL,
		WebsiteURL:      protocol.WebsiteURL,
		TwitterURL:      protocol.TwitterURL,
		GithubURL:       protocol.GithubURL,
		TvlUSD:          protocol.TvlUSD,
		RiskLevel:       int(protocol.RiskLevel),
		IsActive:        protocol.IsActive,
		SupportedChains: supportedChains,
		Metadata:        protocol.Metadata,
		LastSyncedAt:    protocol.LastSyncedAt.Format(time.RFC3339),
		CreatedAt:       protocol.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       protocol.UpdatedAt.Format(time.RFC3339),
	}
}

// convertToTokenResponses 转换数据库代币为响应格式
func (s *protocolService) convertToTokenResponses(tokens []model.ProtocolToken) []model.TokenResponse {
	var responses []model.TokenResponse
	for _, token := range tokens {
		responses = append(responses, model.TokenResponse{
			TokenAddress:        token.TokenAddress,
			TokenSymbol:         token.TokenSymbol,
			TokenName:           token.TokenName,
			TokenDecimals:       token.TokenDecimals,
			IsCollateral:        token.IsCollateral,
			IsBorrowable:        token.IsBorrowable,
			IsSupply:            token.IsSupply,
			SupplyApy:           token.SupplyApy,
			BorrowApy:           token.BorrowApy,
			LiquidationThreshold: token.LiquidationThreshold,
			CollateralFactor:    token.CollateralFactor,
			PriceUSD:            token.PriceUSD,
			TvlUSD:              token.TvlUSD,
			LastUpdatedAt:       token.LastUpdatedAt.Format(time.RFC3339),
		})
	}
	return responses
}

// getProtocolCategory 获取协议类别
func getProtocolCategory(debankProtocol *DebankProtocol) string {
	// 根据DeBank的标签确定类别
	if debankProtocol.TagList != nil {
		for _, tag := range debankProtocol.TagList {
			switch tag {
			case "lending":
				return "lending"
			case "dex":
				return "dex"
			case "yield":
				return "yield"
			case "derivatives":
				return "derivatives"
			case "insurance":
				return "insurance"
			case "staking":
				return "staking"
			}
		}
	}
	
	// 默认类别
	return "other"
}

// calculateRiskLevel 计算风险等级
func calculateRiskLevel(debankProtocol *DebankProtocol) int8 {
	// 简单的风险等级计算
	// 实际项目中应该根据更多因素计算
	riskLevel := int8(3) // 中等风险
	
	if debankProtocol.TVL > 1000000000 { // TVL > 10亿美元
		riskLevel = 2 // 低风险
	} else if debankProtocol.TVL < 10000000 { // TVL < 1000万美元
		riskLevel = 4 // 高风险
	}
	
	// 根据审计状态调整
	if debankProtocol.AuditList != nil && len(debankProtocol.AuditList) > 0 {
		riskLevel--
	}
	
	// 确保在1-5范围内
	if riskLevel < 1 {
		riskLevel = 1
	}
	if riskLevel > 5 {
		riskLevel = 5
	}
	
	return riskLevel
}

// DebankProtocol DeBank协议数据结构
type DebankProtocol struct {
	ID                 string           `json:"id"`
	Name               string           `json:"name"`
	Description        string           `json:"description"`
	LogoURL            string           `json:"logo_url"`
	SiteURL            string           `json:"site_url"`
	TwitterURL         string           `json:"twitter_url"`
	GithubURL          string           `json:"github_url"`
	TVL                float64          `json:"tvl"`
	TagList            []string         `json:"tag_list"`
	ChainList          []DebankChain    `json:"chain_list"`
	AuditList          []DebankAudit    `json:"audit_list"`
	PortfolioItemList  []DebankPortfolioItem `json:"portfolio_item_list"`
}

// DebankChain DeBank链数据结构
type DebankChain struct {
	ChainID   int    `json:"chain_id"`
	Name      string `json:"name"`
	LogoURL   string `json:"logo_url"`
}

// DebankAudit DeBank审计数据结构
type DebankAudit struct {
	AuditFirm string `json:"audit_firm"`
	ReportURL string `json:"report_url"`
	Date      string `json:"date"`
}

// DebankPortfolioItem DeBank投资组合项数据结构
type DebankPortfolioItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// 导入必要的包
import (
	"encoding/json"
)