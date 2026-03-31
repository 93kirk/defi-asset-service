package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"defi-asset-service/exchange-rate-system/internal/adapter"
	"defi-asset-service/exchange-rate-system/internal/calculator"
	"defi-asset-service/exchange-rate-system/internal/models"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// TestController 汇率测试控制器
type TestController struct {
	engine          *calculator.ExchangeRateEngine
	adapterFactory  *adapter.AdapterFactory
	ethClient       *ethclient.Client
	rpcEndpoints    map[string]string
}

// NewTestController 创建新的测试控制器
func NewTestController(engine *calculator.ExchangeRateEngine, factory *adapter.AdapterFactory) *TestController {
	// 初始化RPC端点
	rpcEndpoints := map[string]string{
		"eth":    "https://eth-mainnet.g.alchemy.com/v2/demo",
		"bsc":    "https://bsc-dataseed.binance.org",
		"matic":  "https://polygon-rpc.com",
		"arb":    "https://arb1.arbitrum.io/rpc",
		"avax":   "https://api.avax.network/ext/bc/C/rpc",
	}
	
	return &TestController{
		engine:         engine,
		adapterFactory: factory,
		rpcEndpoints:   rpcEndpoints,
	}
}

// RegisterTestRoutes 注册测试路由
func (c *TestController) RegisterTestRoutes(r chi.Router) {
	r.Route("/test/exchange-rates", func(r chi.Router) {
		r.Get("/protocols", c.ListSupportedProtocols)
		r.Post("/direct-call", c.DirectContractCall)
		r.Post("/calculate-real", c.CalculateRealRate)
		r.Get("/pool-info/{chain}/{address}", c.GetPoolInfo)
		r.Get("/staking-info/{chain}/{protocol}", c.GetStakingInfo)
		r.Get("/lending-info/{chain}/{protocol}/{token}", c.GetLendingInfo)
		r.Get("/amm-info/{chain}/{pool}", c.GetAMMInfo)
		r.Post("/validate", c.ValidateRate)
		r.Get("/sources", c.ListDataSources)
	})
}

// ListSupportedProtocols 列出支持的真实协议
func (c *TestController) ListSupportedProtocols(w http.ResponseWriter, r *http.Request) {
	realProtocols := []map[string]interface{}{
		// 流动性质押协议
		{
			"id":          "lido",
			"name":        "Lido Finance",
			"type":        "liquid_staking",
			"chain":       "eth",
			"contract":    "0xae7ab96520DE3A18E5e111B5EaAb095312D7fE84", // stETH
			"underlying":  "ETH",
			"receipt":     "stETH",
			"description": "ETH流动性质押，获取stETH",
		},
		{
			"id":          "rocketpool",
			"name":        "Rocket Pool",
			"type":        "liquid_staking",
			"chain":       "eth",
			"contract":    "0xae78736Cd615f374D3085123A210448E74Fc6393", // rETH
			"underlying":  "ETH",
			"receipt":     "rETH",
			"description": "去中心化ETH质押",
		},
		
		// 借贷协议
		{
			"id":          "aave_v3",
			"name":        "Aave V3",
			"type":        "lending",
			"chain":       "eth",
			"contract":    "0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2", // Pool
			"underlying":  "多种资产",
			"receipt":     "aToken",
			"description": "借贷市场，获取计息aToken",
		},
		{
			"id":          "compound_v3",
			"name":        "Compound V3",
			"type":        "lending",
			"chain":       "eth",
			"contract":    "0xc3d688B66703497DAA19211EEdff47f25384cdc3", // USDC市场
			"underlying":  "USDC",
			"receipt":     "cUSDCv3",
			"description": "USDC借贷市场",
		},
		
		// AMM协议
		{
			"id":          "uniswap_v3",
			"name":        "Uniswap V3",
			"type":        "amm",
			"chain":       "eth",
			"contract":    "0xC36442b4a4522E871399CD717aBDD847Ab11FE88", // 位置管理器
			"underlying":  "交易对",
			"receipt":     "NFT",
			"description": "集中流动性AMM",
		},
		{
			"id":          "curve",
			"name":        "Curve Finance",
			"type":        "amm",
			"chain":       "eth",
			"contract":    "0xD51a44d3FaE010294C616388b506AcdA1bfAAE46", // 3pool
			"underlying":  "稳定币",
			"receipt":     "3Crv",
			"description": "稳定币交换协议",
		},
		
		// 收益聚合器
		{
			"id":          "yearn",
			"name":        "Yearn Finance",
			"type":        "yield_aggregator",
			"chain":       "eth",
			"contract":    "0x19D3364A399d251E894aC732651be8B0E4e85001", // yvDAI
			"underlying":  "DAI",
			"receipt":     "yvDAI",
			"description": "DAI收益金库",
		},
		
		// LSD收益协议
		{
			"id":          "etherfi",
			"name":        "ether.fi",
			"type":        "lsd_rewards",
			"chain":       "eth",
			"contract":    "0x4bc3263Eb5bb2Ef7Ad9aB6FB68be80E43b43801F", // eETH
			"underlying":  "ETH",
			"receipt":     "eETH",
			"description": "流动性质押+再质押",
		},
		
		// Origin协议
		{
			"id":          "origin",
			"name":        "Origin Protocol",
			"type":        "stablecoin_yield",
			"chain":       "eth",
			"contract":    "0x2A8e1E676Ec238d8A992307B495b45B3fEAa5e86", // OUSD
			"underlying":  "USD",
			"receipt":     "OUSD",
			"description": "算法稳定币+收益协议",
		},
	}
	
	render.JSON(w, r, map[string]interface{}{
		"protocols": realProtocols,
		"count":     len(realProtocols),
		"timestamp": time.Now().Unix(),
		"note":      "这些是真实可调用的协议合约地址",
	})
}

