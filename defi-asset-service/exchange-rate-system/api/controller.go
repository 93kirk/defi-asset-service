package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"defi-asset-service/exchange-rate-system/internal/calculator"
	"defi-asset-service/exchange-rate-system/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// ExchangeRateController 汇率控制器
type ExchangeRateController struct {
	engine *calculator.ExchangeRateEngine
}

// NewExchangeRateController 创建新的汇率控制器
func NewExchangeRateController(engine *calculator.ExchangeRateEngine) *ExchangeRateController {
	return &ExchangeRateController{
		engine: engine,
	}
}

// RegisterRoutes 注册路由
func (c *ExchangeRateController) RegisterRoutes(r chi.Router) {
	r.Route("/exchange-rates", func(r chi.Router) {
		r.Get("/", c.GetAllProtocols)
		r.Get("/{protocolID}", c.GetProtocolRates)
		r.Post("/calculate", c.CalculateRate)
		r.Get("/{protocolID}/history", c.GetHistoricalRates)
		r.Get("/{protocolID}/tokens", c.GetSupportedTokens)
		r.Get("/health", c.HealthCheck)
	})
}

// GetAllProtocols 获取所有支持的协议
func (c *ExchangeRateController) GetAllProtocols(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// 这里应该从数据库或配置中获取所有协议
	// 暂时返回示例数据
	protocols := []map[string]interface{}{
		{
			"id":   "eth2",
			"name": "Eth2",
			"type": "liquid_staking",
			"chain": "eth",
		},
		{
			"id":   "aave3",
			"name": "Aave V3",
			"type": "lending",
			"chain": "eth",
		},
		{
			"id":   "lido",
			"name": "LIDO",
			"type": "liquid_staking",
			"chain": "eth",
		},
	}
	
	render.JSON(w, r, map[string]interface{}{
		"protocols": protocols,
		"count":     len(protocols),
		"timestamp": time.Now().Unix(),
	})
}

// GetProtocolRates 获取协议汇率信息
func (c *ExchangeRateController) GetProtocolRates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	protocolID := chi.URLParam(r, "protocolID")
	
	if protocolID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "protocolID is required"})
		return
	}
	
	info, err := c.engine.GetProtocolInfo(ctx, protocolID)
	if err != nil {
		render.Status(r, http.StatusNotFound)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}
	
	render.JSON(w, r, info)
}

// CalculateRate 计算汇率
func (c *ExchangeRateController) CalculateRate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	var req models.RateCalculationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}
	
	// 验证请求
	if req.ProtocolID == "" || req.UnderlyingToken == "" || req.Amount <= 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "protocolID, underlyingToken, and amount are required"})
		return
	}
	
	// 设置时间戳
	if req.Timestamp == nil {
		now := time.Now()
		req.Timestamp = &now
	}
	
	response, err := c.engine.CalculateRate(ctx, req)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}
	
	render.JSON(w, r, response)
}

// BatchCalculateRates 批量计算汇率
func (c *ExchangeRateController) BatchCalculateRates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	var requests []models.RateCalculationRequest
	if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}
	
	if len(requests) == 0 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "requests array cannot be empty"})
		return
	}
	
	// 限制批量请求数量
	if len(requests) > 100 {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "maximum 100 requests per batch"})
		return
	}
	
	responses, err := c.engine.BatchCalculateRates(ctx, requests)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}
	
	render.JSON(w, r, map[string]interface{}{
		"responses": responses,
		"count":     len(responses),
		"timestamp": time.Now().Unix(),
	})
}

// GetHistoricalRates 获取历史汇率
func (c *ExchangeRateController) GetHistoricalRates(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	protocolID := chi.URLParam(r, "protocolID")
	
	if protocolID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "protocolID is required"})
		return
	}
	
	// 解析查询参数
	query := models.HistoricalRateQuery{
		ProtocolID: protocolID,
	}
	
	// 开始时间
	if startStr := r.URL.Query().Get("start"); startStr != "" {
		if startTime, err := time.Parse(time.RFC3339, startStr); err == nil {
			query.StartTime = startTime
		}
	} else {
		// 默认最近7天
		query.StartTime = time.Now().Add(-7 * 24 * time.Hour)
	}
	
	// 结束时间
	if endStr := r.URL.Query().Get("end"); endStr != "" {
		if endTime, err := time.Parse(time.RFC3339, endStr); err == nil {
			query.EndTime = endTime
		}
	} else {
		query.EndTime = time.Now()
	}
	
	// 时间间隔
	if interval := r.URL.Query().Get("interval"); interval != "" {
		query.Interval = interval
	} else {
		query.Interval = "1h"
	}
	
	// 限制数量
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			query.Limit = limit
		}
	}
	
	// 代币过滤
	if token := r.URL.Query().Get("token"); token != "" {
		query.UnderlyingToken = token
	}
	
	rates, err := c.engine.GetHistoricalRates(ctx, query)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}
	
	render.JSON(w, r, map[string]interface{}{
		"protocol_id": protocolID,
		"rates":       rates,
		"count":       len(rates),
		"query":       query,
		"timestamp":   time.Now().Unix(),
	})
}

// GetSupportedTokens 获取支持的代币列表
func (c *ExchangeRateController) GetSupportedTokens(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	protocolID := chi.URLParam(r, "protocolID")
	
	if protocolID == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "protocolID is required"})
		return
	}
	
	// 这里应该从适配器获取支持的代币
	// 暂时返回示例数据
	tokens := []models.SupportedToken{
		{
			TokenSymbol:  "ETH",
			TokenName:    "Ethereum",
			Decimals:     18,
			IsUnderlying: true,
			IsReceipt:    false,
			CurrentRate:  1.0,
		},
		{
			TokenSymbol:  "stETH",
			TokenName:    "Lido Staked ETH",
			Decimals:     18,
			IsUnderlying: false,
			IsReceipt:    true,
			CurrentRate:  1.02,
		},
	}
	
	render.JSON(w, r, map[string]interface{}{
		"protocol_id": protocolID,
		"tokens":      tokens,
		"count":       len(tokens),
		"timestamp":   time.Now().Unix(),
	})
}

// HealthCheck 健康检查
func (c *ExchangeRateController) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	health := c.engine.HealthCheck(ctx)
	
	// 添加系统信息
	health["system"] = map[string]interface{}{
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
		"uptime":    time.Since(startTime).String(),
	}
	
	// 确定整体状态
	status := "healthy"
	if adapterErrors, ok := health["adapters"].(map[string]interface{}); ok {
		if errors, ok := adapterErrors["errors"].(int); ok && errors > 0 {
			status = "degraded"
		}
	}
	
	health["status"] = status
	
	// 根据状态设置HTTP状态码
	if status == "healthy" {
		render.Status(r, http.StatusOK)
	} else {
		render.Status(r, http.StatusServiceUnavailable)
	}
	
	render.JSON(w, r, health)
}

// 集成到现有DeFi服务的中间件
func (c *ExchangeRateController) IntegrationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 添加汇率引擎到上下文
		ctx := context.WithValue(r.Context(), "exchange_rate_engine", c.engine)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// 辅助函数：从上下文获取引擎
func GetEngineFromContext(ctx context.Context) (*calculator.ExchangeRateEngine, error) {
	if engine, ok := ctx.Value("exchange_rate_engine").(*calculator.ExchangeRateEngine); ok {
		return engine, nil
	}
	return nil, fmt.Errorf("exchange rate engine not found in context")
}

var startTime = time.Now()