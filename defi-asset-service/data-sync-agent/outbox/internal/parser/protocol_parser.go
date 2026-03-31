package parser

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/openclaw/defi-asset-service/data-sync-agent/internal/models"
	"github.com/openclaw/defi-asset-service/data-sync-agent/internal/scraper"
)

// ProtocolParser 协议元数据解析器
type ProtocolParser struct {
	logger *slog.Logger
}

// NewProtocolParser 创建新的协议解析器
func NewProtocolParser(logger *slog.Logger) *ProtocolParser {
	return &ProtocolParser{
		logger: logger,
	}
}

// ParseProtocolInfo 解析协议信息
func (p *ProtocolParser) ParseProtocolInfo(ctx context.Context, scrapedInfo *scraper.ProtocolInfo) (*models.Protocol, error) {
	if scrapedInfo == nil {
		return nil, fmt.Errorf("抓取的协议信息为空")
	}

	protocol := &models.Protocol{
		ProtocolID:      scrapedInfo.ProtocolID,
		Name:            scrapedInfo.Name,
		Description:     scrapedInfo.Description,
		Category:        p.normalizeCategory(scrapedInfo.Category),
		LogoURL:         scrapedInfo.LogoURL,
		WebsiteURL:      scrapedInfo.WebsiteURL,
		TwitterURL:      scrapedInfo.TwitterURL,
		GitHubURL:       scrapedInfo.GitHubURL,
		TvlUSD:          scrapedInfo.TvlUSD,
		RiskLevel:       scrapedInfo.RiskLevel,
		IsActive:        scrapedInfo.IsActive,
		SyncSource:      "debank",
		SyncVersion:     1,
		LastSyncedAt:    time.Now(),
	}

	// 处理支持的链
	if len(scrapedInfo.SupportedChains) > 0 {
		chainsJSON, err := json.Marshal(scrapedInfo.SupportedChains)
		if err != nil {
			p.logger.Warn("序列化支持的链失败", "error", err)
		} else {
			protocol.SupportedChains = chainsJSON
		}
	}

	// 处理扩展元数据
	metadata := make(map[string]interface{})
	if scrapedInfo.Metadata != nil {
		metadata = scrapedInfo.Metadata
	}

	// 添加解析时间戳
	metadata["parsed_at"] = time.Now().Format(time.RFC3339)
	metadata["source"] = "debank"

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		p.logger.Warn("序列化元数据失败", "error", err)
	} else {
		protocol.Metadata = metadataJSON
	}

	// 验证协议数据
	if err := p.validateProtocol(protocol); err != nil {
		return nil, fmt.Errorf("协议验证失败: %w", err)
	}

	p.logger.Debug("协议解析完成", "protocol_id", protocol.ProtocolID, "name", protocol.Name)
	return protocol, nil
}

// ParseTokenInfo 解析代币信息
func (p *ProtocolParser) ParseTokenInfo(ctx context.Context, protocolID string, scrapedInfo *scraper.TokenInfo) (*models.ProtocolToken, error) {
	if scrapedInfo == nil {
		return nil, fmt.Errorf("抓取的代币信息为空")
	}

	token := &models.ProtocolToken{
		ProtocolID:     protocolID,
		ChainID:        1, // 默认以太坊主网，实际应从上下文获取
		TokenAddress:   scrapedInfo.TokenAddress,
		TokenSymbol:    scrapedInfo.TokenSymbol,
		TokenName:      scrapedInfo.TokenName,
		TokenDecimals:  scrapedInfo.TokenDecimals,
		IsCollateral:   scrapedInfo.IsCollateral,
		IsBorrowable:   scrapedInfo.IsBorrowable,
		IsSupply:       scrapedInfo.IsSupply,
		SupplyAPY:      scrapedInfo.SupplyAPY,
		BorrowAPY:      scrapedInfo.BorrowAPY,
		PriceUSD:       scrapedInfo.PriceUSD,
		TvlUSD:         scrapedInfo.TvlUSD,
		LastUpdatedAt:  time.Now(),
	}

	// 处理扩展元数据
	metadata := make(map[string]interface{})
	metadata["parsed_at"] = time.Now().Format(time.RFC3339)
	metadata["source"] = "debank"

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		p.logger.Warn("序列化代币元数据失败", "error", err)
	} else {
		token.Metadata = metadataJSON
	}

	// 验证代币数据
	if err := p.validateToken(token); err != nil {
		return nil, fmt.Errorf("代币验证失败: %w", err)
	}

	p.logger.Debug("代币解析完成", 
		"protocol_id", protocolID, 
		"token_symbol", token.TokenSymbol,
		"token_address", token.TokenAddress)
	return token, nil
}

