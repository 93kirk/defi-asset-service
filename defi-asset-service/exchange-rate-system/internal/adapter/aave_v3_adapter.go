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

// AaveV3Adapter Aave V3协议适配器
type AaveV3Adapter struct {
	BaseAdapter
	client        *ethclient.Client
	contract      common.Address
	rpcURL        string
	decimals      uint8
	symbol        string
	underlying    common.Address
}

// NewAaveV3Adapter 创建Aave V3适配器
func NewAaveV3Adapter(rpcURL string, contractAddress string) (*AaveV3Adapter, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum: %v", err)
	}
	
	contract := common.HexToAddress(contractAddress)
	
	// 创建适配器
	adapter := &AaveV3Adapter{
		BaseAdapter: BaseAdapter{
			ProtocolID:      "aave_v3",
			ProtocolName:    "Aave V3",
			ProtocolType:    "lending",
			UnderlyingToken: "USDC",
			ReceiptToken:    "aUSDC",
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

// Aave V3 aToken ABI
const aaveV3ABI = `[
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
		"name": "UNDERLYING_ASSET_ADDRESS",
		"outputs": [{"name": "", "type": "address"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "getReserveNormalizedIncome",
		"outputs": [{"name": "", "type": "uint256"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "getReserveData",
		"outputs": [
			{"name": "configuration", "type": "uint256"},
			{"name": "liquidityIndex", "type": "uint128"},
			{"name": "currentLiquidityRate", "type": "uint128"},
			{"name": "currentVariableBorrowRate", "type": "uint128"},
			{"name": "currentStableBorrowRate", "type": "uint128"},
			{"name": "lastUpdateTimestamp", "type": "uint40"},
			{"name": "id", "type": "uint16"}
		],
		"type": "function"
	}
]`

// CalculateRate 计算Aave V3汇率
func (a *AaveV3Adapter) CalculateRate(ctx context.Context, request models.RateCalculationRequest) (*models.RateCalculationResponse, error) {
	startTime := time.Now()
	
	// 1. 获取总供应量
	totalSupply, err := a.getTotalSupply(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get total supply: %v", err)
	}
	
	// 2. 尝试获取标准化收入（汇率计算）
	normalizedIncome, err := a.getNormalizedIncome(ctx)
	var exchangeRate *big.Float
	var rateSource string
	
	if err == nil && normalizedIncome.Cmp(big.NewInt(0)) > 0 {
		// 使用标准化收入计算汇率
		exchangeRate = a.calculateRateFromNormalizedIncome(normalizedIncome)
		rateSource = "normalized_income"
	} else {
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
			"underlying_asset":      a.underlying.Hex(),
			"total_supply":          totalSupply.String(),
			"total_supply_formatted": formatWei(totalSupply, int(a.decimals)),
			"rate_calculation":      rateSource,
			"rpc_url":              a.rpcURL,
		},
	}
	
	if receiptAmount != nil {
		response.ReceiptAmount = receiptAmount
	}
	
	return response, nil
}

// GetProtocolInfo 获取协议信息
func (a *AaveV3Adapter) GetProtocolInfo(ctx context.Context) (*models.ProtocolInfo, error) {
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
			"underlying_asset":      a.underlying.Hex(),
			"total_supply":          formatWei(totalSupply, int(a.decimals)),
			"description":           "Aave V3是领先的去中心化借贷协议，提供aToken作为存款凭证",
			"website":               "https://aave.com",
			"docs":                  "https://docs.aave.com",
			"rate_calculation_note": "Aave V3汇率通过标准化收入计算，反映累积利息",
		},
	}, nil
}

// 私有方法
func (a *AaveV3Adapter) initialize(ctx context.Context) error {
	// 获取代币符号
	symbol, err := a.getSymbol(ctx)
	if err != nil {
		return fmt.Errorf("failed to get symbol: %v", err)
	}
	a.symbol = symbol
	
	// 获取小数位数
	decimals, err := a.getDecimals(ctx)
	if err != nil {
		return fmt.Errorf("failed to get decimals: %v", err)
	}
	a.decimals = decimals
	
	// 获取底层资产地址
	underlying, err := a.getUnderlyingAsset(ctx)
	if err != nil {
		return fmt.Errorf("failed to get underlying asset: %v", err)
	}
	a.underlying = underlying
	
	// 根据符号更新收据代币名称
	if a.symbol != "" {
		a.ReceiptToken = a.symbol
	}
	
	return nil
}

func (a *AaveV3Adapter) getTotalSupply(ctx context.Context) (*big.Int, error) {
	return a.callContractMethod(ctx, "totalSupply")
}

