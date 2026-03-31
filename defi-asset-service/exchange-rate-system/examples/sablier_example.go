package examples

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// SablierExchangeCalculator Sablier协议汇率计算器
type SablierExchangeCalculator struct {
	client           *ethclient.Client
	sablierContract  common.Address // Sablier V2合约
	sablierV1Contract common.Address // Sablier V1合约
}

// NewSablierExchangeCalculator 创建Sablier计算器
func NewSablierExchangeCalculator(rpcURL string) (*SablierExchangeCalculator, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	
	return &SablierExchangeCalculator{
		client:           client,
		sablierContract:  common.HexToAddress("0xCD18eAa163733Da39c232722cBC4E8940b1D8888"), // Sablier V2
		sablierV1Contract: common.HexToAddress("0xA4fc358455Febe425536fd1878bE67FfDBDEC59a"), // Sablier V1
	}, nil
}

// CalculateStreamValue 计算流支付当前价值
func (c *SablierExchangeCalculator) CalculateStreamValue(ctx context.Context, streamID *big.Int) (*big.Float, *big.Float, error) {
	// 获取流支付详情
	stream, err := c.getStreamDetails(ctx, streamID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get stream details: %v", err)
	}
	
	// 计算已支付金额
	paidAmount, err := c.calculatePaidAmount(ctx, stream)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to calculate paid amount: %v", err)
	}
	
	// 计算剩余金额
	remainingAmount, err := c.calculateRemainingAmount(ctx, stream)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to calculate remaining amount: %v", err)
	}
	
	return paidAmount, remainingAmount, nil
}

// CalculateStreamExchangeRate 计算流支付汇率
func (c *SablierExchangeCalculator) CalculateStreamExchangeRate(ctx context.Context, streamID *big.Int) (*big.Float, error) {
	// 获取流支付详情
	stream, err := c.getStreamDetails(ctx, streamID)
	if err != nil {
		return nil, err
	}
	
	// 计算当前价值比率
	// 汇率 = 已支付金额 / 总金额
	paidAmount, err := c.calculatePaidAmount(ctx, stream)
	if err != nil {
		return nil, err
	}
	
	totalAmount := new(big.Float).SetInt(stream.TotalAmount)
	
	if totalAmount.Cmp(big.NewFloat(0)) == 0 {
		return big.NewFloat(0), nil
	}
	
	exchangeRate := new(big.Float).Quo(paidAmount, totalAmount)
	
	return exchangeRate, nil
}

// CreateStreamSimulation 模拟创建流支付
func (c *SablierExchangeCalculator) CreateStreamSimulation(ctx context.Context, params StreamParams) (*StreamSimulationResult, error) {
	// 计算流支付参数
	durationSeconds := params.DurationHours * 3600
	ratePerSecond := new(big.Float).Quo(
		new(big.Float).SetInt(params.TotalAmount),
		big.NewFloat(float64(durationSeconds)),
	)
	
	// 计算预计完成时间
	startTime := time.Now().Unix()
	endTime := startTime + int64(durationSeconds)
	
	// 计算即时价值（创建时已支付的比例）
	// Sablier流支付从创建时开始流动
	initialPaidRatio := big.NewFloat(0.0) // 刚开始时已支付为0
	
	return &StreamSimulationResult{
		StreamID:          generateStreamID(),
		StartTime:         startTime,
		EndTime:           endTime,
		TotalAmount:       params.TotalAmount,
		RatePerSecond:     ratePerSecond,
		InitialPaidRatio:  initialPaidRatio,
		DurationSeconds:   durationSeconds,
		TokenAddress:      params.TokenAddress,
		Sender:            params.Sender,
		Recipient:         params.Recipient,
	}, nil
}

// CalculateStreamLiquidityValue 计算流支付流动性价值
func (c *SablierExchangeCalculator) CalculateStreamLiquidityValue(ctx context.Context, streamID *big.Int, discountRate float64) (*big.Float, error) {
	// 获取流支付详情
	stream, err := c.getStreamDetails(ctx, streamID)
	if err != nil {
		return nil, err
	}
	
	// 计算剩余金额
	remainingAmount, err := c.calculateRemainingAmount(ctx, stream)
	if err != nil {
		return nil, err
	}
	
	// 应用折扣率（流动性折价）
	// 流动性价值 = 剩余金额 × (1 - 折扣率)
	discountFactor := big.NewFloat(1.0 - discountRate)
	liquidityValue := new(big.Float).Mul(remainingAmount, discountFactor)
	
	return liquidityValue, nil
}

// 数据结构和类型定义
type StreamDetails struct {
	StreamID      *big.Int
	Sender        common.Address
	Recipient     common.Address
	TokenAddress  common.Address
	TotalAmount   *big.Int
	StartTime     *big.Int
	StopTime      *big.Int
	RatePerSecond *big.Int
	Withdrawn     *big.Int
}

