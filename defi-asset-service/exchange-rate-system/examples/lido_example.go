package examples

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// LidoExchangeCalculator Lido汇率计算器
type LidoExchangeCalculator struct {
	client        *ethclient.Client
	stETHContract common.Address
}

// NewLidoExchangeCalculator 创建Lido计算器
func NewLidoExchangeCalculator(rpcURL string) (*LidoExchangeCalculator, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	
	return &LidoExchangeCalculator{
		client:        client,
		stETHContract: common.HexToAddress("0xae7ab96520DE3A18E5e111B5EaAb095312D7fE84"),
	}, nil
}

// CalculateExchangeRate 计算ETH到stETH的兑换率
func (c *LidoExchangeCalculator) CalculateExchangeRate(ctx context.Context) (*big.Float, error) {
	// 1. 获取池子总资产（ETH数量）
	totalAssets, err := c.getTotalAssets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get total assets: %v", err)
	}
	
	// 2. 获取stETH总供应量
	totalSupply, err := c.getTotalSupply(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get total supply: %v", err)
	}
	
	// 3. 计算兑换率：totalAssets / totalSupply
	if totalSupply.Cmp(big.NewInt(0)) == 0 {
		return big.NewFloat(1.0), nil
	}
	
	// 转换为big.Float进行计算
	totalAssetsFloat := new(big.Float).SetInt(totalAssets)
	totalSupplyFloat := new(big.Float).SetInt(totalSupply)
	
	exchangeRate := new(big.Float).Quo(totalAssetsFloat, totalSupplyFloat)
	
	return exchangeRate, nil
}

// CalculateStETHAmount 计算给定ETH数量对应的stETH数量
func (c *LidoExchangeCalculator) CalculateStETHAmount(ctx context.Context, ethAmount *big.Float) (*big.Float, error) {
	exchangeRate, err := c.CalculateExchangeRate(ctx)
	if err != nil {
		return nil, err
	}
	
	// stETH数量 = ETH数量 × 兑换率
	stETHAmount := new(big.Float).Mul(ethAmount, exchangeRate)
	
	return stETHAmount, nil
}

// 私有方法：获取池子总资产
func (c *LidoExchangeCalculator) getTotalAssets(ctx context.Context) (*big.Int, error) {
	// Lido stETH合约的getTotalAssets()函数
	// 函数签名：getTotalAssets() returns (uint256)
	
	// 构建调用数据
	data := common.Hex2Bytes("0x01e1d114") // getTotalAssets()的函数选择器
	
	msg := ethereum.CallMsg{
		To:   &c.stETHContract,
		Data: data,
	}
	
	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		// 如果调用失败，使用备用方法：查询合约余额
		return c.getContractBalance(ctx)
	}
	
	if len(result) == 0 {
		return big.NewInt(0), nil
	}
	
	return new(big.Int).SetBytes(result), nil
}

// 私有方法：获取stETH总供应量
func (c *LidoExchangeCalculator) getTotalSupply(ctx context.Context) (*big.Int, error) {
	// ERC20 totalSupply()函数
	// 函数签名：totalSupply() returns (uint256)
	
	data := common.Hex2Bytes("0x18160ddd") // totalSupply()的函数选择器
	
	msg := ethereum.CallMsg{
		To:   &c.stETHContract,
		Data: data,
	}
	
	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, err
	}
	
	if len(result) == 0 {
		return big.NewInt(0), nil
	}
	
	return new(big.Int).SetBytes(result), nil
}

// 私有方法：获取合约余额（备用方法）
func (c *LidoExchangeCalculator) getContractBalance(ctx context.Context) (*big.Int, error) {
	balance, err := c.client.BalanceAt(ctx, c.stETHContract, nil)
	if err != nil {
		return nil, err
	}
	
	return balance, nil
}

