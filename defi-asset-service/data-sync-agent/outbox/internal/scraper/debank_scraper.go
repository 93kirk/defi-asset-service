package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/time/rate"
)

// DeBankScraper DeBank网页抓取器
type DeBankScraper struct {
	collector   *colly.Collector
	rateLimiter *rate.Limiter
	config      *DeBankConfig
	logger      *slog.Logger
}

// DeBankConfig DeBank抓取配置
type DeBankConfig struct {
	BaseURL     string        `yaml:"base_url"`
	Timeout     time.Duration `yaml:"timeout"`
	UserAgent   string        `yaml:"user_agent"`
	MaxRetries  int           `yaml:"max_retries"`
	RetryDelay  time.Duration `yaml:"retry_delay"`
	RateLimit   int           `yaml:"rate_limit"` // 每秒请求数
}

// ProtocolInfo 协议信息
type ProtocolInfo struct {
	ProtocolID   string   `json:"protocol_id"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Category     string   `json:"category"`
	LogoURL      string   `json:"logo_url"`
	WebsiteURL   string   `json:"website_url"`
	TwitterURL   string   `json:"twitter_url"`
	GitHubURL    string   `json:"github_url"`
	TvlUSD       float64  `json:"tvl_usd"`
	RiskLevel    int      `json:"risk_level"`
	IsActive     bool     `json:"is_active"`
	SupportedChains []int  `json:"supported_chains"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// TokenInfo 代币信息
type TokenInfo struct {
	TokenAddress  string  `json:"token_address"`
	TokenSymbol   string  `json:"token_symbol"`
	TokenName     string  `json:"token_name"`
	TokenDecimals int     `json:"token_decimals"`
	IsCollateral  bool    `json:"is_collateral"`
	IsBorrowable  bool    `json:"is_borrowable"`
	IsSupply      bool    `json:"is_supply"`
	SupplyAPY     float64 `json:"supply_apy"`
	BorrowAPY     float64 `json:"borrow_apy"`
	PriceUSD      float64 `json:"price_usd"`
	TvlUSD        float64 `json:"tvl_usd"`
}

// NewDeBankScraper 创建新的DeBank抓取器
func NewDeBankScraper(config *DeBankConfig, logger *slog.Logger) (*DeBankScraper, error) {
	if config == nil {
		config = &DeBankConfig{
			BaseURL:    "https://debank.com",
			Timeout:    30 * time.Second,
			UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
			MaxRetries: 3,
			RetryDelay: 2 * time.Second,
			RateLimit:  10,
		}
	}

	// 创建Colly收集器
	collector := colly.NewCollector(
		colly.UserAgent(config.UserAgent),
		colly.AllowURLRevisit(),
		colly.Async(true),
	)

	// 设置超时
	collector.SetRequestTimeout(config.Timeout)

	// 设置重试
	collector.OnError(func(r *colly.Response, err error) {
		retries := r.Request.RetryTimes
		if retries < config.MaxRetries {
			slog.Warn("请求失败，准备重试",
				"url", r.Request.URL,
				"error", err,
				"retry", retries+1,
				"max_retries", config.MaxRetries)
			time.Sleep(config.RetryDelay)
			r.Request.Retry()
		} else {
			slog.Error("请求失败，达到最大重试次数",
				"url", r.Request.URL,
				"error", err,
				"retries", retries)
		}
	})

	// 创建速率限制器
	rateLimiter := rate.NewLimiter(rate.Limit(config.RateLimit), 1)

	return &DeBankScraper{
		collector:   collector,
		rateLimiter: rateLimiter,
		config:      config,
		logger:      logger,
	}, nil
}

// FetchProtocolList 获取协议列表
func (s *DeBankScraper) FetchProtocolList(ctx context.Context) ([]ProtocolInfo, error) {
	s.logger.Info("开始获取DeBank协议列表")

	protocols := make([]ProtocolInfo, 0)
	url := fmt.Sprintf("%s/protocols", s.config.BaseURL)

	// 等待速率限制
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("速率限制等待失败: %w", err)
	}

	s.collector.OnHTML("body", func(e *colly.HTMLElement) {
		// 查找协议列表容器
		e.DOM.Find(".ProtocolList__list").Each(func(i int, sel *goquery.Selection) {
			sel.Find(".ProtocolItem").Each(func(j int, item *goquery.Selection) {
				protocol, err := s.parseProtocolItem(item)
				if err != nil {
					s.logger.Warn("解析协议项失败", "error", err)
					return
				}
				protocols = append(protocols, protocol)
			})
		})

		// 如果没有找到传统列表，尝试解析JSON数据
		if len(protocols) == 0 {
			s.parseProtocolJSON(e)
		}
	})

	// 发送请求
	err := s.collector.Visit(url)
	if err != nil {
		return nil, fmt.Errorf("访问URL失败: %w", err)
	}

	s.collector.Wait()

	s.logger.Info("协议列表获取完成", "count", len(protocols))
	return protocols, nil
}