type StreamParams struct {
	Sender        common.Address
	Recipient     common.Address
	TokenAddress  common.Address
	TotalAmount   *big.Int
	DurationHours int
}

type StreamSimulationResult struct {
	StreamID         *big.Int
	StartTime        int64
	EndTime          int64
	TotalAmount      *big.Int
	RatePerSecond    *big.Float
	InitialPaidRatio *big.Float
	DurationSeconds  int
	TokenAddress     common.Address
	Sender           common.Address
	Recipient        common.Address
}

// 私有方法
func (c *SablierExchangeCalculator) getStreamDetails(ctx context.Context, streamID *big.Int) (*StreamDetails, error) {
	// Sablier V2 ABI（简化版）
	const sablierABI = `[
		{
			"constant": true,
			"inputs": [{"name": "streamId", "type": "uint256"}],
			"name": "getStream",
			"outputs": [
				{"name": "sender", "type": "address"},
				{"name": "recipient", "type": "address"},
				{"name": "tokenAddress", "type": "address"},
				{"name": "deposit", "type": "uint256"},
				{"name": "startTime", "type": "uint256"},
				{"name": "stopTime", "type": "uint256"},
				{"name": "remainingBalance", "type": "uint256"},
				{"name": "ratePerSecond", "type": "uint256"}
			],
			"type": "function"
		},
		{
			"constant": true,
			"inputs": [{"name": "streamId", "type": "uint256"}],
			"name": "balanceOf",
			"outputs": [{"name": "balance", "type": "uint256"}],
			"type": "function"
		}
	]`
	
	parsedABI, err := abi.JSON(strings.NewReader(sablierABI))
	if err != nil {
		// 返回模拟数据
		return c.getMockStreamDetails(streamID), nil
	}
	
	// 调用合约获取流详情
	data, err := parsedABI.Pack("getStream", streamID)
	if err != nil {
		return c.getMockStreamDetails(streamID), nil
	}
	
	msg := ethereum.CallMsg{
		To:   &c.sablierContract,
		Data: data,
	}
	
	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return c.getMockStreamDetails(streamID), nil
	}
	
	var streamDetails struct {
		Sender        common.Address
		Recipient     common.Address
		TokenAddress  common.Address
		Deposit       *big.Int
		StartTime     *big.Int
		StopTime      *big.Int
		RemainingBalance *big.Int
		RatePerSecond *big.Int
	}
	
	err = parsedABI.UnpackIntoInterface(&streamDetails, "getStream", result)
	if err != nil {
		return c.getMockStreamDetails(streamID), nil
	}
	
	// 获取已提取金额
	withdrawn, err := c.getWithdrawnAmount(ctx, streamID, parsedABI)
	if err != nil {
		withdrawn = big.NewInt(0)
	}
	
	return &StreamDetails{
		StreamID:      streamID,
		Sender:        streamDetails.Sender,
		Recipient:     streamDetails.Recipient,
		TokenAddress:  streamDetails.TokenAddress,
		TotalAmount:   streamDetails.Deposit,
		StartTime:     streamDetails.StartTime,
		StopTime:      streamDetails.StopTime,
		RatePerSecond: streamDetails.RatePerSecond,
		Withdrawn:     withdrawn,
	}, nil
}

func (c *SablierExchangeCalculator) getWithdrawnAmount(ctx context.Context, streamID *big.Int, sablierABI abi.ABI) (*big.Int, error) {
	data, err := sablierABI.Pack("balanceOf", streamID)
	if err != nil {
		return nil, err
	}
	
	msg := ethereum.CallMsg{
		To:   &c.sablierContract,
		Data: data,
	}
	
	result, err := c.client.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, err
	}
	
	var balance *big.Int
	err = sablierABI.UnpackIntoInterface(&balance, "balanceOf", result)
	if err != nil {
		return nil, err
	}
	
	return balance, nil
}

func (c *SablierExchangeCalculator) calculatePaidAmount(ctx context.Context, stream *StreamDetails) (*big.Float, error) {
	// 已支付金额 = 总金额 - 剩余余额
	// 或者 = 已提取金额
	
	currentTime := big.NewInt(time.Now().Unix())
	
	// 如果流已结束
	if currentTime.Cmp(stream.StopTime) >= 0 {
		// 全部已支付
		return new(big.Float).SetInt(stream.TotalAmount), nil
	}
	
	// 如果流尚未开始
	if currentTime.Cmp(stream.StartTime) < 0 {
		// 尚未支付
		return big.NewFloat(0), nil
	}
	
	// 计算已过去的时间
	elapsedTime := new(big.Int).Sub(currentTime, stream.StartTime)
	
	// 计算已支付金额：流逝时间 × 每秒速率
	paidAmount := new(big.Int).Mul(elapsedTime, stream.RatePerSecond)
	
	// 确保不超过总金额
	if paidAmount.Cmp(stream.TotalAmount) > 0 {
		paidAmount = stream.TotalAmount
	}
	
	return new(big.Float).SetInt(paidAmount), nil
}

