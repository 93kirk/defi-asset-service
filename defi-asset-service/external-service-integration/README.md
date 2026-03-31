# DeFi资产展示服务 - 外部服务集成

## 项目概述

本项目实现了DeFi资产展示服务的外部服务集成，对标DeBank功能。包含两个外部服务的客户端实现：

1. **服务A客户端**：提供有balance概念的协议资产实时查询
2. **服务B客户端**：提供无balance概念的协议仓位数据查询，支持缓存和持久化存储

## 架构设计

基于已完成的架构设计文档，本实现包含以下核心组件：

### 1. HTTP客户端封装 (`client/http_client.go`)
- 可配置的连接池管理
- 超时控制
- TLS配置支持
- 统一的请求/响应处理

### 2. 重试机制 (`retry/retry.go`)
- 指数退避算法
- 可配置的重试策略
- 支持抖动（Jitter）
- 可重试与不可重试错误区分

### 3. 限流器 (`rate_limit/rate_limiter.go`)
- 令牌桶算法实现
- 滑动窗口限流
- 多维度限流支持
- HTTP中间件集成

### 4. 熔断器 (`circuit_breaker/circuit_breaker.go`)
- 三种状态：闭合、打开、半开
- 可配置的失败阈值
- 自动恢复机制
- 统计信息收集

### 5. 服务A客户端 (`client/service_a_client.go`)
- 实时资产查询
- 批量查询优化
- 协议过滤支持
- 健康检查

### 6. 服务B客户端 (`client/service_b_client.go`)
- 带缓存的仓位查询
- MySQL持久化存储
- Redis缓存支持
- 队列更新处理

## 快速开始

### 环境要求
- Go 1.21+
- MySQL 8.0+
- Redis 7.0+

### 安装依赖
```bash
cd external-service-integration
go mod download
```

### 配置说明

复制配置文件模板并修改：
```bash
cp config/config_template.yaml config/config.yaml
```

主要配置项：
```yaml
service_a:
  base_url: "https://api.service-a.com/v1"
  api_key: "${SERVICE_A_API_KEY}"
  timeout: "10s"

service_b:
  base_url: "https://api.service-b.com/v1"
  api_key: "${SERVICE_B_API_KEY}"
  cache_ttl: "10m"

database:
  mysql:
    host: "localhost"
    database: "defi_asset_service"
  redis:
    host: "localhost"
```

### 使用示例

#### 1. 服务A客户端使用
```go
package main

import (
    "context"
    "defi-asset-service/external-service-integration/client"
)

func main() {
    ctx := context.Background()
    
    // 创建服务A客户端
    config := client.DefaultServiceAConfig()
    config.BaseURL = "https://api.service-a.com/v1"
    config.APIKey = "your-api-key"
    
    serviceA, err := client.NewServiceAClient(config)
    if err != nil {
        panic(err)
    }
    defer serviceA.Close()
    
    // 查询用户资产
    assets, err := serviceA.GetUserAssets(ctx, "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae", 1)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("总资产: %s USD\n", assets.TotalValueUSD)
}
```

#### 2. 服务B客户端使用
```go
package main

import (
    "context"
    "defi-asset-service/external-service-integration/client"
    "github.com/go-redis/redis/v8"
    "gorm.io/driver/mysql"
    "gorm.io/gorm"
)

func main() {
    ctx := context.Background()
    
    // 初始化数据库和Redis
    dsn := "user:password@tcp(localhost:3306)/defi_asset_service?charset=utf8mb4"
    db, _ := gorm.Open(mysql.Open(dsn), &gorm.Config{})
    
    redisClient := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    
    // 创建服务B客户端
    config := client.DefaultServiceBConfig()
    config.BaseURL = "https://api.service-b.com/v1"
    config.APIKey = "your-api-key"
    
    serviceB, err := client.NewServiceBClient(config, db, redisClient)
    if err != nil {
        panic(err)
    }
    defer serviceB.Close()
    
    // 查询用户仓位（带缓存）
    positions, err := serviceB.GetUserPositions(ctx, "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae", 1, false)
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("总仓位价值: %s USD\n", positions.TotalValueUSD)
    fmt.Printf("是否来自缓存: %v\n", positions.Cached)
}
```

## 核心特性

### 1. 高可用设计
- **熔断保护**：防止级联故障
- **自动重试**：网络波动时自动恢复
- **服务降级**：外部服务不可用时返回降级数据
- **健康检查**：定期检查服务状态

### 2. 性能优化
- **连接池复用**：减少连接建立开销
- **多级缓存**：内存 + Redis缓存
- **批量查询**：合并多个用户请求
- **异步处理**：耗时操作异步执行

### 3. 数据一致性
- **缓存失效策略**：主动失效 + TTL过期
- **数据库事务**：保证数据原子性
- **最终一致性**：通过队列保证数据同步
- **版本控制**：缓存数据带版本号

### 4. 监控告警
- **统计信息收集**：请求量、成功率、响应时间
- **熔断器状态监控**：实时状态和失败率
- **限流器统计**：允许/拒绝请求计数
- **错误日志记录**：详细错误信息和堆栈

## 数据流设计

### 服务A查询流程（实时）
```
客户端请求 → API网关 → 服务A客户端 → 外部服务A → 返回数据 → 客户端
```

