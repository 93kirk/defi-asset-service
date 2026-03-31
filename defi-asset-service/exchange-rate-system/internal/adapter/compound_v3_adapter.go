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

// CompoundV3Adapter Compound V3协议适配器
type CompoundV3Adapter struct {
	BaseAdapter
	client        *ethclient.Client
	contract      common.Address
	rpcURL        string
	decimals      uint8
	symbol        string
}

// NewCompoundV3Adapter 创建Compound V3适配器
func NewCompoundV3Adapter(rpcURL string) (*CompoundV3Adapter, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum: %v", err)
	}
	
	contract := common.HexToAddress("0xc3d688B66703497DAA19211EEdff47f25384cdc3")
	
	// 创建适配器
	adapter := &CompoundV3Adapter{
		BaseAdapter: BaseAdapter{
			ProtocolID:      "compound_v3",
			ProtocolName:    "Compound V3",
			ProtocolType:    "lending",
			UnderlyingToken: "USDC",
			ReceiptToken:    "cUSDC",
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

// Compound V3合约ABI
const compoundV3ABI = `[
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
		"name": "exchangeRate",
		"outputs": [{"name": "", "type": "uint256"}],
		"type": "function"
	}
]`

// CalculateRate 计算Compound V3汇率
func (a *CompoundV3Adapter) CalculateRate(ctx context.Context, request models.RateCalculationRequest) (*models.RateCalculationResponse, error) {
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
			"protocol_version":      "V3",
			"rpc_url":              a.rpcURL,
		},
	}
	
	if receiptAmount != nil {
		response.ReceiptAmount = receiptAmount
	}
	
	return response, nil
}

// GetProtocolInfo 获取协议信息
func (a *CompoundV3Adapter) GetProtocolInfo(ctx context.Context) (*models.ProtocolInfo, error) {
	// 获取总供应量
	totalSupply, err := a.getTotalSupply(ctx)
	if err != nil {
		return nil, err
	}
	
	// 计算TVL（假设USDC价格=$1.0）
	usdPrice := big.NewFloat(1.0)
	tvl := new(big.Float).Mul(
		new(big.Float).SetInt(totalSupply),
		usdPrice,
	)
	
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
			"description":           "Compound V3是升级版的去中心化借贷协议，提供cToken作为存款凭证",
			"website":               "https://compound.finance",
			"docs":                  "https://docs.compound.finance",
			"protocol_features":    []string{"isolated_collateral", "efficient_liquidity", "gas_optimized"},
		},
	}, nil
}

// 私有方法
func (a *CompoundV3Adapter) initialize(ctx context.Context) error {
	// 获取代币符号
	symbol, err := a.getSymbol(ctx)
	if err != nil {
		a.symbol = "cUSDCv3"
	} else {
		a.symbol = symbol
	}
	
	// 获取小数位数
	decimals, err := a.getDecimals(ctx)
	if err != nil {
		a.decimals = 6 // USDC标准
	} else {
		a.decimals = decimals
	}
	
	// 根据符号更新收据代币名称
	if a.symbol != "" {
		a.ReceiptToken = a.symbol
	}
	
	return nil
}

func (a *CompoundV3Adapter) getTotalSupply(ctx context.Context) (*big.Int, error) {
	return a.callContractMethod(ctx, "totalSupply")
}

func (a *CompoundV3Adapter) getSymbol(ctx context.Context) (string, error) {
	return a.callContractMethodString(ctx, "symbol")
}

func (a *CompoundV3Adapter) getDecimals(ctx context.Context) (uint8, error) {
	return a.callContractMethodUint8(ctx, "decimals")
}

func (a *CompoundV3Adapter) getExchangeRate(ctx context.Context) (*big.Float, string, error) {
	exchangeRateRaw, err := a.callContractMethod(ctx, "exchangeRate")
	if err != nil {
		return nil, "", err
	}
	
	// Compound汇率通常乘以1e18
	rate := new(big.Float).Quo(
		new(big.Float).SetInt(exchangeRateRaw),
		new(big.Float).SetInt(big.NewInt(1e18)),
	)
	
	return rate, "contract_exchange_rate", nil
}

func (a *CompoundV3Adapter) estimateExchangeRate() *big.Float {
	// Compound V3的估计汇率
	return big.NewFloat(1.0018) // 0.18%收益
}

func (a *CompoundV3Adapter) estimateAPY() *big.Float {
	// Compound V3 USDC的典型APY
	return big.NewFloat(2.8) // 2.8% APY
}

func (a *CompoundV3Adapter) callContractMethod(ctx context.Context, methodName string) (*big.Int, error) {
	parsedABI, err := abi.JSON(strings.NewReader(compoundV3ABI))
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

func (a *CompoundV3Adapter) callContractMethodString(ctx context.Context, methodName string) (string, error) {
	parsedABI, err := abi.JSON(strings.NewReader(compoundV3ABI))
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

func (a *CompoundV3Adapter) callContractMethodUint8(ctx context.Context, methodName string) (uint8, error) {
	parsedABI, err := abi.JSON(strings.NewReader(compoundV3ABI))
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
func (a *CompoundV3Adapter) Supports(protocolID string) bool {
	return strings.Contains(strings.ToLower(protocolID), "compound")
}

func (a *CompoundV3Adapter) GetHistoricalRates(ctx context.Context, query models.HistoricalRateQuery) ([]models.ExchangeRate, error) {
	return []models.ExchangeRate{}, nil
}

func (a *CompoundV3Adapter) GetSupportedTokens(ctx context.Context, protocolID string) ([]models.SupportedToken, error) {
	return []models.SupportedToken{
		{
			TokenSymbol: a.UnderlyingToken,
			TokenName:   a.UnderlyingToken,
			IsActive:    true,
		},
	}, nil
}

func (a *CompoundV3Adapter) GetRateSources(ctx context.Context, protocolID string) ([]models.RateSource, error) {
	return []models.RateSource{
		{
			Name:     "contract_exchange_rate",
			Weight:   0.7,
			IsActive: true,
		},
		{
			Name:     "estimated_rate",
			Weight:   0.3,
			IsActive: true,
		},
	}, nil
}

func (a *CompoundV3Adapter) HealthCheck(ctx context.Context) error {
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