# DeFi资产展示服务

一个对标DeBank功能的DeFi资产展示服务，提供用户在DeFi协议中的资产持仓查询功能。

## 功能特性

### 核心功能
1. **服务A集成** - 实时查询有balance概念的协议资产
2. **服务B集成** - 查询无balance概念的协议仓位数据（带缓存）
3. **协议元数据同步** - 定时从DeBank抓取协议信息
4. **实时更新处理** - 通过Redis队列接收服务B的仓位更新

### 技术特性
- **高性能架构**：基于Go语言，支持高并发查询
- **智能缓存**：多级缓存策略，Redis缓存热点数据
- **队列处理**：Redis Streams实现消息队列，保证数据一致性
- **监控告警**：集成Prometheus和Grafana监控
- **容器化部署**：支持Docker和Kubernetes部署

## 系统架构

### 架构图
```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   客户端层       │    │   API网关层     │    │   业务服务层    │
│                 │    │                 │    │                 │
│ 前端/API调用方  │───▶│ 认证/限流/监控  │───▶│ 服务A/B调用模块 │
│                 │    │                 │    │ 协议元数据服务  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                                                         │
                                                         ▼
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  数据存储层     │    │   外部服务层    │    │   队列处理层    │
│                 │    │                 │    │                 │
│ MySQL + Redis   │◀───│ 服务A + 服务B   │◀───│ Redis Streams   │
│                 │    │ DeBank网页      │    │ 队列处理Worker  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

### 数据流
1. **实时balance查询**：客户端 → API网关 → 服务A调用模块 → 外部服务A → 返回数据
2. **协议仓位查询**：客户端 → API网关 → 服务B调用模块 → Redis缓存 → 未命中 → 外部服务B → 存储到MySQL和Redis → 返回数据
3. **协议元数据同步**：定时任务 → 协议元数据服务 → DeBank网页 → 解析清洗 → 更新MySQL → 清除缓存
4. **实时更新处理**：外部服务B推送 → Redis Streams队列 → 队列处理Worker → 更新MySQL和Redis

## 快速开始

### 环境要求
- Go 1.21+
- MySQL 8.0+
- Redis 7.0+
- Docker & Docker Compose（可选）

### 本地开发

1. **克隆项目**
```bash
git clone <repository-url>
cd defi-asset-service/go-service
```

2. **配置环境变量**
```bash
cp .env.example .env
# 编辑.env文件，配置数据库、Redis等连接信息
```

3. **安装依赖**
```bash
go mod download
```

4. **启动依赖服务**
```bash
docker-compose up -d mysql redis
```

5. **初始化数据库**
```bash
# 执行数据库迁移
mysql -h localhost -u defi_user -p defi_asset_service < migrations/init.sql
```

6. **运行应用**
```bash
go run cmd/api/main.go
```

### Docker部署
```bash
# 构建镜像
docker build -t defi-asset-service .

# 运行容器
docker run -d \
  --name defi-api \
  -p 8080:8080 \
  --env-file .env \
  defi-asset-service
```

### Docker Compose部署
```bash
# 启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f api

# 停止服务
docker-compose down
```

## API文档

### 基础信息
- **基础URL**: `http://localhost:8080/v1`
- **认证方式**: API密钥（Header: `X-API-Key`）
- **数据格式**: JSON

### 用户相关API

#### 获取用户资产总览
```
GET /users/{address}/summary
```

**参数**:
- `chain_id` (可选): 链ID，默认1（以太坊主网）
- `include_assets` (可选): 是否包含资产详情，默认false
- `include_positions` (可选): 是否包含仓位详情，默认false

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "user": {
      "address": "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
      "chain_id": 1,
      "total_value_usd": "125430.25",
      "total_asset_value_usd": "85430.15",
      "total_position_value_usd": "40000.10",
      "protocol_count": 8,
      "position_count": 12,
      "last_updated_at": "2026-03-30T10:30:00Z"
    },
    "assets": [...],
    "positions": [...]
  },
  "timestamp": 1678886400
}
```

#### 获取用户实时资产
```
GET /users/{address}/assets
```

**参数**:
- `chain_id` (可选): 链ID，默认1
- `protocol_id` (可选): 协议ID过滤
- `token_address` (可选): 代币地址过滤

#### 获取用户协议仓位
```
GET /users/{address}/positions
```

**参数**:
- `chain_id` (可选): 链ID，默认1
- `protocol_id` (可选): 协议ID过滤
- `position_type` (可选): 仓位类型过滤
- `refresh` (可选): 强制刷新缓存，默认false

#### 批量查询用户资产
```
POST /users/batch/assets
```

**请求体**:
```json
{
  "addresses": ["0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae"],
  "chain_id": 1,
  "include_assets": true,
  "include_positions": true
}
```

### 协议相关API

#### 获取协议列表
```
GET /protocols
```

**参数**:
- `category` (可选): 协议类别过滤
- `chain_id` (可选): 链ID过滤
- `is_active` (可选): 是否只返回活跃协议，默认true
- `page` (可选): 页码，默认1
- `page_size` (可选): 每页数量，默认20

#### 获取协议详情
```
GET /protocols/{protocol_id}
```

#### 获取协议代币列表
```
GET /protocols/{protocol_id}/tokens
```

### 管理相关API

#### 触发协议元数据同步
```
POST /admin/sync/protocols
```

**请求体**:
```json
{
  "force_full_sync": false,
  "protocol_ids": ["aave", "compound"]
}
```

#### 获取同步状态
```
GET /admin/sync/{sync_id}
```

## 配置说明

### 配置文件
配置文件位于 `configs/config.yaml`，支持环境变量覆盖。

### 主要配置项

#### 服务配置
```yaml
server:
  host: "0.0.0.0"
  port: 8080
  mode: "release"
  read_timeout: 30
  write_timeout: 30
