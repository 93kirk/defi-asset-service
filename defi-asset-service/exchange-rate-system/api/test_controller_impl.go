package api

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// 继续TestController的实现

func (c *TestController) fetchPoolInfo(ctx context.Context, chain, address string) (map[string]interface{}, error) {
	// 连接到链
	client, err := ethclient.DialContext(ctx, c.rpcEndpoints[chain])
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", chain, err)
	}
	defer client.Close()
	
	// 检查合约地址
	contractAddr := common.HexToAddress(address)
	code, err := client.CodeAt(ctx, contractAddr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract code: %v", err)
	}
	
	if len(code) == 0 {
		return nil, fmt.Errorf("no contract code at address %s", address)
	}
	
	// 获取基础信息
	balance, err := client.BalanceAt(ctx, contractAddr, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %v", err)
	}
	
	// 尝试识别合约类型
	contractType := c.identifyContractType(ctx, client, contractAddr)
	
	return map[string]interface{}{
		"chain":         chain,
		"address":       address,
		"contract_type": contractType,
		"balance_wei":   balance.String(),
		"balance_eth":   weiToEth(balance).String(),
		"has_code":      len(code) > 0,
		"code_size":     len(code),
		"timestamp":     time.Now().Unix(),
	}, nil
}

func (c *TestController) fetchStakingInfo(ctx context.Context, chain, protocol string) (map[string]interface{}, error) {
	// 协议特定的合约地址
	contracts := map[string]map[string]string{
		"lido": {
			"eth": "0xae7ab96520DE3A18E5e111B5EaAb095312D7fE84", // stETH
		},
		"rocketpool": {
			"eth": "0xae78736Cd615f374D3085123A210448E74Fc6393", // rETH
		},
		"etherfi": {
			"eth": "0x4bc3263Eb5bb2Ef7Ad9aB6FB68be80E43b43801F", // eETH
		},
	}
	
	protocolContracts, ok := contracts[strings.ToLower(protocol)]
	if !ok {
		return nil, fmt.Errorf("protocol %s not supported", protocol)
	}
	
	contractAddr, ok := protocolContracts[chain]
	if !ok {
		return nil, fmt.Errorf("chain %s not supported for protocol %s", chain, protocol)
	}
	
	// 连接到链
	client, err := ethclient.DialContext(ctx, c.rpcEndpoints[chain])
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", chain, err)
	}
	defer client.Close()
	
	// 获取质押信息
	info := map[string]interface{}{
		"chain":     chain,
		"protocol":  protocol,
		"contract":  contractAddr,
		"timestamp": time.Now().Unix(),
	}
	
	// 根据协议类型获取特定信息
	switch strings.ToLower(protocol) {
	case "lido":
		lidoInfo, err := c.getLidoInfo(ctx, client, common.HexToAddress(contractAddr))
		if err == nil {
			info["details"] = lidoInfo
		}
	case "rocketpool":
		rocketInfo, err := c.getRocketPoolInfo(ctx, client, common.HexToAddress(contractAddr))
		if err == nil {
			info["details"] = rocketInfo
		}
	case "etherfi":
		etherfiInfo, err := c.getEtherFiInfo(ctx, client, common.HexToAddress(contractAddr))
		if err == nil {
			info["details"] = etherfiInfo
		}
	}
	
	return info, nil
}

func (c *TestController) fetchLendingInfo(ctx context.Context, chain, protocol, token string) (map[string]interface{}, error) {
	// 借贷协议合约地址
	contracts := map[string]map[string]string{
		"aave_v3": {
			"eth": "0x87870Bca3F3fD6335C3F4ce8392D69350B4fA4E2", // Pool
		},
		"compound_v3": {
			"eth": "0xc3d688B66703497DAA19211EEdff47f25384cdc3", // USDC市场
		},
	}
	
	protocolContracts, ok := contracts[strings.ToLower(protocol)]
	if !ok {
		return nil, fmt.Errorf("protocol %s not supported", protocol)
	}
	
	contractAddr, ok := protocolContracts[chain]
	if !ok {
		return nil, fmt.Errorf("chain %s not supported for protocol %s", chain, protocol)
	}
	
	// 连接到链
	client, err := ethclient.DialContext(ctx, c.rpcEndpoints[chain])
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", chain, err)
	}
	defer client.Close()
	
	info := map[string]interface{}{
		"chain":     chain,
		"protocol":  protocol,
		"token":     token,
		"contract":  contractAddr,
		"timestamp": time.Now().Unix(),
	}
	
	// 获取借贷市场信息
	switch strings.ToLower(protocol) {
	case "aave_v3":
		aaveInfo, err := c.getAaveV3Info(ctx, client, common.HexToAddress(contractAddr), token)
		if err == nil {
			info["details"] = aaveInfo
		}
	case "compound_v3":
		compoundInfo, err := c.getCompoundV3Info(ctx, client, common.HexToAddress(contractAddr), token)
		if err == nil {
			info["details"] = compoundInfo
		}
	}
	
	return info, nil
}

