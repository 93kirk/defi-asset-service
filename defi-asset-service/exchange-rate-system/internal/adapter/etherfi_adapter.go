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

// EtherFiAdapter ether.fi协议适配器
type EtherFiAdapter struct {
	BaseAdapter
	client        *ethclient.Client
	contract      common.Address
	rpcURL        string
	decimals      uint8
	symbol        string
}

// NewEtherFiAdapter 创建ether.fi适配器
func NewEtherFiAdapter(rpcURL string) (*EtherFiAdapter, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum: %v", err)
	}
	
	contract := common.HexToAddress("0x4bc3263Eb5bb2Ef7Ad9aB6FB68be80E43b43801F")
	
	// 创建适配器
	adapter := &EtherFiAdapter{
		BaseAdapter: BaseAdapter{
			ProtocolID:      "etherfi",
			ProtocolName:    "ether.fi",
			ProtocolType:    "lsd_rewards",
			UnderlyingToken: "ETH",
			ReceiptToken:    "eETH",
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

// ether.fi eETH合约ABI（基于测试结果）
const etherFiABI = `[
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
		"name": "balanceOf",
		"outputs": [{"name": "", "type": "uint256"}],
		"type": "function"
	}
]`

// CalculateRate 计算ether.fi汇率
func (a *EtherFiAdapter) CalculateRate(ctx context.Context, request models.RateCalculationRequest) (*models.RateCalculationResponse, error) {
	startTime := time.Now()
	
	// 1. 获取总供应量
	totalSupply, err := a.getTotalSupply(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get total supply: %v", err)
	}
	
	// 2. ether.fi是流动性质押+再质押协议
	// 由于无法直接获取总资产，使用估计汇率
	exchangeRate := a.estimateExchangeRate()
	
	// 3. 计算APY（包含再质押收益）
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
		Source:          "estimated",
		DataProvider:    "ethereum_mainnet",
		Metadata: map[string]interface{}{
			"contract_address":       a.contract.Hex(),
			"symbol":                a.symbol,
			"decimals":              a.decimals,
			"total_supply":          totalSupply.String(),
			"total_supply_formatted": formatWei(totalSupply, int(a.decimals)),
			"rate_calculation":      "estimated",
			"protocol_type":         "liquid_staking_restaking",
			"description":           "ether.fi是流动性质押+再质押协议，eETH包含LSD收益和再质押收益",
			"rpc_url":              a.rpcURL,
		},
	}
	
	if receiptAmount != nil {
		response.ReceiptAmount = receiptAmount
	}
	
	return response, nil
}

// GetProtocolInfo 获取协议信息
func (a *EtherFiAdapter) GetProtocolInfo(ctx context.Context) (*models.ProtocolInfo, error) {
	// 获取总供应量
	totalSupply, err := a.getTotalSupply(ctx)
	if err != nil {
		return nil, err
	}
	
	// 计算TVL（估计）
	// 假设汇率1.021，ETH价格=$3500
	ethPrice := big.NewFloat(3500.0)
	estimatedRate := big.NewFloat(1.021)
	
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
			"estimated_exchange_rate": 1.021,
			"description":           "ether.fi是领先的流动性质押+再质押协议，eETH代表质押的ETH加上EigenLayer再质押收益",
			"website":               "https://www.ether.fi",
			"docs":                  "https://docs.ether.fi",
			"features":             []string{"liquid_staking", "restaking", "eigenlayer_integration"},
		},
	}, nil
}

// 私有方法
func (a *EtherFiAdapter) initialize(ctx context.Context) error {
	// 获取代币符号
	symbol, err := a.getSymbol(ctx)
	if err != nil {
		// 如果失败，使用默认值
		a.symbol = "eETH"
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

func (a *EtherFiAdapter) getTotalSupply(ctx context.Context) (*big.Int, error) {
	return a.callContractMethod(ctx, "totalSupply")
}

func (a *EtherFiAdapter) getSymbol(ctx context.Context) (string, error) {
	return a.callContractMethodString(ctx, "symbol")
}

func (a *EtherFiAdapter) getDecimals(ctx context.Context) (uint8, error) {
	return a.callContractMethodUint8(ctx, "decimals")
}

func (a *EtherFiAdapter) estimateExchangeRate() *big.Float {
	// ether.fi eETH的估计汇率
	// 包含流动性质押收益 + EigenLayer再质押收益
	return big.NewFloat(1.021) // 2.1%收益
}

func (a *EtherFiAdapter) estimateAPY() *big.Float {
	// ether.fi的典型APY（LSD + 再质押）
	return big.NewFloat(4.2) // 4.2% APY
}

func (a *EtherFiAdapter) callContractMethod(ctx context.Context, methodName string) (*big.Int, error) {
	parsedABI, err := abi.JSON(strings.NewReader(etherFiABI))
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

func (a *EtherFiAdapter) callContractMethodString(ctx context.Context, methodName string) (string, error) {
	parsedABI, err := abi.JSON(strings.NewReader(etherFiABI))
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

func (a *EtherFiAdapter) callContractMethodUint8(ctx context.Context, methodName string) (uint8, error) {
	parsedABI, err := abi.JSON(strings.NewReader(etherFiABI))
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
func (a *EtherFiAdapter) Supports(protocolID string) bool {
	return strings.Contains(strings.ToLower(protocolID), "etherfi") || 
	       strings.Contains(strings.ToLower(protocolID), "eeth")
}

func (a *EtherFiAdapter) GetHistoricalRates(ctx context.Context, query models.HistoricalRateQuery) ([]models.ExchangeRate, error) {
	return []models.ExchangeRate{}, nil
}

func (a *EtherFiAdapter) GetSupportedTokens(ctx context.Context, protocolID string) ([]models.SupportedToken, error) {
	return []models.SupportedToken{
		{
			TokenSymbol: a.UnderlyingToken,
			TokenName:   a.UnderlyingToken,
			IsActive:    true,
		},
	}, nil
}

func (a *EtherFiAdapter) GetRateSources(ctx context.Context, protocolID string) ([]models.RateSource, error) {
	return []models.RateSource{
		{
			Name:     "estimated_lsd_restaking",
			Weight:   1.0,
			IsActive: true,
		},
	}, nil
}

func (a *EtherFiAdapter) HealthCheck(ctx context.Context) error {
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