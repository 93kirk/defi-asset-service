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

// LidoAdapter Lido协议适配器
type LidoAdapter struct {
	BaseAdapter
	client        *ethclient.Client
	contract      common.Address
	rpcURL        string
}

// NewLidoAdapter 创建Lido适配器
func NewLidoAdapter(rpcURL string) (*LidoAdapter, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum: %v", err)
	}
	
	return &LidoAdapter{
		BaseAdapter: BaseAdapter{
			ProtocolID:      "lido",
			ProtocolName:    "Lido Finance",
			ProtocolType:    "liquid_staking",
			UnderlyingToken: "ETH",
			ReceiptToken:    "stETH",
		},
		client:   client,
		contract: common.HexToAddress("0xae7ab96520DE3A18E5e111B5EaAb095312D7fE84"),
		rpcURL:   rpcURL,
	}, nil
}

// Lido合约ABI（汇率计算所需的最小ABI）
const lidoABI = `[
	{
		"constant": true,
		"inputs": [],
		"name": "getTotalPooledEther",
		"outputs": [{"name": "", "type": "uint256"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [],
		"name": "getTotalShares",
		"outputs": [{"name": "", "type": "uint256"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [{"name": "_sharesAmount", "type": "uint256"}],
		"name": "getPooledEthByShares",
		"outputs": [{"name": "", "type": "uint256"}],
		"type": "function"
	},
	{
		"constant": true,
		"inputs": [{"name": "_ethAmount", "type": "uint256"}],
		"name": "getSharesByPooledEth",
		"outputs": [{"name": "", "type": "uint256"}],
		"type": "function"
	}
]`

// CalculateRate 计算Lido汇率
func (a *LidoAdapter) CalculateRate(ctx context.Context, request models.RateCalculationRequest) (*models.RateCalculationResponse, error) {
	startTime := time.Now()
	
	// 1. 获取总质押ETH
	totalPooledEther, err := a.getTotalPooledEther(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get total pooled ether: %v", err)
	}
	
	// 2. 获取总stETH份额
	totalShares, err := a.getTotalShares(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get total shares: %v", err)
	}
	
	// 3. 计算汇率：总质押ETH / 总份额
	var exchangeRate *big.Float
	if totalShares.Cmp(big.NewInt(0)) > 0 {
		exchangeRate = new(big.Float).Quo(
			new(big.Float).SetInt(totalPooledEther),
			new(big.Float).SetInt(totalShares),
		)
	} else {
		exchangeRate = big.NewFloat(1.0)
	}
	
	// 4. 计算APY（基于汇率变化，简化计算）
	apy := a.calculateAPY(exchangeRate)
	
	// 5. 验证汇率（通过getPooledEthByShares）
	verifiedRate, verificationSuccess := a.verifyExchangeRate(ctx, exchangeRate)
	
	// 6. 计算兑换金额（如果请求中有金额）
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
		Source:          "contract_call",
		DataProvider:    "ethereum_mainnet",
		Metadata: map[string]interface{}{
			"contract_address":     a.contract.Hex(),
			"total_pooled_ether":   totalPooledEther.String(),
			"total_shares":         totalShares.String(),
			"verified_rate":        verifiedRate,
			"verification_success": verificationSuccess,
			"rpc_url":              a.rpcURL,
		},
	}
	
	if receiptAmount != nil {
		response.ReceiptAmount = receiptAmount
	}
	
	return response, nil
}

// GetProtocolInfo 获取协议信息
func (a *LidoAdapter) GetProtocolInfo(ctx context.Context) (*models.ProtocolInfo, error) {
	// 获取实时数据
	totalPooledEther, err := a.getTotalPooledEther(ctx)
	if err != nil {
		return nil, err
	}
	
	totalShares, err := a.getTotalShares(ctx)
	if err != nil {
		return nil, err
	}
	
	// 计算TVL（总质押价值，假设ETH价格=$3500）
	ethPrice := big.NewFloat(3500.0)
	tvl := new(big.Float).Mul(
		new(big.Float).SetInt(totalPooledEther),
		ethPrice,
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
			"total_pooled_ether": formatWei(totalPooledEther, 18),
			"total_shares":       formatWei(totalShares, 18),
			"description":        "Lido是最大的流动性质押协议，允许用户质押ETH获得stETH，同时保持流动性",
			"website":           "https://lido.fi",
			"docs":              "https://docs.lido.fi",
		},
	}, nil
}