// ParseBatchProtocols 批量解析协议信息
func (p *ProtocolParser) ParseBatchProtocols(ctx context.Context, scrapedInfos []*scraper.ProtocolInfo) ([]*models.Protocol, []error) {
	protocols := make([]*models.Protocol, 0, len(scrapedInfos))
	errors := make([]error, 0)

	for _, scrapedInfo := range scrapedInfos {
		protocol, err := p.ParseProtocolInfo(ctx, scrapedInfo)
		if err != nil {
			errors = append(errors, fmt.Errorf("解析协议失败 %s: %w", scrapedInfo.ProtocolID, err))
			continue
		}
		protocols = append(protocols, protocol)
	}

	p.logger.Info("批量协议解析完成", 
		"total", len(scrapedInfos), 
		"success", len(protocols), 
		"failed", len(errors))
	return protocols, errors
}

// ParseBatchTokens 批量解析代币信息
func (p *ProtocolParser) ParseBatchTokens(ctx context.Context, protocolID string, scrapedInfos []*scraper.TokenInfo) ([]*models.ProtocolToken, []error) {
	tokens := make([]*models.ProtocolToken, 0, len(scrapedInfos))
	errors := make([]error, 0)

	for _, scrapedInfo := range scrapedInfos {
		token, err := p.ParseTokenInfo(ctx, protocolID, scrapedInfo)
		if err != nil {
			errors = append(errors, fmt.Errorf("解析代币失败 %s: %w", scrapedInfo.TokenSymbol, err))
			continue
		}
		tokens = append(tokens, token)
	}

	p.logger.Info("批量代币解析完成", 
		"protocol_id", protocolID,
		"total", len(scrapedInfos), 
		"success", len(tokens), 
		"failed", len(errors))
	return tokens, errors
}

// normalizeCategory 规范化协议类别
func (p *ProtocolParser) normalizeCategory(category string) string {
	category = strings.ToLower(strings.TrimSpace(category))
	
	// 类别映射
	categoryMap := map[string]string{
		// 借贷协议
		"lending": "lending",
		"borrow": "lending",
		"loan": "lending",
		"credit": "lending",
		"mortgage": "lending",
		
		// DEX
		"dex": "dex",
		"exchange": "dex",
		"swap": "dex",
		"amm": "dex",
		
		// 收益聚合
		"yield": "yield",
		"farm": "yield",
		"vault": "yield",
		"aggregator": "yield",
		
		// 衍生品
		"derivative": "derivative",
		"futures": "derivative",
		"options": "derivative",
		"perpetual": "derivative",
		
		// 保险
		"insurance": "insurance",
		"cover": "insurance",
		
		// 预测市场
		"prediction": "prediction",
		"betting": "prediction",
		
		// NFT
		"nft": "nft",
		"marketplace": "nft",
		
		// 跨链
		"bridge": "bridge",
		"cross-chain": "bridge",
		
		// 其他
		"staking": "staking",
		"wallet": "wallet",
		"index": "index",
		"options": "options",
	}

	// 精确匹配
	if normalized, exists := categoryMap[category]; exists {
		return normalized
	}

	// 部分匹配
	for key, value := range categoryMap {
		if strings.Contains(category, key) {
			return value
		}
	}

	// 默认类别
	return "other"
}

// validateProtocol 验证协议数据
func (p *ProtocolParser) validateProtocol(protocol *models.Protocol) error {
	// 检查必需字段
	if protocol.ProtocolID == "" {
		return fmt.Errorf("协议ID不能为空")
	}
	if protocol.Name == "" {
		return fmt.Errorf("协议名称不能为空")
	}
	if protocol.Category == "" {
		return fmt.Errorf("协议类别不能为空")
	}

	// 验证协议ID格式
	if !p.isValidProtocolID(protocol.ProtocolID) {
		return fmt.Errorf("协议ID格式无效: %s", protocol.ProtocolID)
	}

	// 验证URL格式
	if protocol.WebsiteURL != "" && !p.isValidURL(protocol.WebsiteURL) {
		p.logger.Warn("网站URL格式无效", "url", protocol.WebsiteURL)
		protocol.WebsiteURL = ""
	}
	if protocol.TwitterURL != "" && !p.isValidURL(protocol.TwitterURL) {
		p.logger.Warn("Twitter URL格式无效", "url", protocol.TwitterURL)
		protocol.TwitterURL = ""
	}
	if protocol.GitHubURL != "" && !p.isValidURL(protocol.GitHubURL) {
		p.logger.Warn("GitHub URL格式无效", "url", protocol.GitHubURL)
		protocol.GitHubURL = ""
	}

	// 验证风险等级
	if protocol.RiskLevel < 1 || protocol.RiskLevel > 5 {
		p.logger.Warn("风险等级超出范围，重置为默认值", "risk_level", protocol.RiskLevel)
		protocol.RiskLevel = 3
	}

	// 验证TVL
	if protocol.TvlUSD < 0 {
		p.logger.Warn("TVL不能为负数，重置为0", "tvl_usd", protocol.TvlUSD)
		protocol.TvlUSD = 0
	}

	return nil
}

