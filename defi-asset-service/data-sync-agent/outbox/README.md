# DeFi资产展示服务 - 数据同步系统

## 项目概述

这是一个对标DeBank功能的DeFi资产展示服务的数据同步系统。系统负责从DeBank网页抓取协议元数据，并定时同步到本地数据库，为前端提供实时、准确的DeFi协议信息。

## 核心功能

1. **DeBank网页抓取器** - 使用Colly/GoQuery抓取DeBank协议数据
2. **协议元数据解析器** - 解析和清洗抓取的数据
3. **定时同步任务** - 使用cron库实现定时同步
4. **增量同步机制** - 智能识别和同步变更数据
5. **错误处理和重试逻辑** - 完善的容错机制
6. **同步状态管理** - 实时监控同步状态和健康度

## 系统架构

### 组件架构

```
┌─────────────────────────────────────────────────────────────┐
│                     数据同步系统                             │
├──────────────┬──────────────┬──────────────┬──────────────┤
│  网页抓取器   │  数据解析器   │  同步服务     │  状态管理     │
│  (Colly)     │  (Parser)    │  (Service)   │  (State)     │
├──────────────┼──────────────┼──────────────┼──────────────┤
│  定时调度器   │  错误重试     │  缓存管理     │  监控告警     │
│  (Cron)      │  (Retry)     │  (Cache)     │  (Monitor)   │
└──────────────┴──────────────┴──────────────┴──────────────┘
                            │
                    ┌───────▼───────┐
                    │   数据存储     │
                    ├───────┬───────┤
                    │ MySQL │ Redis │
                    └───────┴───────┘
```

### 数据流

1. **定时触发** → 调度器启动同步任务
2. **网页抓取** → 从DeBank抓取协议列表和详情
3. **数据解析** → 清洗、验证、格式化数据
4. **数据同步** → 对比并更新数据库
5. **状态更新** → 记录同步结果和状态
6. **缓存更新** → 更新Redis缓存
7. **监控告警** → 发送监控指标和告警

## 技术栈

- **编程语言**: Go 1.21+
- **Web框架**: 标准库 + Gin/Echo (可选)
- **网页抓取**: Colly + GoQuery
- **定时任务**: cron/v3
- **数据库**: MySQL 8.0+, GORM
- **缓存**: Redis 7.0+
- **配置管理**: YAML
- **日志**: slog (结构化日志)
- **容器化**: Docker + Docker Compose
- **监控**: Prometheus + Grafana

## 快速开始

### 环境要求

- Go 1.21+
- MySQL 8.0+
- Redis 7.0+
- Docker & Docker Compose (可选)

### 使用Docker Compose运行

```bash
# 克隆项目
git clone <repository-url>
cd defi-asset-service/data-sync-agent

# 启动所有服务
docker-compose up -d

# 查看日志
docker-compose logs -f data-sync

# 停止服务
docker-compose down
```

### 手动安装和运行

```bash
# 1. 安装依赖
go mod download

# 2. 配置数据库
mysql -u root -p < init.sql

# 3. 编辑配置文件
cp config/config.example.yaml config/config.yaml
vim config/config.yaml

# 4. 构建应用
go build -o data-sync ./cmd/data-sync

# 5. 运行应用
./data-sync
```

## 配置说明

### 主要配置项

```yaml
# 应用配置
app:
  name: "defi-data-sync"
  environment: "development"  # development, staging, production

# 数据库配置
database:
  mysql:
    host: "localhost"
    port: 3306
    username: "defi_sync"
    password: "sync_password"
    database: "defi_asset_service"

# DeBank抓取配置
external:
  debank:
    base_url: "https://debank.com"
    timeout: 30s
    rate_limit: 10  # 每秒请求数限制

# 同步任务配置
sync:
  protocol_metadata:
    enabled: true
    schedule: "0 0 2 * * *"  # 每天凌晨2点
    batch_size: 50
    concurrency: 5
```

### 环境变量覆盖

支持通过环境变量覆盖配置：

```bash
export DB_HOST=mysql
export DB_PORT=3306
export DB_USER=defi_sync
export DB_PASSWORD=sync_password
export REDIS_HOST=redis
export REDIS_PORT=6379
```

## 同步机制

### 全量同步

- **触发条件**: 每天凌晨2点执行
- **流程**: 抓取所有协议 → 对比数据库 → 更新变更
- **特点**: 保证数据完整性，清理过时协议

### 增量同步

- **触发条件**: 每5分钟检查一次
- **流程**: 检查需要同步的协议 → 抓取协议详情 → 更新变更
- **特点**: 高效，减少网络请求，实时性高

### 错误重试

- **指数退避**: 1s, 2s, 4s, 8s, 最大30s
- **最大重试**: 3次
- **熔断机制**: 连续失败时暂停请求

## API接口

### 管理接口

```
GET  /health                    # 健康检查
GET  /metrics                   # Prometheus指标
GET  /api/v1/sync/status        # 同步状态
POST /api/v1/sync/trigger       # 手动触发同步
GET  /api/v1/sync/history       # 同步历史记录
```