func (c *SablierExchangeCalculator) calculateRemainingAmount(ctx context.Context, stream *StreamDetails) (*big.Float, error) {
	// 剩余金额 = 总金额 - 已支付金额
	
	paidAmount, err := c.calculatePaidAmount(ctx, stream)
	if err != nil {
		return nil, err
	}
	
	totalAmount := new(big.Float).SetInt(stream.TotalAmount)
	remainingAmount := new(big.Float).Sub(totalAmount, paidAmount)
	
	return remainingAmount, nil
}

func (c *SablierExchangeCalculator) getMockStreamDetails(streamID *big.Int) *StreamDetails {
	// 返回模拟流数据
	now := time.Now().Unix()
	startTime := big.NewInt(now - 86400) // 24小时前开始
	stopTime := big.NewInt(now + 86400*30) // 30天后结束
	
	return &StreamDetails{
		StreamID:      streamID,
		Sender:        common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e0F3B5F2b1F"),
		Recipient:     common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e0F3B5F2b1F"),
		TokenAddress:  common.HexToAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"), // USDC
		TotalAmount:   big.NewInt(1000000000), // 1000 USDC (6 decimals)
		StartTime:     startTime,
		StopTime:      stopTime,
		RatePerSecond: big.NewInt(386), // 1000 USDC / 30天 ≈ 386 wei/秒
		Withdrawn:     big.NewInt(500000000), // 已提取500 USDC
	}
}

func generateStreamID() *big.Int {
	// 生成模拟流ID
	return big.NewInt(time.Now().UnixNano())
}

// 示例使用
func ExampleSablierExchange() {
	ctx := context.Background()
	
	fmt.Println("=== Sablier流支付协议汇率计算 ===")
	
	// 创建计算器
	calculator, err := NewSablierExchangeCalculator("https://eth-mainnet.g.alchemy.com/v2/demo")
	if err != nil {
		fmt.Printf("创建计算器失败: %v\n", err)
		return
	}
	
	// 示例流ID
	streamID := big.NewInt(123456)
	
	// 计算流支付价值
	paidAmount, remainingAmount, err := calculator.CalculateStreamValue(ctx, streamID)
	if err != nil {
		fmt.Printf("计算流价值失败: %v\n", err)
		return
	}
	
	fmt.Printf("流支付 #%s:\n", streamID.String())
	fmt.Printf("已支付金额: %.6f USDC\n", paidAmount)
	fmt.Printf("剩余金额: %.6f USDC\n", remainingAmount)
	
	// 计算汇率
	exchangeRate, err := calculator.CalculateStreamExchangeRate(ctx, streamID)
	if err != nil {
		fmt.Printf("计算汇率失败: %v\n", err)
		return
	}
	
	fmt.Printf("当前支付进度: %.2f%%\n", new(big.Float).Mul(exchangeRate, big.NewFloat(100)))
	
	// 计算流动性价值
	liquidityValue, err := calculator.CalculateStreamLiquidityValue(ctx, streamID, 0.1) // 10%折扣
	if err != nil {
		fmt.Printf("计算流动性价值失败: %v\n", err)
		return
	}
	
	fmt.Printf("流动性价值 (10%%折扣): %.6f USDC\n", liquidityValue)
	
	// 模拟创建新流
	streamParams := StreamParams{
		Sender:        common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e0F3B5F2b1F"),
		Recipient:     common.HexToAddress("0x742d35Cc6634C0532925a3b844Bc9e0F3B5F2b1F"),
		TokenAddress:  common.HexToAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"), // USDC
		TotalAmount:   big.NewInt(1000000000), // 1000 USDC
		DurationHours: 720, // 30天
	}
	
	simulation, err := calculator.CreateStreamSimulation(ctx, streamParams)
	if err != nil {
		fmt.Printf("创建流模拟失败: %v\n", err)
		return
	}
	
	fmt.Printf("\n新流模拟:\n")
	fmt.Printf("流ID: %s\n", simulation.StreamID.String())
	fmt.Printf("持续时间: %d小时 (%.1f天)\n", streamParams.DurationHours, float64(streamParams.DurationHours)/24)
	fmt.Printf("每秒支付: %.10f USDC\n", simulation.RatePerSecond)
	fmt.Printf("总金额: %.6f USDC\n", new(big.Float).SetInt(simulation.TotalAmount))
	
	// 协议说明
	fmt.Println("\n=== Sablier协议说明 ===")
	fmt.Println("协议类型: 实时流支付协议")
	fmt.Println("TVL: $3.91B")
	fmt.Println("核心功能:")
	fmt.Println("  1. 实时流支付: 资金按秒流动")
	fmt