// validateToken 验证代币数据
func (p *ProtocolParser) validateToken(token *models.ProtocolToken) error {
	// 检查必需字段
	if token.ProtocolID == "" {
		return fmt.Errorf("协议ID不能为空")
	}
	if token.TokenAddress == "" {
		return fmt.Errorf("代币地址不能为空")
	}
	if token.TokenSymbol == "" {
		return fmt.Errorf("代币符号不能为空")
	}
	if token.TokenName == "" {
		return fmt.Errorf("代币名称不能为空")
	}

	// 验证代币地址格式
	if !p.isValidTokenAddress(token.TokenAddress) {
		return fmt.Errorf("代币地址格式无效: %s", token.TokenAddress)
	}

	// 验证代币精度
	if token.TokenDecimals < 0 || token.TokenDecimals > 36 {
		p.logger.Warn("代币精度超出范围，重置为18", 
			"token_symbol", token.TokenSymbol, 
			"decimals", token.TokenDecimals)
		token.TokenDecimals = 18
	}

	// 验证价格
	if token.PriceUSD < 0 {
		p.logger.Warn("代币价格不能为负数，重置为0", 
			"token_symbol", token.TokenSymbol, 
			"price_usd", token.PriceUSD)
		token.PriceUSD = 0
	}

	// 验证TVL
	if token.TvlUSD < 0 {
		p.logger.Warn("代币TVL不能为负数，重置为0", 
			"token_symbol", token.TokenSymbol, 
			"tvl_usd", token.TvlUSD)
		token.TvlUSD = 0
	}

	// 验证APY
	if token.SupplyAPY < 0 || token.SupplyAPY > 100 {
		p.logger.Warn("供应APY超出合理范围，重置为0", 
			"token_symbol", token.TokenSymbol, 
			"supply_apy", token.SupplyAPY)
		token.SupplyAPY = 0
	}
	if token.BorrowAPY < 0 || token.BorrowAPY > 100 {
		p.logger.Warn("借款APY超出合理范围，重置为0", 
			"token_symbol", token.TokenSymbol, 
			"borrow_apy", token.BorrowAPY)
		token.BorrowAPY = 0
	}

	return nil
}

// isValidProtocolID 验证协议ID格式
func (p *ProtocolParser) isValidProtocolID(id string) bool {
	// 协议ID应该只包含小写字母、数字、下划线和连字符
	re := regexp.MustCompile(`^[a-z0-9_-]+$`)
	return re.MatchString(id) && len(id) <= 100
}

// isValidTokenAddress 验证代币地址格式
func (p *ProtocolParser) isValidTokenAddress(address string) bool {
	// Ethereum地址格式
	if strings.HasPrefix(address, "0x") && len(address) == 42 {
		re := regexp.MustCompile(`^0x[a-fA-F0-9]{40}$`)
		return re.MatchString(address)
	}
	
	// 其他链的地址格式（这里可以根据需要扩展）
	return true
}

// isValidURL 验证URL格式
func (p *ProtocolParser) isValidURL(url string) bool {
	re := regexp.MustCompile(`^(https?|ftp)://[^\s/$.?#].[^\s]*$`)
	return re.MatchString(url)
}

// ExtractProtocolChanges 提取协议变更
func (p *ProtocolParser) ExtractProtocolChanges(oldProtocol, newProtocol *models.Protocol) map[string]interface{} {
	changes := make(map[string]interface{})

	// 比较基本字段
	if oldProtocol.Name != newProtocol.Name {
		changes["name"] = map[string]interface{}{
			"old": oldProtocol.Name,
			"new": newProtocol.Name,
		}
	}

	if oldProtocol.Description != newProtocol.Description {
		changes["description"] = map[string]interface{}{
			"old": oldProtocol.Description,
			"new": newProtocol.Description,
		}
	}

	if oldProtocol.Category != newProtocol.Category {
		changes["category"] = map[string]interface{}{
			"old": oldProtocol.Category,
			"new": newProtocol.Category,
		}
	}

	// 比较TVL（变化超过10%才记录）
	oldTvl := oldProtocol.TvlUSD
	newTvl := newProtocol.TvlUSD
	if oldTvl > 0 && newTvl > 0 {
		changePercent := (newTvl - oldTvl) / oldTvl * 100
		if abs(changePercent) > 10 {
			changes["tvl_usd"] = map[string]interface{}{
				"old": oldTvl,
				"new": newTvl,
				"change_percent": changePercent,
			}
		}
	}

	// 比较风险等级
	if oldProtocol.RiskLevel != newProtocol.RiskLevel {
		changes["risk_level"] = map[string]interface{}{
			"old": oldProtocol.RiskLevel,
			"new": newProtocol.RiskLevel,
		}
	}

	// 比较活跃状态
	if oldProtocol.IsActive != newProtocol.IsActive {
		changes["is_active"] = map[string]interface{}{
			"old": oldProtocol.IsActive,
			"new": newProtocol.IsActive,
		}
	}

	return changes
}

// abs 绝对值函数
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}