func (a *AaveV3Adapter) getSymbol(ctx context.Context) (string, error) {
	return a.callContractMethodString(ctx, "symbol")
}

func (a *AaveV3Adapter) getDecimals(ctx context.Context) (uint8, error) {
	return a.callContractMethodUint8(ctx, "decimals")
}

func (a *AaveV3Adapter) getUnderlyingAsset(ctx context.Context) (common.Address, error) {
	return a.callContractMethodAddress(ctx, "UNDERLYING_ASSET_ADDRESS")
}

func (a *AaveV3Adapter) getNormalizedIncome(ctx context.Context) (*big.Int, error) {
	return a.callContractMethod(ctx, "getReserveNormalizedIncome")
}

func (a *AaveV3Adapter) calculateRateFromNormalizedIncome(normalizedIncome *big.Int) *big.Float {
	// Aave V3汇率计算：标准化收入 / 1e27
	ray := new(big.Int).Exp(big.NewInt(10), big.NewInt(27), nil)
	rayFloat := new(big.Float).SetInt(ray)
	incomeFloat := new(big.Float).SetInt(normalizedIncome)
	
	return new(big.Float).Quo(incomeFloat, rayFloat)
}

func (a *AaveV3Adapter) estimateExchangeRate() *big.Float {
	// Aave V3的估计汇率（基于典型值）
	return big.NewFloat(1.0023) // 0.23%收益
}

func (a *AaveV3Adapter) estimateAPY() *big.Float {
	// Aave V3 USDC的典型APY
	return big.NewFloat(3.5) // 3.5% APY
}

func (a *AaveV3Adapter) callContractMethod(ctx context.Context, methodName string) (*big.Int, error) {
	parsedABI, err := abi.JSON(strings.NewReader(aaveV3ABI))
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

func (a *AaveV3Adapter) callContractMethodString(ctx context.Context, methodName string) (string, error) {
	parsedABI, err := abi.JSON(strings.NewReader(aaveV3ABI))
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

func (a *AaveV3Adapter) callContractMethodUint8(ctx context.Context, methodName string) (uint8, error) {
	parsedABI, err := abi.JSON(strings.NewReader(aaveV3ABI))
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

func (a *AaveV3Adapter) callContractMethodAddress(ctx context.Context, methodName string) (common.Address, error) {
	parsedABI, err := abi.JSON(strings.NewReader(aaveV3ABI))
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to parse ABI: %v", err)
	}
	
	data, err := parsedABI.Pack(methodName)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to pack method %s: %v", methodName, err)
	}
	
	msg := ethereum.CallMsg{
		To:   &a.contract,
		Data: data,
	}
	
	result, err := a.client.CallContract(ctx, msg, nil)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to call contract method %s: %v", methodName, err)
	}
	
	var value common.Address
	err = parsedABI.UnpackIntoInterface(&value, methodName, result)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to unpack result for method %s: %v", methodName, err)
	}
	
	return value, nil
}

// 实现ProtocolAdapter接口的其他方法
func (a *AaveV3Adapter) Supports(protocolID string) bool {
	return strings.Contains(strings.ToLower(protocolID), "aave")
}

func (a *AaveV3Adapter) GetHistoricalRates(ctx context.Context, query models.HistoricalRateQuery) ([]models.ExchangeRate, error) {
	// 简化实现
	return []models.ExchangeRate{}, nil
}

func (a *AaveV3Adapter) GetSupportedTokens(ctx context.Context, protocolID string) ([]models.SupportedToken, error) {
	return []models.SupportedToken{
		{
			TokenSymbol: a.UnderlyingToken,
			TokenName:   a.UnderlyingToken,
			IsActive:    true,
		},
	}, nil
}

func (a *AaveV3Adapter) GetRateSources(ctx context.Context, protocolID string) ([]models.RateSource, error) {
	return []models.RateSource{
		{
			Name:   "contract_normalized_income",
			Weight: 0.8,
			IsActive: true,
		},
		{
			Name:   "estimated_rate",
			Weight: 0.2,
			IsActive: true,
		},
	}, nil
}

func (a *AaveV3Adapter) HealthCheck(ctx context.Context) error {
	// 检查合约是否存在
	code, err := a.client.CodeAt(ctx, a.contract, nil)
	if err != nil {
		return fmt.Errorf("failed to check contract code: %v", err)
	}
	
	if len(code) == 0 {
		return fmt.Errorf("contract has no code at address %s", a.contract.Hex())
	}
	
	return nil
}