// FetchProtocolDetail 获取协议详情
func (s *DeBankScraper) FetchProtocolDetail(ctx context.Context, protocolID string) (*ProtocolInfo, error) {
	s.logger.Info("获取协议详情", "protocol_id", protocolID)

	url := fmt.Sprintf("%s/protocol/%s", s.config.BaseURL, protocolID)
	var protocol *ProtocolInfo

	// 等待速率限制
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("速率限制等待失败: %w", err)
	}

	s.collector.OnHTML("body", func(e *colly.HTMLElement) {
		protocol = s.parseProtocolDetail(e.DOM)
		if protocol != nil {
			protocol.ProtocolID = protocolID
		}
	})

	// 发送请求
	err := s.collector.Visit(url)
	if err != nil {
		return nil, fmt.Errorf("访问协议详情URL失败: %w", err)
	}

	s.collector.Wait()

	if protocol == nil {
		return nil, fmt.Errorf("未找到协议详情: %s", protocolID)
	}

	return protocol, nil
}

// FetchProtocolTokens 获取协议代币列表
func (s *DeBankScraper) FetchProtocolTokens(ctx context.Context, protocolID string) ([]TokenInfo, error) {
	s.logger.Info("获取协议代币列表", "protocol_id", protocolID)

	tokens := make([]TokenInfo, 0)
	url := fmt.Sprintf("%s/protocol/%s/tokens", s.config.BaseURL, protocolID)

	// 等待速率限制
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("速率限制等待失败: %w", err)
	}

	s.collector.OnHTML("body", func(e *colly.HTMLElement) {
		e.DOM.Find(".TokenList__list .TokenItem").Each(func(i int, sel *goquery.Selection) {
			token, err := s.parseTokenItem(sel)
			if err != nil {
				s.logger.Warn("解析代币项失败", "error", err)
				return
			}
			tokens = append(tokens, token)
		})

		// 尝试解析JSON数据
		if len(tokens) == 0 {
			s.parseTokenJSON(e)
		}
	})

	// 发送请求
	err := s.collector.Visit(url)
	if err != nil {
		return nil, fmt.Errorf("访问代币列表URL失败: %w", err)
	}

	s.collector.Wait()

	s.logger.Info("代币列表获取完成", "protocol_id", protocolID, "count", len(tokens))
	return tokens, nil
}

// parseProtocolItem 解析协议列表项
func (s *DeBankScraper) parseProtocolItem(item *goquery.Selection) (ProtocolInfo, error) {
	var protocol ProtocolInfo

	// 提取协议ID
	href, exists := item.Find("a").Attr("href")
	if exists {
		re := regexp.MustCompile(`/protocol/([^/]+)`)
		matches := re.FindStringSubmatch(href)
		if len(matches) > 1 {
			protocol.ProtocolID = matches[1]
		}
	}

	// 提取协议名称
	protocol.Name = strings.TrimSpace(item.Find(".ProtocolItem__name").Text())

	// 提取描述
	protocol.Description = strings.TrimSpace(item.Find(".ProtocolItem__description").Text())

	// 提取类别
	protocol.Category = strings.TrimSpace(item.Find(".ProtocolItem__category").Text())

	// 提取Logo URL
	logoURL, exists := item.Find(".ProtocolItem__logo img").Attr("src")
	if exists {
		protocol.LogoURL = s.normalizeURL(logoURL)
	}

	// 提取TVL
	tvlText := strings.TrimSpace(item.Find(".ProtocolItem__tvl").Text())
	protocol.TvlUSD = s.parseTVL(tvlText)

	// 设置默认值
	protocol.IsActive = true
	protocol.RiskLevel = 3 // 默认中等风险
	protocol.SupportedChains = []int{1} // 默认以太坊主网

	return protocol, nil
}