### 服务B查询流程（带缓存）
```
客户端请求 → 检查Redis缓存 → 命中则返回
            ↓ 未命中
调用外部服务B → 存储到MySQL → 存储到Redis → 返回数据
```

### 实时更新流程
```
外部服务B推送 → Redis Streams队列 → Worker处理 → 更新MySQL → 更新Redis缓存
```

## 错误处理策略

### 1. 重试策略
- **最大重试次数**：3次（可配置）
- **退避算法**：指数退避 + 抖动
- **可重试错误**：网络超时、5xx错误等
- **不可重试错误**：4xx错误、业务逻辑错误等

### 2. 熔断策略
- **失败阈值**：5次失败（可配置）
- **打开状态超时**：30秒
- **半开状态测试**：允许少量请求通过
- **恢复条件**：连续成功达到阈值

### 3. 降级策略
- **缓存降级**：返回缓存数据
- **数据库降级**：返回数据库历史数据
- **默认值降级**：返回安全默认值
- **部分功能降级**：关闭非核心功能

## 部署建议

### 开发环境
```yaml
# 使用Docker Compose
version: '3.8'
services:
  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: root
      MYSQL_DATABASE: defi_asset_service
    ports:
      - "3306:3306"
  
  redis:
    image: redis:7.0
    ports:
      - "6379:6379"
  
  app:
    build: .
    environment:
      DB_HOST: mysql
      REDIS_HOST: redis
    depends_on:
      - mysql
      - redis
```

### 生产环境
- **容器化部署**：Docker + Kubernetes
- **多可用区部署**：保证高可用
- **自动扩缩容**：根据负载调整实例数
- **监控告警**：Prometheus + Grafana + Alertmanager

## API文档

### 服务A客户端API

#### GetUserAssets
获取用户实时资产
```go
func (c *ServiceAClient) GetUserAssets(ctx context.Context, address string, chainID int) (*UserAssetsResponse, error)
```

#### GetUserAssetsByProtocol
获取用户在特定协议的资产
```go
func (c *ServiceAClient) GetUserAssetsByProtocol(ctx context.Context, address string, chainID int, protocolID string) (*ProtocolAssetsResponse, error)
```

#### BatchGetUserAssets
批量获取用户资产
```go
func (c *ServiceAClient) BatchGetUserAssets(ctx context.Context, addresses []string, chainID int) (map[string]*UserAssetsResponse, error)
```

### 服务B客户端API

#### GetUserPositions
获取用户仓位数据（带缓存）
```go
func (c *ServiceBClient) GetUserPositions(ctx context.Context, address string, chainID int, forceRefresh bool) (*UserPositionsResponse, error)
```

#### GetUserPositionsByProtocol
获取用户在特定协议的仓位
```go
func (c *ServiceBClient) GetUserPositionsByProtocol(ctx context.Context, address string, chainID int, protocolID string) (*ProtocolPositionsResponse, error)
```

#### BatchGetUserPositions
批量获取用户仓位
```go
func (c *ServiceBClient) BatchGetUserPositions(ctx context.Context, addresses []string, chainID int) (map[string]*UserPositionsResponse, error)
```

## 测试

### 单元测试
```bash
go test ./client/... -v
go test ./retry/... -v
go test ./rate_limit/... -v
go test ./circuit_breaker/... -v
```

### 集成测试
```bash
# 需要运行MySQL和Redis
docker-compose up -d mysql redis
go test ./integration/... -v
```

### 性能测试
```bash
go test ./client/... -bench=. -benchmem
```

## 监控指标

### Prometheus指标
- `defi_external_requests_total` - 总请求数
- `defi_external_errors_total` - 错误数
- `defi_external_response_time_seconds` - 响应时间
- `defi_circuit_breaker_state` - 熔断器状态
- `defi_rate_limit_rejected_total` - 限流拒绝数

### Grafana仪表板
1. **服务健康仪表板**：显示服务状态和错误率
2. **性能仪表板**：显示响应时间和吞吐量
3. **熔断器仪表板**：显示熔断器状态和失败率
4. **缓存仪表板**：显示缓存命中率和效果

## 故障排除

### 常见问题

#### 1. 连接超时
- 检查网络连接
- 调整超时配置
- 检查防火墙规则

#### 2. 限流触发
- 检查请求频率
- 调整限流配置
- 实现请求合并

#### 3. 熔断器打开
- 检查外部服务状态
- 查看错误日志
- 等待自动恢复或手动重置

#### 4. 缓存不一致
- 检查缓存TTL配置
- 验证缓存失效逻辑
- 检查数据库同步

### 日志级别
- **DEBUG**：详细调试信息
- **INFO**：正常操作信息
- **WARN**：警告信息
- **ERROR**：错误信息

## 贡献指南

1. Fork项目
2. 创建特性分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建Pull Request

## 许可证

本项目采用MIT许可证。详见 [LICENSE](LICENSE) 文件。

## 联系方式

- 项目维护者：DeFi资产展示服务团队
- 问题反馈：[GitHub Issues](https://github.com/your-org/defi-asset-service/issues)
- 文档更新：请提交Pull Request

---

**版本**: v1.0.0  
**最后更新**: 2026-03-30  
**状态**: 生产就绪