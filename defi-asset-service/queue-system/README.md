# DeFi资产展示服务 - Redis队列系统

基于Redis Streams的实时仓位更新队列系统，用于处理DeFi资产展示服务中服务B的实时仓位更新。

## 架构概述

本系统实现了基于Redis Streams的消息队列，包含以下核心组件：

1. **生产者 (Producer)** - 接收服务B的仓位更新，发布到Redis Streams
2. **消费者 (Consumer)** - 从Redis Streams消费消息，更新MySQL和Redis缓存
3. **监控系统 (Monitoring)** - 监控队列状态、性能和健康度
4. **死信队列 (DLQ)** - 处理失败消息的重试和归档

## 系统特性

- ✅ **高可靠性** - 基于Redis Streams，支持消息持久化和至少一次消费
- ✅ **可扩展性** - 多消费者并发处理，水平扩展能力
- ✅ **容错性** - 自动重试机制和死信队列处理
- ✅ **实时监控** - Prometheus指标收集和健康检查
- ✅ **易于部署** - 支持独立或组合部署模式

## 快速开始

### 环境要求

- Go 1.21+
- Redis 7.0+
- MySQL 8.0+
- 确保已创建数据库 `defi_asset_service`

### 安装依赖

```bash
cd queue-system
go mod download
```

### 配置

复制示例配置文件并修改：

```bash
cp config/config.yaml.example config/config.yaml
```

编辑 `config/config.yaml`：

```yaml
redis:
  address: "localhost:6379"
  password: ""
  db: 3

queue:
  stream_name: "defi:stream:position_updates"
  consumer_group: "position_workers"
  dlq_stream_name: "defi:stream:dlq:position_updates"
  max_retries: 3
  retry_delay: "30s"
  workers: 5

mysql:
  host: "localhost"
  port: 3306
  user: "root"
  password: ""
  database: "defi_asset_service"
```

### 运行模式

#### 1. 运行所有组件（开发模式）

```bash
go run main.go
```

#### 2. 仅运行生产者

```bash
go run main.go producer
```

#### 3. 仅运行消费者

```bash
go run main.go consumer
```

#### 4. 仅运行监控

```bash
go run main.go monitor
```

#### 5. 运行API服务器（包含所有功能）

```bash
go run main.go api
```

## API接口

### 生产者API (端口: 8081)

- `GET /health` - 健康检查
- `GET /stats` - 队列统计
- `POST /publish` - 发布单个消息
- `POST /publish/batch` - 批量发布消息
- `GET /metrics` - 监控指标

### 监控API (端口: 9090)

- `GET /health` - 健康检查
- `GET /metrics` - Prometheus指标
- `GET /report` - 监控报告
- `GET /alerts` - 当前告警

### 组合API (端口: 8080)

- `GET /` - 服务信息
- `GET /health` - 综合健康检查
- `GET /producer/*` - 生产者API
- `GET /monitor/*` - 监控API

## 消息格式

### 仓位更新消息

```json
{
  "event_id": "evt_1234567890",
  "event_type": "position_update",
  "user_address": "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
  "protocol_id": "aave",
  "chain_id": 1,
  "position_data": {
    "token_address": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
    "token_symbol": "USDC",
    "amount": "10000.0",
    "amount_usd": "10000.0",
    "apy": "2.15",
    "risk_level": 2,
    "metadata": {}
  },
  "timestamp": 1678886400,
  "source": "service_b",
  "version": "1.0"
}
```

## 部署指南

### Docker部署

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o queue-system main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/queue-system .
COPY config/config.yaml ./config/
EXPOSE 8080 8081 9090
CMD ["./queue-system", "api"]
```

### Kubernetes部署

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: defi-queue-system
spec:
  replicas: 3
  selector:
    matchLabels:
      app: defi-queue
  template:
    metadata:
      labels:
        app: defi-queue
    spec:
      containers:
      - name: queue-system
        image: defi-queue-system:latest
        ports:
        - containerPort: 8080
        - containerPort: 8081
        - containerPort: 9090
        env:
        - name: REDIS_ADDRESS
          value: "redis-master:6379"
        - name: MYSQL_HOST
          value: "mysql-service"
```

## 监控和告警

### 监控指标

- `defi_queue_length` - 队列长度
- `defi_queue_pending_count` - 待处理消息数
- `defi_messages_processed_total` - 已处理消息总数
- `defi_messages_failed_total` - 失败消息总数
- `defi_processing_latency_seconds` - 处理延迟
- `defi_consumer_lag_messages` - 消费者延迟

### 告警配置

默认告警阈值：

- 队列长度 > 1000 (警告), > 5000 (严重)
- 消费者延迟 > 100 (警告), > 500 (严重)
- 死信队列 > 10 (警告), > 100 (严重)
- Redis内存使用 > 80% (警告), > 95% (严重)

## 故障排除

### 常见问题

1. **Redis连接失败**
   - 检查Redis服务状态
   - 验证配置中的地址和端口
   - 检查防火墙设置

2. **MySQL连接失败**
   - 检查MySQL服务状态
   - 验证数据库用户权限
   - 确认数据库已创建

3. **队列积压**
   - 增加消费者数量
   - 检查消费者处理逻辑
   - 监控系统资源使用

4. **消息处理失败**
   - 检查死信队列中的错误信息
   - 验证消息格式
   - 检查数据库约束

### 日志查看

```bash
# 查看生产者日志
tail -f producer.log

# 查看消费者日志
tail -f consumer.log

# 查看监控日志
tail -f monitor.log
```

## 开发指南

### 项目结构

```
queue-system/
├── config/           # 配置文件
│   ├── config.go
│   ├── loader.go
│   └── config.yaml
├── producer/         # 生产者实现
│   ├── producer.go
│   └── http_handler.go
├── consumer/         # 消费者实现
│   ├── consumer.go
│   ├── processor.go
│   ├── retry_handler.go
│   └── dlq_handler.go
├── monitoring/       # 监控系统
│   ├── monitor.go
│   └── monitor_continued.go
├── models/           # 数据模型
│   └── message.go
├── main.go           # 主程序入口
├── go.mod           # Go模块定义
└── README.md        # 本文档
```

### 添加新功能

1. **添加新的消息类型**
   - 在 `models/message.go` 中定义新结构
   - 实现验证和序列化方法
   - 更新处理器逻辑

2. **扩展监控指标**
   - 在 `monitoring/monitor.go` 中添加新指标
   - 更新指标收集逻辑
   - 添加相应的告警规则

3. **集成新数据源**
   - 创建新的处理器
   - 实现数据转换逻辑
   - 更新配置和部署

## 性能优化

### 批量处理

```go
// 使用Pipeline批量发布消息
producer.PublishBatch(ctx, messages)
```

### 连接池优化

```yaml
redis:
  pool_size: 100  # 根据负载调整

mysql:
  max_connections: 25
```

### 缓存策略

- 热点数据预加载
- 多级缓存策略
- 缓存失效机制

## 安全考虑

1. **API认证** - 生产环境应添加API密钥认证
2. **数据加密** - 敏感数据应加密存储
3. **访问控制** - 限制数据库和Redis访问权限
4. **审计日志** - 记录所有关键操作

## 许可证

MIT License

## 支持

如有问题，请提交Issue或联系开发团队。