package adapter

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"exchange-rate-system/internal/models"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// RocketPoolAdapter Rocket Pool协议适配器
type RocketPoolAdapter struct {
	BaseAdapter
	client        *ethclient.Client
	contract      common.Address
	rpcURL        string
	decimals      uint8
	symbol        string
}

// NewRocketPoolAdapter 创建Rocket Pool适配器
func NewRocketPoolAdapter(rpcURL string) (*RocketPoolAdapter, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum: %v", err)
	}
	
	contract := common.HexToAddress("0xae78736Cd615f374D3085123A210448E74Fc6393")
	
	// 创建适配器
	adapter := &RocketPoolAdapter{
		BaseAdapter: BaseAdapter{
			ProtocolID:      "rocketpool",
			ProtocolName:    "Rocket Pool",
			ProtocolType:    "liquid_staking",
			UnderlyingToken: "ETH",
			ReceiptToken:    "rETH",
		},
		client:   client,
		contract: contract,
		rpcURL:   rpcURL,
	}
	
	// 初始化合约信息
	ctx := context.Background()
	if err := adapter.initialize(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize adapter: %v", err)
	}
	
	return adapter, nil
}

// Rocket Pool rETH合约ABI
const rocketPoolABI = `[
	{
		"constant": true,
		"inputs": [],
		"name": "totalSupply",
		"outputs": [{"name": "", "type": "uint256"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "decimals",
		"outputs": [{"name": "", "type": "uint8"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "symbol",
		"outputs": [{"name": "", "type": "string"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "getExchangeRate",
		"outputs": [{"name": "", "type": "uint256"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "getEthValue",
		"outputs": [{"name": "", "type": "uint256"}],
		"type": "function"
	}
]`

// CalculateRate 计算Rocket Pool汇率
func (a *RocketPoolAdapter) CalculateRate(ctx context.Context, request models.RateCalculationRequest) (*models.RateCalculationResponse, error) {
	startTime := time.Now()
	
	// 1. 获取总供应量
	totalSupply, err := a.getTotalSupply(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get total supply: %v", err)
	}
	
	// 2. 获取汇率
	exchangeRate, rateSource, err := a.getExchangeRate(ctx)
	if err != nil {
		// 使用估计汇率
		exchangeRate = a.estimateExchangeRate()
		rateSource = "estimated"
	}
	
	// 3. 计算APY
	apy := a.estimateAPY()
	
	// 4. 计算兑换金额
	var receiptAmount *big.Float
	if request.Amount > 0 {
		receiptAmount = new(big.Float).Mul(big.NewFloat(request.Amount), exchangeRate)
	}
	
	response := &models.RateCalculationResponse{
		ProtocolID:      a.ProtocolID,
		ProtocolName:    a.ProtocolName,
		UnderlyingToken: a.UnderlyingToken,
		ReceiptToken:    a.ReceiptToken,
		ExchangeRate:    exchangeRate,
		APY:             apy,
		CalculationTime: time.Since(startTime),
		UpdatedAt:       time.Now(),
		Source:          rateSource,
		DataProvider:    "ethereum_mainnet",
		Metadata: map[string]interface{}{
			"contract_address":       a.contract.Hex(),
			"symbol":                a.symbol,
			"decimals":              a.decimals,
			"total_supply":          totalSupply.String(),
			"total_supply_formatted": formatWei(totalSupply, int(a.decimals)),
			"rate_calculation":      rateSource,
			"protocol_type":         "decentralized_liquid_staking",
			"description":           "Rocket Pool是去中心化的流动性质押协议，rETH代表质押的ETH",
			"rpc_url":              a.rpcURL,
		},
	}
	
	if receiptAmount != nil {
		response.ReceiptAmount = receiptAmount
	}
	
	return response, nil
}