// parseProtocolDetail 解析协议详情页面
func (s *DeBankScraper) parseProtocolDetail(doc *goquery.Document) *ProtocolInfo {
	protocol := &ProtocolInfo{
		IsActive: true,
		RiskLevel: 3,
		Metadata: make(map[string]interface{}),
	}

	// 提取基本信息
	protocol.Name = strings.TrimSpace(doc.Find(".ProtocolHeader__name").Text())
	protocol.Description = strings.TrimSpace(doc.Find(".ProtocolHeader__description").Text())

	// 提取类别
	protocol.Category = strings.TrimSpace(doc.Find(".ProtocolHeader__category").Text())

	// 提取Logo
	logoURL, exists := doc.Find(".ProtocolHeader__logo img").Attr("src")
	if exists {
		protocol.LogoURL = s.normalizeURL(logoURL)
	}

	// 提取网站链接
	websiteURL, exists := doc.Find("a[href*='website']").Attr("href")
	if exists {
		protocol.WebsiteURL = websiteURL
	}

	// 提取Twitter链接
	twitterURL, exists := doc.Find("a[href*='twitter.com']").Attr("href")
	if exists {
		protocol.TwitterURL = twitterURL
	}

	// 提取GitHub链接
	githubURL, exists := doc.Find("a[href*='github.com']").Attr("href")
	if exists {
		protocol.GitHubURL = githubURL
	}

	// 提取TVL
	tvlText := strings.TrimSpace(doc.Find(".ProtocolStats__tvl").Text())
	protocol.TvlUSD = s.parseTVL(tvlText)

	// 提取支持的链
	doc.Find(".ChainList__item").Each(func(i int, sel *goquery.Selection) {
		chainName := strings.TrimSpace(sel.Text())
		chainID := s.getChainID(chainName)
		if chainID > 0 {
			protocol.SupportedChains = append(protocol.SupportedChains, chainID)
		}
	})

	// 提取风险等级（如果有）
	riskText := strings.TrimSpace(doc.Find(".RiskIndicator__level").Text())
	protocol.RiskLevel = s.parseRiskLevel(riskText)

	return protocol
}

// parseTokenItem 解析代币列表项
func (s *DeBankScraper) parseTokenItem(item *goquery.Selection) (TokenInfo, error) {
	var token TokenInfo

	// 提取代币地址
	href, exists := item.Find("a").Attr("href")
	if exists {
		re := regexp.MustCompile(`/token/(0x[a-fA-F0-9]{40})`)
		matches := re.FindStringSubmatch(href)
		if len(matches) > 1 {
			token.TokenAddress = matches[1]
		}
	}

	// 提取代币符号
	token.TokenSymbol = strings.TrimSpace(item.Find(".TokenItem__symbol").Text())

	// 提取代币名称
	token.TokenName = strings.TrimSpace(item.Find(".TokenItem__name").Text())

	// 提取价格
	priceText := strings.TrimSpace(item.Find(".TokenItem__price").Text())
	token.PriceUSD = s.parsePrice(priceText)

	// 提取TVL
	tvlText := strings.TrimSpace(item.Find(".TokenItem__tvl").Text())
	token.TvlUSD = s.parseTVL(tvlText)

	// 提取APY（如果有）
	apyText := strings.TrimSpace(item.Find(".TokenItem__apy").Text())
	if apyText != "" {
		token.SupplyAPY = s.parseAPY(apyText)
	}

	// 设置默认值
	token.TokenDecimals = 18
	token.IsCollateral = strings.Contains(item.Text(), "collateral")
	token.IsBorrowable = strings.Contains(item.Text(), "borrow")
	token.IsSupply = strings.Contains(item.Text(), "supply")

	return token, nil
}