// DirectContractCall 直接合约调用测试
func (c *TestController) DirectContractCall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Chain       string `json:"chain"`
		Contract    string `json:"contract"`
		Method      string `json:"method"`
		Params      []interface{} `json:"params"`
		ABI         string `json:"abi,omitempty"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}
	
	// 验证请求
	if req.Chain == "" || req.Contract == "" || req.Method == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "chain, contract, and method are required"})
		return
	}
	
	// 执行合约调用
	result, err := c.callContract(r.Context(), req.Chain, req.Contract, req.Method, req.Params, req.ABI)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}
	
	render.JSON(w, r, map[string]interface{}{
		"chain":    req.Chain,
		"contract": req.Contract,
		"method":   req.Method,
		"result":   result,
		"timestamp": time.Now().Unix(),
	})
}

// CalculateRealRate 计算真实汇率
func (c *TestController) CalculateRealRate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProtocolID      string  `json:"protocol_id"`
		UnderlyingToken string  `json:"underlying_token"`
		Amount          float64 `json:"amount"`
		UseRealData     bool    `json:"use_real_data"`
		DataSources     []string `json:"data_sources"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}
	
	// 使用真实数据源计算
	rate, sources, err := c.calculateWithRealSources(r.Context(), req.ProtocolID, req.UnderlyingToken, req.Amount, req.DataSources)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}
	
	receiptAmount := req.Amount * rate
	
	render.JSON(w, r, map[string]interface{}{
		"protocol_id":      req.ProtocolID,
		"underlying_token": req.UnderlyingToken,
		"amount":           req.Amount,
		"exchange_rate":    rate,
		"receipt_amount":   receiptAmount,
		"data_sources":     sources,
		"calculation_time": time.Now().Format(time.RFC3339),
		"note":            "基于真实数据源计算",
	})
}

// GetPoolInfo 获取池子信息
func (c *TestController) GetPoolInfo(w http.ResponseWriter, r *http.Request) {
	chain := chi.URLParam(r, "chain")
	address := chi.URLParam(r, "address")
	
	if chain == "" || address == "" {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "chain and address are required"})
		return
	}
	
	// 根据链和地址获取池子信息
	poolInfo, err := c.fetchPoolInfo(r.Context(), chain, address)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}
	
	render.JSON(w, r, poolInfo)
}

// GetStakingInfo 获取质押信息
func (c *TestController) GetStakingInfo(w http.ResponseWriter, r *http.Request) {
	chain := chi.URLParam(r, "chain")
	protocol := chi.URLParam(r, "protocol")
	
	stakingInfo, err := c.fetchStakingInfo(r.Context(), chain, protocol)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}
	
	render.JSON(w, r, stakingInfo)
}