// 使用ABI的完整版本
func (c *LidoExchangeCalculator) CalculateExchangeRateWithABI(ctx context.Context) (*big.Float, error) {
	// Lido stETH合约ABI（简化版）
	const lidoABI = `[
		{
			"constant": true,
			"inputs": [],
			"name": "getTotalAssets",
			"outputs": [{"name": "", "type": "uint256"}],
			"type": "function"
		},
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
			"name": "getPooledEthByShares",
			"outputs": [{"name": "", "type": "uint256"}],
			"type": "function"
		},
		{
			"constant": true,
			"inputs": [],
			"name": "getSharesByPooledEth",
			"outputs": [{"name": "", "type": "uint256"}],
			"type": "function"
		}
	]`
	
	parsedABI, err := abi.JSON(strings.NewReader(lidoABI))
	if err != nil {
		return nil, err
	}
	
	// 方法1：使用getTotalAssets和totalSupply计算
	totalAssetsData, err := parsedABI.Pack("getTotalAssets")
	if err != nil {
		return nil, err
	}
	
	totalSupplyData, err := parsedABI.Pack("totalSupply")
	if err != nil {
		return nil, err
	}
	
	// 调用合约
	totalAssetsResult, err := c.client.CallContract(ctx, ethereum.CallMsg{
		To:   &c.stETHContract,
		Data: totalAssetsData,
	}, nil)
	
	totalSupplyResult, err := c.client.CallContract(ctx, ethereum.CallMsg{
		To:   &c.stETHContract,
		Data: totalSupplyData,
	}, nil)
	
	// 解析结果
	var totalAssets, totalSupply *big.Int
	parsedABI.UnpackIntoInterface(&totalAssets, "getTotalAssets", totalAssetsResult)
	parsedABI.UnpackIntoInterface(&totalSupply, "totalSupply", totalSupplyResult)
	
	// 计算兑换率
	if totalSupply.Cmp(big.NewInt(0)) == 0 {
		return big.NewFloat(1.0), nil
	}
	
	totalAssetsFloat := new(big.Float).SetInt(totalAssets)
	totalSupplyFloat := new(big.Float).SetInt(totalSupply)
	exchangeRate := new(big.Float).Quo(totalAssetsFloat, totalSupplyFloat)
	
	return exchangeRate, nil
}

// 方法2：使用getSharesByPooledEth直接计算
func (c *LidoExchangeCalculator) CalculateDirectExchangeRate(ctx context.Context, ethAmount *big.Int) (*big.Float, error) {
	const lidoABI = `[
		{
			"constant": true,
			"inputs": [{"name": "ethAmount", "type": "uint256"}],
			"name": "getSharesByPooledEth",
			"outputs": [{"name": "", "type": "uint256"}],
			"type": "function"
		}
	]`
	
	parsedABI, err := abi.JSON(strings.NewReader(lidoABI))
	if err != nil {
		return nil, err
	}
	
	// 打包调用数据
	data, err := parsedABI.Pack("getSharesByPooledEth", ethAmount)
	if err != nil {
		return nil, err
	}
	
	// 调用合约
	result, err := c.client.CallContract(ctx, ethereum.CallMsg{
		To:   &c.stETHContract,
		Data: data,
	}, nil)
	if err != nil {
		return nil, err
	}
	
	// 解析结果
	var shares *big.Int
	err = parsedABI.UnpackIntoInterface(&shares, "getSharesByPooledEth", result)
	if err != nil {
		return nil, err
	}
	
	// 计算兑换率：shares / ethAmount
	sharesFloat := new(big.Float).SetInt(shares)
	ethAmountFloat := new(big.Float).SetInt(ethAmount)
	
	if ethAmountFloat.Cmp(big.NewFloat(0)) == 0 {
		return big.NewFloat(1.0), nil
	}
	
	exchangeRate := new(big.Float).Quo(sharesFloat, ethAmountFloat)
	
	return exchangeRate, nil
}

// 示例使用
func ExampleLidoExchange() {
	ctx := context.Background()
	
	// 创建计算器
	calculator, err := NewLidoExchangeCalculator("https://eth-mainnet.g.alchemy.com/v2/demo")
	if err != nil {
		fmt.Printf("Failed to create calculator: %v\n", err)
		return
	}
	
	// 计算兑换率
	exchangeRate, err := calculator.CalculateExchangeRate(ctx)
	if err != nil {
		fmt.Printf("Failed to calculate exchange rate: %v\n", err)
		return
	}
	
	fmt.Printf("Lido ETH → stETH 兑换率: %s\n", exchangeRate.String())
	
	// 计算10 ETH对应的stETH数量
	ethAmount := big.NewFloat(10.0)
	stETHAmount, err := calculator.CalculateStETHAmount(ctx, ethAmount)
	if err != nil {
		fmt.Printf("Failed to calculate stETH amount: %v\n", err)
		return
	}
	
	fmt.Printf("10 ETH = %s stETH\n", stETHAmount.String())
	
	// 使用直接计算方法
	ethAmountInt := big.NewInt(10_000_000_000_000_000_000) // 10 ETH in wei
	directRate, err := calculator.CalculateDirectExchangeRate(ctx, ethAmountInt)
	if err != nil {
		fmt.Printf("Failed to calculate direct exchange rate: %v\n", err)
		return
	}
	
	fmt.Printf("直接计算的兑换率: %s\n", directRate.String())
}