func (c *TestController) fetchAMMInfo(ctx context.Context, chain, pool string) (map[string]interface{}, error) {
	// AMM池地址
	pools := map[string]map[string]string{
		"uniswap_v3_usdc_eth": {
			"eth": "0x8ad599c3A0ff1De082011EFDDc58f1908eb6e6D8", // USDC/ETH 0.05%
		},
		"curve_3pool": {
			"eth": "0xbEbc44782C7dB0a1A60Cb6fe97d0b483032FF1C7",
		},
		"balancer_weth_dai": {
			"eth": "0x0b09deA16768f0799065C475bE02919503cB2a35",
		},
	}
	
	poolContracts, ok := pools[strings.ToLower(pool)]
	if !ok {
		return nil, fmt.Errorf("pool %s not supported", pool)
	}
	
	contractAddr, ok := poolContracts[chain]
	if !ok {
		return nil, fmt.Errorf("chain %s not supported for pool %s", chain, pool)
	}
	
	// 连接到链
	client, err := ethclient.DialContext(ctx, c.rpcEndpoints[chain])
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %v", chain, err)
	}
	defer client.Close()
	
	info := map[string]interface{}{
		"chain":     chain,
		"pool":      pool,
		"contract":  contractAddr,
		"timestamp": time.Now().Unix(),
	}
	
	// 获取池子信息
	switch {
	case strings.Contains(pool, "uniswap"):
		uniswapInfo, err := c.getUniswapV3Info(ctx, client, common.HexToAddress(contractAddr))
		if err == nil {
			info["details"] = uniswapInfo
		}
	case strings.Contains(pool, "curve"):
		curveInfo, err := c.getCurveInfo(ctx, client, common.HexToAddress(contractAddr))
		if err == nil {
			info["details"] = curveInfo
		}
	case strings.Contains(pool, "balancer"):
		balancerInfo, err := c.getBalancerInfo(ctx, client, common.HexToAddress(contractAddr))
		if err == nil {
			info["details"] = balancerInfo
		}
	}
	
	return info, nil
}

func (c *TestController) calculateWithRealSources(ctx context.Context, protocolID, underlyingToken string, amount float64, preferredSources []string) (float64, []map[string]interface{}, error) {
	var rates []float64
	var sources []map[string]interface{}
	
	// 1. 尝试链上合约调用
	if containsSource(preferredSources, "contract") || len(preferredSources) == 0 {
		rate, sourceInfo, err := c.getRateFromContract(ctx, protocolID, underlyingToken)
		if err == nil && rate > 0 {
			rates = append(rates, rate)
			sources = append(sources, map[string]interface{}{
				"type":   "contract",
				"rate":   rate,
				"method": "on-chain_call",
			})
		}
	}
	
	// 2. 尝试DeBank API
	if containsSource(preferredSources, "debank") || len(preferredSources) == 0 {
		rate, sourceInfo, err := c.getRateFromDebank(ctx, protocolID, underlyingToken)
		if err == nil && rate > 0 {
			rates = append(rates, rate)
			sources = append(sources, map[string]interface{}{
				"type":   "api",
				"rate":   rate,
				"source": "debank",
				"info":   sourceInfo,
			})
		}
	}
	
	// 3. 尝试Chainlink预言机
	if containsSource(preferredSources, "chainlink") || len(preferredSources) == 0 {
		rate, sourceInfo, err := c.getRateFromChainlink(ctx, protocolID, underlyingToken)
		if err == nil && rate > 0 {
			rates = append(rates, rate)
			sources = append(sources, map[string]interface{}{
				"type":   "oracle",
				"rate":   rate,
				"source": "chainlink",
				"info":   sourceInfo,
			})
		}
	}
	
	// 4. 尝试The Graph
	if containsSource(preferredSources, "thegraph") || len(preferredSources) == 0 {
		rate, sourceInfo, err := c.getRateFromTheGraph(ctx, protocolID, underlyingToken)
		if err == nil && rate > 0 {
			rates = append(rates, rate)
			sources = append(sources, map[string]interface{}{
				"type":   "subgraph",
				"rate":   rate,
				"source": "thegraph",
				"info":   sourceInfo,
			})
		}
	}
	
	if len(rates) == 0 {
		return 0, nil, fmt.Errorf("no real data sources available for %s/%s", protocolID, underlyingToken)
	}
	
	// 计算平均汇率
	var sum float64
	for _, rate := range rates {
		sum += rate
	}
	averageRate := sum / float64(len(rates))
	
	return averageRate, sources, nil
}