// GetProtocolInfo 获取协议信息
func (a *RocketPoolAdapter) GetProtocolInfo(ctx context.Context) (*models.ProtocolInfo, error) {
	// 获取总供应量
	totalSupply, err := a.getTotalSupply(ctx)
	if err != nil {
		return nil, err
	}
	
	// 计算TVL（估计）
	// 假设汇率1.019，ETH价格=$3500
	ethPrice := big.NewFloat(3500.0)
	estimatedRate := big.NewFloat(1.019)
	
	tvl := new(big.Float).Mul(
		new(big.Float).SetInt(totalSupply),
		estimatedRate,
	)
	tvl = new(big.Float).Mul(tvl, ethPrice)
	
	return &models.ProtocolInfo{
		ProtocolID:      a.ProtocolID,
		ProtocolName:    a.ProtocolName,
		ProtocolType:    a.ProtocolType,
		UnderlyingToken: a.UnderlyingToken,
		ReceiptToken:    a.ReceiptToken,
		ContractAddress: a.contract.Hex(),
		TVL:             tvl,
		APY:             a.estimateAPY(),
		IsActive:        true,
		LastUpdated:     time.Now(),
		Metadata: map[string]interface{}{
			"symbol":                a.symbol,
			"decimals":              a.decimals,
			"total_supply":          formatWei(totalSupply, int(a.decimals)),
			"estimated_exchange_rate": 1.019,
			"description":           "Rocket Pool是领先的去中心化流动性质押协议，通过节点运营商网络实现ETH质押",
			"website":               "https://rocketpool.net",
			"docs":                  "https://docs.rocketpool.net",
			"features":             []string{"decentralized", "node_operator_network", "permissionless"},
		},
	}, nil
}

// 私有方法
func (a *RocketPoolAdapter) initialize(ctx context.Context) error {
	// 获取代币符号
	symbol, err := a.getSymbol(ctx)
	if err != nil {
		a.symbol = "rETH"
	} else {
		a.symbol = symbol
	}
	
	// 获取小数位数
	decimals, err := a.getDecimals(ctx)
	if err != nil {
		a.decimals = 18 // ETH标准
	} else {
		a.decimals = decimals
	}
	
	// 根据符号更新收据代币名称
	if a.symbol != "" {
		a.ReceiptToken = a.symbol
	}
	
	return nil
}

func (a *RocketPoolAdapter) getTotalSupply(ctx context.Context) (*big.Int, error) {
	return a.callContractMethod(ctx, "totalSupply")
}

func (a *RocketPoolAdapter) getSymbol(ctx context.Context) (string, error) {
	return a.callContractMethodString(ctx, "symbol")
}

func (a *RocketPoolAdapter) getDecimals(ctx context.Context) (uint8, error) {
	return a.callContractMethodUint8(ctx, "decimals")
}

func (a *RocketPoolAdapter) getExchangeRate(ctx context.Context) (*big.Float, string, error) {
	// 尝试getExchangeRate方法
	exchangeRateRaw, err := a.callContractMethod(ctx, "getExchangeRate")
	if err != nil {
		// 尝试getEthValue方法
		ethValue, err := a.callContractMethod(ctx, "getEthValue")
		if err != nil {
			return nil, "", err
		}
		
		// getEthValue返回1 rETH对应的ETH数量
		rate := new(big.Float).Quo(
			new(big.Float).SetInt(ethValue),
			new(big.Float).SetInt(big.NewInt(1e18)),
		)
		
		return rate, "contract_getEthValue", nil
	}
	
	// getExchangeRate通常乘以1e18
	rate := new(big.Float).Quo(
		new(big.Float).SetInt(exchangeRateRaw),
		new(big.Float).SetInt(big.NewInt(1e18)),
	)
	
	return rate, "contract_getExchangeRate", nil
}

func (a *RocketPoolAdapter) estimateExchangeRate() *big.Float {
	// Rocket Pool rETH的估计汇率
	return big.NewFloat(1.019) // 1.9%收益
}

func (a *RocketPoolAdapter) estimateAPY() *big.Float {
	// Rocket Pool的典型APY
	return big.NewFloat(3.2) // 3.2% APY
}

func (a *RocketPoolAdapter) callContractMethod(ctx context.Context, methodName string) (*big.Int, error) {
	parsedABI, err := abi.JSON(strings.NewReader(rocketPoolABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %v", err)
	}
	
	data, err := parsedABI.Pack(methodName)
	if err != nil {
		return nil, fmt.Errorf("failed to pack method %s: %v", methodName, err)
	}
	
	msg := ethereum.CallMsg{
		To:   &a.contract,
		Data: data,
	}
	
	result, err := a.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call contract method %s: %v", methodName, err)
	}
	
	var value *big.Int
	err = parsedABI.UnpackIntoInterface(&value, methodName, result)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack result for method %s: %v", methodName, err)
	}
	
	return value, nil
}