// 私有方法
func (a *LidoAdapter) getTotalPooledEther(ctx context.Context) (*big.Int, error) {
	return a.callContractMethod(ctx, "getTotalPooledEther")
}

func (a *LidoAdapter) getTotalShares(ctx context.Context) (*big.Int, error) {
	return a.callContractMethod(ctx, "getTotalShares")
}

func (a *LidoAdapter) getPooledEthByShares(ctx context.Context, shares *big.Int) (*big.Int, error) {
	return a.callContractMethodWithArgs(ctx, "getPooledEthByShares", shares)
}

func (a *LidoAdapter) getSharesByPooledEth(ctx context.Context, eth *big.Int) (*big.Int, error) {
	return a.callContractMethodWithArgs(ctx, "getSharesByPooledEth", eth)
}

func (a *LidoAdapter) callContractMethod(ctx context.Context, methodName string) (*big.Int, error) {
	parsedABI, err := abi.JSON(strings.NewReader(lidoABI))
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

func (a *LidoAdapter) callContractMethodWithArgs(ctx context.Context, methodName string, args ...interface{}) (*big.Int, error) {
	parsedABI, err := abi.JSON(strings.NewReader(lidoABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %v", err)
	}
	
	data, err := parsedABI.Pack(methodName, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to pack method %s with args: %v", methodName, err)
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

func (a *LidoAdapter) calculateAPY(exchangeRate *big.Float) *big.Float {
	// 简化APY计算：基于汇率变化
	// 实际应该基于历史汇率变化计算
	baseRate := big.NewFloat(1.0)
	apy := new(big.Float).Sub(exchangeRate, baseRate)
	apy = new(big.Float).Mul(apy, big.NewFloat(100)) // 转换为百分比
	
	// 确保APY为正数
	if apy.Cmp(big.NewFloat(0)) < 0 {
		apy = big.NewFloat(3.5) // 默认3.5% APY
	}
	
	return apy
}

func (a *LidoAdapter) estimateAPY() *big.Float {
	// Lido的典型APY范围
	return big.NewFloat(3.5) // 3.5% APY
}

func (a *LidoAdapter) verifyExchangeRate(ctx context.Context, calculatedRate *big.Float) (float64, bool) {
	// 通过getPooledEthByShares验证汇率
	testShares := big.NewInt(1e18) // 1 stETH
	pooledEth, err := a.getPooledEthByShares(ctx, testShares)
	if err != nil {
		return 0, false
	}
	
	verifiedRate := new(big.Float).Quo(
		new(big.Float).SetInt(pooledEth),
		new(big.Float).SetInt(testShares),
	)
	
	// 计算误差率
	errorRate := new(big.Float).Sub(calculatedRate, verifiedRate)
	errorRate = new(big.Float).Quo(errorRate, calculatedRate)
	errorRate = new(big.Float).Mul(errorRate, big.NewFloat(100))
	
	errorAbs := new(big.Float).Abs(errorRate)
	errorValue, _ := errorAbs.Float64()
	
	// 如果误差小于0.1%，认为验证成功
	success := errorValue < 0.1
	
	verifiedValue, _ := verifiedRate.Float64()
	return verifiedValue, success
}

// 辅助函数
func formatWei(value *big.Int, decimals int) string {
	if value == nil {
		return "0"
	}
	
	divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	result := new(big.Float).Quo(
		new(big.Float).SetInt(value),
		new(big.Float).SetInt(divisor),
	)
	
	return fmt.Sprintf("%.2f", result)
}