// 辅助方法
func (c *TestController) identifyContractType(ctx context.Context, client *ethclient.Client, addr common.Address) string {
	// 通过检查函数选择器或已知模式来识别合约类型
	// 这里简化实现
	
	// 检查是否是ERC20
	if c.isERC20(ctx, client, addr) {
		return "ERC20"
	}
	
	// 检查是否是质押合约
	if c.isStakingContract(ctx, client, addr) {
		return "Staking"
	}
	
	// 检查是否是借贷合约
	if c.isLendingContract(ctx, client, addr) {
		return "Lending"
	}
	
	// 检查是否是AMM池
	if c.isAMMPool(ctx, client, addr) {
		return "AMM"
	}
	
	return "Unknown"
}

func (c *TestController) isERC20(ctx context.Context, client *ethclient.Client, addr common.Address) bool {
	// 检查标准ERC20函数
	funcSigs := []string{
		"totalSupply()",           // 0x18160ddd
		"balanceOf(address)",      // 0x70a08231
		"transfer(address,uint256)", // 0xa9059cbb
		"allowance(address,address)", // 0xdd62ed3e
		"approve(address,uint256)",   // 0x095ea7b3
		"transferFrom(address,address,uint256)", // 0x23b872dd
	}
	
	// 简化：总是返回true（实际应该检查合约字节码）
	return true
}

func (c *TestController) isStakingContract(ctx context.Context, client *ethclient.Client, addr common.Address) bool {
	// 检查质押相关函数
	funcSigs := []string{
		"stake(uint256)",          // 质押
		"unstake(uint256)",        // 取消质押
		"getReward()",             // 获取奖励
		"totalStaked()",           // 总质押量
	}
	
	// 简化实现
	return false
}

func (c *TestController) isLendingContract(ctx context.Context, client *ethclient.Client, addr common.Address) bool {
	// 检查借贷相关函数
	funcSigs := []string{
		"deposit(uint256)",        // 存款
		"withdraw(uint256)",       // 取款
		"borrow(uint256)",         // 借款
		"repay(uint256)",          // 还款
	}
	
	// 简化实现
	return false
}

func (c *TestController) isAMMPool(ctx context.Context, client *ethclient.Client, addr common.Address) bool {
	// 检查AMM相关函数
	funcSigs := []string{
		"swap(uint256,uint256)",   // 交换
		"addLiquidity(uint256,uint256)", // 添加流动性
		"removeLiquidity(uint256)", // 移除流动性
		"getReserves()",           // 获取储备
	}
	
	// 简化实现
	return false
}

// 协议特定的信息获取方法
func (c *TestController) getLidoInfo(ctx context.Context, client *ethclient.Client, addr common.Address) (map[string]interface{}, error) {
	// Lido stETH合约信息
	// 实际应该调用合约函数
	
	return map[string]interface{}{
		"protocol":      "Lido",
		"token":         "stETH",
		"underlying":    "ETH",
		"apy":           "3.8%",
		"total_staked":  "9,800,000 ETH",
		"exchange_rate": "1 ETH = 1.02 stETH",
		"note":          "实际数据需要调用合约函数",
	}, nil
}

func (c *TestController) getRocketPoolInfo(ctx context.Context, client *ethclient.Client, addr common.Address) (map[string]interface{}, error) {
	return map[string]interface{}{
		"protocol":      "Rocket Pool",
		"token":         "rETH",
		"underlying":    "ETH",
		"apy":           "3.5%",
		"total_staked":  "1,200,000 ETH",
		"exchange_rate": "1 ETH = 1.019 rETH",
		"note":          "实际数据需要调用合约函数",
	}, nil
}

func (c *TestController) getEtherFiInfo(ctx context.Context, client *ethclient.Client, addr common.Address) (map[string]interface{}, error) {
	return map[string]interface{}{
		"protocol":      "ether.fi",
		"token":         "eETH",
		"underlying":    "ETH",
		"apy":           "4.2%",
		"total_staked":  "500,000 ETH",
		"exchange_rate": "1 ETH = 1.021 eETH",
		"features":      []string{"Liquid Staking", "Restaking", "EigenLayer"},
		"note":          "实际数据需要调用合约函数",
	}, nil
}

func (c *TestController) getAaveV3Info(ctx context.Context, client *ethclient.Client, addr common.Address, token string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"protocol":       "Aave V3",
		"market":         token,
		"supply_apy":     "2.1%",
		"borrow_apy":     "3.5%",
		"utilization":    "65%",
		"total_supply":   "$1.2B",
		"total_borrowed": "$780M",
		"note":           "实际数据