### 数据接口

```
GET  /api/v1/protocols          # 协议列表
GET  /api/v1/protocols/{id}     # 协议详情
GET  /api/v1/tokens             # 代币列表
GET  /api/v1/tokens/{address}   # 代币详情
```

## 监控告警

### 监控指标

- **同步成功率**: `sync_success_rate`
- **同步延迟**: `sync_duration_seconds`
- **抓取错误率**: `scrape_error_rate`
- **数据库连接数**: `db_connections`
- **Redis命中率**: `redis_hit_rate`

### 告警规则

- 同步错误率 > 1% (5分钟窗口)
- 同步延迟 > 5分钟
- 队列积压 > 1000条
- 数据库连接数 > 80%

## 部署指南

### 生产环境部署

```bash
# 1. 创建生产配置文件
mkdir -p /etc/defi-data-sync
cp config/config.production.yaml /etc/defi-data-sync/config.yaml

# 2. 使用systemd管理服务
cp systemd/defi-data-sync.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable defi-data-sync
systemctl start defi-data-sync

# 3. 配置日志轮转
cp logrotate/defi-data-sync /etc/logrotate.d/
```

### Kubernetes部署

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: defi-data-sync
spec:
  replicas: 2
  selector:
    matchLabels:
      app: defi-data-sync
  template:
    metadata:
      labels:
        app: defi-data-sync
    spec:
      containers:
      - name: data-sync
        image: defi/data-sync:latest
        ports:
        - containerPort: 8080
        env:
        - name: ENVIRONMENT
          value: "production"
```

## 开发指南

### 项目结构

```
data-sync-agent/
├── cmd/data-sync/          # 主程序入口
├── internal/               # 内部包
│   ├── config/            # 配置管理
│   ├── scraper/           # 网页抓取器
│   ├── parser/            # 数据解析器
│   ├── service/           # 业务服务
│   ├── sync/              # 同步调度
│   ├── retry/             # 错误重试
│   ├── state/             # 状态管理
│   └── models/            # 数据模型
├── config/                 # 配置文件
├── scripts/               # 工具脚本
└── tests/                 # 测试文件
```

### 添加新的同步类型

1. 在`models`中定义数据结构
2. 在`scraper`中实现抓取逻辑
3. 在`parser`中实现解析逻辑
4. 在`service`中实现同步逻辑
5. 在`sync`中配置定时任务
6. 在`config`中添加配置项

### 测试

```bash
# 运行单元测试
go test ./internal/...

# 运行集成测试
go test -tags=integration ./tests/...

# 运行性能测试
go test -bench=. ./internal/...
```

## 故障排除

### 常见问题

1. **数据库连接失败**
   - 检查MySQL服务状态
   - 验证连接参数
   - 检查防火墙设置

2. **DeBank抓取失败**
   - 检查网络连接
   - 验证DeBank网站可访问
   - 调整抓取频率和超时设置

3. **同步性能问题**
   - 调整批处理大小
   - 增加并发数
   - 优化数据库索引

4. **内存泄漏**
   - 检查goroutine泄漏
   - 监控内存使用
   - 调整连接池大小

### 日志分析

```bash
# 查看错误日志
grep "ERROR" logs/data-sync.log

# 查看同步统计
grep "同步完成" logs/data-sync.log

# 查看性能指标
grep "duration" logs/data-sync.log | tail -20
```

## 性能优化

### 数据库优化

```sql
-- 添加索引
CREATE INDEX idx_protocol_last_synced ON protocols(last_synced_at);
CREATE INDEX idx_sync_status ON sync_records(status, started_at);

-- 分区表（大数据量）
ALTER TABLE user_positions PARTITION BY HASH(user_id) PARTITIONS 8;
```

### 缓存优化

- 使用多级缓存（内存 + Redis）
- 设置合理的TTL
- 实现缓存预热
- 使用缓存穿透保护

### 网络优化

- 使用HTTP连接池
- 启用HTTP/2
- 实现请求合并
- 使用CDN加速静态资源

## 安全考虑

### 数据安全

- 敏感数据加密存储
- 传输使用TLS 1.3
- 实现数据脱敏
- 定期安全审计

### 访问控制

- API密钥认证
- 基于角色的访问控制
- 请求频率限制
- IP白名单

### 网络安全

- 防火墙配置
- DDoS防护
- 漏洞扫描
- 安全更新

## 扩展计划

### 短期计划

1. 支持更多数据源（CoinGecko, DefiLlama等）
2. 实现实时数据推送（WebSocket）
3. 添加数据质量监控
4. 支持多语言协议信息

### 长期计划

1. 分布式抓取集群
2. 机器学习预测模型
3. 跨链协议支持
4. 移动端SDK

## 贡献指南

1. Fork项目
2. 创建功能分支
3. 提交代码变更
4. 编写测试用例
5. 更新文档
6. 创建Pull Request

## 许可证

MIT License

## 联系方式

- 项目主页: <repository-url>
- 问题反馈: <issues-url>
- 文档: <docs-url>