// parseProtocolJSON 解析页面中的JSON数据
func (s *DeBankScraper) parseProtocolJSON(e *colly.HTMLElement) {
	// 查找页面中的JSON数据
	scriptText := e.Text
	re := regexp.MustCompile(`window\.__INITIAL_STATE__\s*=\s*({.*?});`)
	matches := re.FindStringSubmatch(scriptText)
	
	if len(matches) > 1 {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(matches[1]), &data); err == nil {
			s.logger.Debug("成功解析页面JSON数据")
			// 这里可以进一步解析协议数据
		}
	}
}

// parseTokenJSON 解析代币JSON数据
func (s *DeBankScraper) parseTokenJSON(e *colly.HTMLElement) {
	// 类似parseProtocolJSON，解析代币相关的JSON数据
}

// normalizeURL 规范化URL
func (s *DeBankScraper) normalizeURL(url string) string {
	if strings.HasPrefix(url, "//") {
		return "https:" + url
	}
	if strings.HasPrefix(url, "/") {
		return s.config.BaseURL + url
	}
	return url
}

// parseTVL 解析TVL文本
func (s *DeBankScraper) parseTVL(text string) float64 {
	// 移除货币符号和逗号
	text = strings.ReplaceAll(text, "$", "")
	text = strings.ReplaceAll(text, ",", "")
	text = strings.ReplaceAll(text, " ", "")

	// 处理单位
	multiplier := 1.0
	if strings.HasSuffix(strings.ToLower(text), "b") {
		multiplier = 1_000_000_000
		text = text[:len(text)-1]
	} else if strings.HasSuffix(strings.ToLower(text), "m") {
		multiplier = 1_000_000
		text = text[:len(text)-1]
	} else if strings.HasSuffix(strings.ToLower(text), "k") {
		multiplier = 1_000
		text = text[:len(text)-1]
	}

	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0
	}

	return value * multiplier
}

// parsePrice 解析价格文本
func (s *DeBankScraper) parsePrice(text string) float64 {
	text = strings.ReplaceAll(text, "$", "")
	text = strings.ReplaceAll(text, ",", "")
	text = strings.ReplaceAll(text, " ", "")

	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0
	}

	return value
}

// parseAPY 解析APY文本
func (s *DeBankScraper) parseAPY(text string) float64 {
	text = strings.ReplaceAll(text, "%", "")
	text = strings.ReplaceAll(text, " ", "")

	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0
	}

	return value / 100.0
}

// parseRiskLevel 解析风险等级
func (s *DeBankScraper) parseRiskLevel(text string) int {
	switch strings.ToLower(text) {
	case "low", "very low":
		return 1
	case "medium low":
		return 2
	case "medium":
		return 3
	case "medium high":
		return 4
	case "high", "very high":
		return 5
	default:
		return 3 // 默认中等风险
	}
}

// getChainID 根据链名称获取链ID
func (s *DeBankScraper) getChainID(chainName string) int {
	chainMap := map[string]int{
		"ethereum":    1,
		"bsc":         56,
		"polygon":     137,
		"arbitrum":    42161,
		"optimism":    10,
		"avalanche":   43114,
		"fantom":      250,
		"cronos":      25,
		"gnosis":      100,
		"harmony":     1666600000,
		"moonriver":   1285,
		"moonbeam":    1284,
		"celo":        42220,
		"heco":        128,
		"okexchain":   66,
		"kcc":         321,
	}

	chainName = strings.ToLower(chainName)
	if id, exists := chainMap[chainName]; exists {
		return id
	}

	// 尝试部分匹配
	for key, id := range chainMap {
		if strings.Contains(chainName, key) {
			return id
		}
	}

	return 0
}

// Close 关闭抓取器
func (s *DeBankScraper) Close() error {
	s.collector.Wait()
	return nil
}