```

#### 数据库配置
```yaml
database:
  mysql:
    host: "localhost"
    port: 3306
    user: "defi_user"
    password: "defi_password"
    dbname: "defi_asset_service"
```

#### Redis配置
```yaml
redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0
```

#### 缓存配置
```yaml
cache:
  position_ttl: 600      # 仓位缓存TTL（秒）
  protocol_ttl: 3600     # 协议缓存TTL（秒）
  price_ttl: 300         # 价格缓存TTL（秒）
  apy_ttl: 600           # APY缓存TTL（秒）
```

#### 外部服务配置
```yaml
external:
  service_a:
    base_url: "https://api.service-a.com/v1"
    api_key: ""
  service_b:
    base_url: "https://api.service-b.com/v1"
    api_key: ""
  debank:
    base_url: "https://openapi.debank.com/v1"
    api_key: ""
```

#### 队列配置
```yaml
queue:
  position_updates:
    stream_name: "defi:stream:position_updates"
    consumer_group: "position_workers"
    batch_size: 10
    max_retries: 3
```

## 监控和运维

### 健康检查
```
GET /health
```

### Prometheus指标
```
GET /metrics
```

### 日志
- 日志格式: JSON（生产环境）或Text（开发环境）
- 日志级别: debug, info, warn, error
- 输出: stdout 或文件

### 性能监控
- **请求指标**: 请求量、成功率、响应时间
- **缓存指标**: 命中率、内存使用
- **队列指标**: 待处理消息数、处理速度
- **数据库指标**: 连接数、慢查询

## 开发指南

### 项目结构
```
defi-asset-service/
├── cmd/
│   └── api/              # 主入口
├── internal/
│   ├── api/              # API控制器
│   ├── config/           # 配置加载
│   ├── model/            # 数据模型
│   ├── repository/       # 数据访问层
│   ├── service/          # 业务逻辑层
│   └── utils/            # 工具函数
├── pkg/
│   ├── middleware/       # 中间件
│   ├── client/           # HTTP客户端
│   └── queue/            # 队列处理
├── configs/              # 配置文件
├── migrations/           # 数据库迁移
├── scripts/              # 部署脚本
└── docs/                 # 文档
```

### 添加新API
1. 在 `internal/api/` 创建控制器
2. 在 `internal/service/` 实现业务逻辑
3. 在 `internal/repository/` 实现数据访问
4. 在 `cmd/api/main.go` 注册路由

### 添加新定时任务
1. 在 `internal/service/` 创建服务
2. 在 `cmd/api/main.go` 的 `initCronScheduler` 中添加任务

## 部署指南

### 生产环境部署

#### 1. 准备服务器
- 安装Docker和Docker Compose
- 配置防火墙（开放80、443、3306、6379端口）
- 配置SSL证书

#### 2. 部署应用
```bash
# 克隆代码
git clone <repository-url>
cd defi-asset-service

# 配置环境变量
cp .env.example .env.production
vim .env.production

# 启动服务
docker-compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

#### 3. 配置Nginx
```nginx
server {
    listen 80;
    server_name api.yourdomain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name api.yourdomain.com;
    
    ssl_certificate /etc/nginx/ssl/yourdomain.com.crt;
    ssl_certificate_key /etc/nginx/ssl/yourdomain.com.key;
    
    location / {
        proxy_pass http://api:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

#### 4. 监控配置
- 配置Prometheus数据源
- 导入Grafana仪表板
- 设置告警规则

## 故障排除

### 常见问题

#### 1. 数据库连接失败
- 检查MySQL服务状态
- 验证连接配置
- 检查防火墙设置

#### 2. Redis连接失败
- 检查Redis服务状态
- 验证密码配置
- 检查内存使用情况

#### 3. 外部服务调用失败
- 检查API密钥配置
- 验证网络连接
- 查看服务状态页

#### 4. 性能问题
- 检查缓存命中率
- 分析慢查询日志
- 监控系统资源使用

### 日志分析
```bash
# 查看应用日志
docker-compose logs api

# 查看错误日志
docker-compose logs api | grep ERROR

# 查看慢查询
docker-compose exec mysql mysql -u root -p -e "SHOW SLOW QUERIES;"
```

## 贡献指南

1. Fork项目
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建Pull Request

## 许可证

本项目采用MIT许可证。详见 [LICENSE](LICENSE) 文件。

## 联系方式

- 项目地址: [GitHub Repository](https://github.com/yourusername/defi-asset-service)
- 问题反馈: [GitHub Issues](https://github.com/yourusername/defi-asset-service/issues)
- 文档: [GitHub Wiki](https://github.com/yourusername/defi-asset-service/wiki)

---

**版本**: v1.0.0  
**最后更新**: 2026-03-30  
**作者**: DeFi资产展示服务团队