// GetLendingInfo 获取借贷信息
func (c *TestController) GetLendingInfo(w http.ResponseWriter, r *http.Request) {
	chain := chi.URLParam(r, "chain")
	protocol := chi.URLParam(r, "protocol")
	token := chi.URLParam(r, "token")
	
	lendingInfo, err := c.fetchLendingInfo(r.Context(), chain, protocol, token)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}
	
	render.JSON(w, r, lendingInfo)
}

// GetAMMInfo 获取AMM池信息
func (c *TestController) GetAMMInfo(w http.ResponseWriter, r *http.Request) {
	chain := chi.URLParam(r, "chain")
	pool := chi.URLParam(r, "pool")
	
	ammInfo, err := c.fetchAMMInfo(r.Context(), chain, pool)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}
	
	render.JSON(w, r, ammInfo)
}

// ValidateRate 验证汇率
func (c *TestController) ValidateRate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProtocolID      string  `json:"protocol_id"`
		UnderlyingToken string  `json:"underlying_token"`
		ExpectedRate    float64 `json:"expected_rate"`
		Tolerance       float64 `json:"tolerance"` // 容忍度百分比
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, map[string]string{"error": "invalid request body"})
		return
	}
	
	// 从多个数据源获取真实汇率
	realRate, sources, err := c.calculateWithRealSources(r.Context(), req.ProtocolID, req.UnderlyingToken, 1.0, nil)
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]string{"error": err.Error()})
		return
	}
	
	// 计算偏差
	deviation := (realRate - req.ExpectedRate) / req.ExpectedRate * 100
	isValid := true
	if req.Tolerance > 0 {
		isValid = abs(deviation) <= req.Tolerance
	}
	
	render.JSON(w, r, map[string]interface{}{
		"protocol_id":      req.ProtocolID,
		"underlying_token": req.UnderlyingToken,
		"expected_rate":    req.ExpectedRate,
		"actual_rate":      realRate,
		"deviation_percent": deviation,
		"is_valid":         isValid,
		"tolerance":        req.Tolerance,
		"data_sources":     sources,
		"validation_time":  time.Now().Format(time.RFC3339),
	})
}

// ListDataSources 列出可用数据源
func (c *TestController) ListDataSources(w http.ResponseWriter, r *http.Request) {
	dataSources := []map[string]interface{}{
		{
			"name": "On-chain Contracts",
			"type": "contract",
			"description": "直接调用智能合约",
			"supported_chains": []string{"eth", "bsc", "matic", "arb", "avax"},
			"endpoints": c.rpcEndpoints,
		},
		{
			"name": "DeBank API",
			"type": "api",
			"description": "DeBank协议数据API",
			"url": "https://api.debank.com",
			"rate_limit": "10 requests/second",
		},
		{
			"name": "Chainlink Price Feeds",
			"type": "oracle",
			"description": "Chainlink价格预言机",
			"url": "https://data.chain.link",
			"supported_assets": []string{"ETH/USD", "BTC/USD", "主要代币"},
		},
		{
			"name": "The Graph",
			"type": "subgraph",
			"description": "去中心化索引协议",
			"url": "https://thegraph.com",
			"supported_protocols": []string{"Uniswap", "Aave", "Compound", "Balancer"},
		},
		{
			"name": "Covalent API",
			"type": "api",
			"description": "多链数据API",
			"url": "https://api.covalenthq.com",
			"supported_chains": []string{"所有EVM链"},
		},
	}
	
	render.JSON(w, r, map[string]interface{}{
		"data_sources": dataSources,
		"count":        len(dataSources),
		"timestamp":    time.Now().Unix(),
	})
}

// 私有方法
func (c *TestController) callContract(ctx context.Context, chain, contract, method string, params []interface{}, abi string) (interface{}, error) {
	// 这里应该实现真实的合约调用
	// 暂时返回示例数据
	
	rpcURL, ok := c.rpcEndpoints[chain]
	if !ok {
		return nil, fmt.Errorf("unsupported chain: %s", chain)
	}
	
	// 模拟合约调用结果
	switch strings.ToLower(method) {
	case "getexchangerate":
		// 模拟汇率查询
		return 1.02, nil
	case "totalassets":
		return "1000000000000000000000", nil // 1,000 ETH
	case "totalsupply":
		return "980000000000000000000", nil // 980 stETH
	case "price