func (a *RocketPoolAdapter) callContractMethodString(ctx context.Context, methodName string) (string, error) {
	parsedABI, err := abi.JSON(strings.NewReader(rocketPoolABI))
	if err != nil {
		return "", fmt.Errorf("failed to parse ABI: %v", err)
	}
	
	data, err := parsedABI.Pack(methodName)
	if err != nil {
		return "", fmt.Errorf("failed to pack method %s: %v", methodName, err)
	}
	
	msg := ethereum.CallMsg{
		To:   &a.contract,
		Data: data,
	}
	
	result, err := a.client.CallContract(ctx, msg, nil)
	if err != nil {
		return "", fmt.Errorf("failed to call contract method %s: %v", methodName, err)
	}
	
	var value string
	err = parsedABI.UnpackIntoInterface(&value, methodName, result)
	if err != nil {
		return "", fmt.Errorf("failed to unpack result for method %s: %v", methodName, err)
	}
	
	return value, nil
}

func (a *RocketPoolAdapter) callContractMethodUint8(ctx context.Context, methodName string) (uint8, error) {
	parsedABI, err := abi.JSON(strings.NewReader(rocketPoolABI))
	if err != nil {
		return 0, fmt.Errorf("failed to parse ABI: %v", err)
	}
	
	data, err := parsedABI.Pack(methodName)
	if err != nil {
		return 0, fmt.Errorf("failed to pack method %s: %v", methodName, err)
	}
	
	msg := ethereum.CallMsg{
		To:   &a.contract,
		Data: data,
	}
	
	result, err := a.client.CallContract(ctx, msg, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to call contract method %s: %v", methodName, err)
	}
	
	var value uint8
	err = parsedABI.UnpackIntoInterface(&value, methodName, result)
	if err != nil {
		return 0, fmt.Errorf("failed to unpack result for method %s: %v", methodName, err)
	}
	
	return value, nil
}

// 实现ProtocolAdapter接口的其他方法
func (a *RocketPoolAdapter) Supports(protocolID string) bool {
	return strings.Contains(strings.ToLower(protocolID), "rocketpool") || 
	       strings.Contains(strings.ToLower(protocolID), "reth")
}

func (a *RocketPoolAdapter) GetHistoricalRates(ctx context.Context, query models.HistoricalRateQuery) ([]models.ExchangeRate, error) {
	return []models.ExchangeRate{}, nil
}

func (a *RocketPoolAdapter) GetSupportedTokens(ctx context.Context, protocolID string) ([]models.SupportedToken, error) {
	return []models.SupportedToken{
		{
			TokenSymbol: a.UnderlyingToken,
			TokenName:   a.UnderlyingToken,
			IsActive:    true,
		},
	}, nil
}

func (a *RocketPoolAdapter) GetRateSources(ctx context.Context, protocolID string) ([]models.RateSource, error) {
	return []models.RateSource{
		{
			Name:     "contract_getExchangeRate",
			Weight:   0.6,
			IsActive: true,
		},
		{
			Name:     "contract_getEthValue",
			Weight:   0.3,
			IsActive: true,
		},
		{
			Name:     "estimated_rate",
			Weight:   0.1,
			IsActive: true,
		},
	}, nil
}

func (a *RocketPoolAdapter) HealthCheck(ctx context.Context) error {
	// 检查合约是否存在
	code, err := a.client.CodeAt(ctx, a.contract, nil)
	if err != nil {
		return fmt.Errorf("failed to check contract code: %v", err)
	}
	
	if len(code) == 0 {
		return fmt.Errorf("contract has no code at address %s", a.contract.Hex())
	}
	
	// 尝试获取总供应量
	_, err = a.getTotalSupply(ctx)
	if err != nil {
		return fmt.Errorf("failed to get total supply: %v", err)
	}
	
	return nil
}