# DeFi协议汇率转换系统

## 概述
基于TVL前200的DeFi协议，实现统一的underlying token到receipt token的汇率转换系统。

## 支持的协议类型
1. **流动性质押协议** (Lido, Rocket Pool, BNB Chain Staking)
2. **借贷协议** (Aave, Compound, Morpho)
3. **DEX流动性池** (Uniswap, Curve, Balancer)
4. **收益聚合器** (Yearn, Convex, Aura)
5. **LSD收益协议** (ether.fi, Kelp, Swell)

## 系统架构

### 核心组件
```
exchange-rate-system/
├── internal/
│   ├── provider/          # 汇率提供者
│   ├── adapter/           # 协议适配器
│   ├── calculator/        # 汇率计算器
│   ├── cache/            # 缓存管理
│   └── models/           # 数据模型
├── api/                  # API接口
├── config/              # 配置文件
└── cmd/                 # 命令行入口
```

### 数据流
```
用户请求 → API网关 → 协议识别 → 适配器选择 → 汇率计算 → 结果返回
      ↓          ↓          ↓           ↓          ↓
   缓存检查 → 数据源查询 → 参数验证 → 计算执行 → 格式转换
```

## 汇率计算方法

### 1. 流动性质押协议
```go
receipt_amount = underlying_amount × (total_assets / total_supply)
```

### 2. 借贷协议
```go
cToken_amount = underlying_amount × exchange_rate_stored
```

### 3. AMM池
```go
LP_token = calculate_lp_amount(amounts, reserves)
```

### 4. 收益聚合器
```go
yToken_amount = underlying_amount × price_per_share
```

### 5. LSD收益协议
```go
lsdToken_amount = staked_amount × (1 + yield_rate)
```

## 集成到DeFi资产展示服务

### 1. 扩展协议服务
```go
// 在protocol_service.go中添加汇率计算功能
type ProtocolService struct {
    exchangeRateProvider ExchangeRateProvider
}

func (s *ProtocolService) GetProtocolWithRates(protocolID string) (ProtocolWithRates, error) {
    protocol, err := s.GetProtocol(protocolID)
    if err != nil {
        return ProtocolWithRates{}, err
    }
    
    // 获取汇率信息
    rates, err := s.exchangeRateProvider.GetRates(protocolID)
    if err != nil {
        return ProtocolWithRates{}, err
    }
    
    return ProtocolWithRates{
        Protocol: protocol,
        Rates:    rates,
    }, nil
}
```

### 2. 扩展用户资产查询
```go
// 在user_controller.go中添加汇率转换功能
func (c *UserController) GetUserAssetsWithRates(userID string) ([]AssetWithRate, error) {
    assets, err := c.GetUserAssets(userID)
    if err != nil {
        return nil, err
    }
    
    // 为每个资产计算汇率
    var assetsWithRates []AssetWithRate
    for _, asset := range assets {
        rate, err := c.exchangeRateCalculator.CalculateRate(
            asset.ProtocolID,
            asset.UnderlyingToken,
            asset.Amount,
        )
        if err != nil {
            // 记录错误但继续处理其他资产
            log.Printf("Failed to calculate rate for asset %s: %v", asset.ID, err)
            continue
        }
        
        assetsWithRates = append(assetsWithRates, AssetWithRate{
            Asset: asset,
            Rate:  rate,
            ReceiptAmount: asset.Amount * rate.ExchangeRate,
        })
    }
    
    return assetsWithRates, nil
}
```

### 3. 新增API端点
```
GET  /api/v1/exchange-rates/:protocol_id          # 获取协议汇率
POST /api/v1/exchange-rates/calculate            # 计算汇率
GET  /api/v1/exchange-rates/history/:protocol_id  # 获取历史汇率
GET  /api/v1/assets/:user_id/with-rates          # 获取用户资产带汇率
```

## 配置

### 环境变量
```bash
EXCHANGE_RATE_REDIS_URL=redis://localhost:6379
EXCHANGE_RATE_CACHE_TTL=300
EXCHANGE_RATE_UPDATE_INTERVAL=60
EXCHANGE_RATE_MAX_RETRIES=3
```

### 配置文件
```yaml
exchange_rate:
  providers:
    - name: "chainlink"
      enabled: true
      url: "https://api.chain.link"
    - name: "protocol_api"
      enabled: true
    - name: "custom_calculator"
      enabled: true
  
  cache:
    redis:
      url: "redis://localhost:6379"
      ttl: 300
    memory:
      size: 1000
      ttl: 60
  
  protocols:
    liquid_staking:
      update_interval: 10  # 每10秒更新
    lending:
      update_interval: 30
    amm:
      update_interval: 5
```

## 部署

### Docker部署
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o exchange-rate-service ./cmd/exchange-rate

FROM alpine:latest
COPY --from=builder /app/exchange-rate-service /app/
EXPOSE 8080
CMD ["/app/exchange-rate-service"]
```

### Kubernetes部署
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: exchange-rate-service
spec:
  replicas: 3
  selector:
    matchLabels:
      app: exchange-rate
  template:
    metadata:
      labels:
        app: exchange-rate
    spec:
      containers:
      - name: exchange-rate
        image: exchange-rate-service:latest
        ports:
        - containerPort: 8080
        env:
        - name: REDIS_URL
          value: "redis://redis-service:6379"
```

## 监控

### Prometheus指标
- `exchange_rate_requests_total`
- `exchange_rate_errors_total`
- `exchange_rate_calculation_duration_seconds`
- `exchange_rate_cache_hit_ratio`

### 健康检查
```
GET /health
GET /ready
GET /metrics
```

## 测试

### 单元测试
```bash
go test ./internal/calculator/... -v
```

### 集成测试
```bash
go test ./integration/... -v
```

### 性能测试
```bash
go test ./benchmark/... -bench=. -benchtime=30s
```

## 开发指南

### 添加新协议适配器
1. 在`internal/adapter/`目录下创建新文件
2. 实现`ProtocolAdapter`接口
3. 注册适配器到`adapter_registry.go`
4. 添加单元测试

### 添加新汇率提供者
1. 在`internal/provider/`目录下创建新文件
2. 实现`ExchangeRateProvider`接口
3. 注册提供者到`provider_registry.go`
4. 添加配置项

## 故障排除

### 常见问题
1. **汇率计算错误**: 检查协议适配器是否正确实现
2. **缓存失效**: 检查Redis连接和TTL设置
3. **API限流**: 调整请求频率和重试策略
4. **数据不一致**: 启用多数据源验证

### 日志级别
```yaml
logging:
  level: "info"
  format: "json"
```

## 性能优化

### 缓存策略
- 一级缓存: 内存缓存 (LRU, 60秒TTL)
- 二级缓存: Redis缓存 (300秒TTL)
- 三级缓存: 数据库持久化

### 并发处理
- 使用goroutine池处理批量请求
- 连接池管理数据库和Redis连接
- 限流和熔断机制

## 安全考虑

### 输入验证
- 验证协议ID格式
- 验证金额范围
- 防止SQL注入

### 访问控制
- API密钥认证
- 速率限制
- IP白名单

## 未来扩展

### 计划功能
1. 实时汇率推送 (WebSocket)
2. 汇率预测模型
3. 跨链汇率计算
4. 风险管理模块

### 协议支持扩展
1. 期权协议
2. 保险协议
3. 衍生品协议
4. NFT金融协议