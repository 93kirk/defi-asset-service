# DeFi Asset Service - 测试和部署系统

基于架构设计文档实现的DeFi资产展示服务的测试和部署系统。

## 项目结构

```
defi-asset-service/test-deploy-system/
├── Dockerfile                    # API网关Docker镜像配置
├── Dockerfile.worker            # 队列Worker Docker镜像配置
├── docker-compose.yml           # 开发环境Docker Compose配置
├── docker-compose.prod.yml      # 生产环境Docker Compose配置
├── go.mod                       # Go模块定义
├── README.md                    # 项目说明文档
├── scripts/
│   └── deploy.sh               # 部署脚本
├── config/
│   ├── cicd/
│   │   └── github-actions.yml  # GitHub Actions CI/CD配置
│   ├── monitoring/
│   │   ├── prometheus.yml      # Prometheus监控配置
│   │   ├── alert-rules.yml     # 告警规则配置
│   │   └── grafana-dashboards/ # Grafana仪表板配置
│   └── nginx/
│       └── nginx.conf          # Nginx负载均衡配置
├── tests/
│   ├── unit/                   # 单元测试
│   │   ├── service_a_test.go   # 服务A单元测试
│   │   ├── service_b_test.go   # 服务B单元测试
│   │   └── api_gateway_test.go # API网关单元测试
│   ├── integration/            # 集成测试
│   │   └── service_integration_test.go
│   ├── api/                    # API测试
│   │   └── api_test_suite.go
│   └── performance/            # 性能测试
│       └── load-test.js
├── k8s/
│   ├── staging/               # 预发布环境K8s配置
│   └── production/            # 生产环境K8s配置
├── database/
│   ├── migrations/            # 数据库迁移脚本
│   └── schema.sql            # 数据库表结构
└── mocks/
    ├── external-service-a/    # 外部服务A模拟
    └── external-service-b/    # 外部服务B模拟
```

## 核心功能

### 1. 测试系统
- **单元测试**: 覆盖核心业务逻辑（服务A、服务B、API网关）
- **集成测试**: 使用Testcontainers进行MySQL和Redis集成测试
- **API测试**: 完整的API接口测试套件
- **性能测试**: 使用k6进行负载测试

### 2. 部署系统
- **Docker化**: 多阶段构建，最小化镜像大小
- **容器编排**: Docker Compose支持多环境部署
- **Kubernetes**: 生产环境K8s部署配置
- **蓝绿部署**: 支持零停机时间部署

### 3. 监控告警
- **Prometheus**: 指标收集和存储
- **Grafana**: 数据可视化和仪表板
- **告警规则**: 多层次告警策略
- **健康检查**: 服务健康状态监控

### 4. CI/CD流水线
- **自动化测试**: 代码提交自动运行测试
- **安全扫描**: Trivy和Gosec安全扫描
- **镜像构建**: 自动构建和推送Docker镜像
- **多环境部署**: 开发、预发布、生产环境自动部署

## 快速开始

### 环境要求
- Docker 20.10+
- Docker Compose 2.0+
- Go 1.21+
- MySQL 8.0+
- Redis 7.0+

### 本地开发环境

1. **启动开发环境**
```bash
# 克隆项目
git clone <repository-url>
cd defi-asset-service/test-deploy-system

# 启动所有服务
docker-compose up -d

# 查看服务状态
docker-compose ps
```

2. **运行测试**
```bash
# 运行单元测试
go test ./tests/unit/... -v

# 运行集成测试（需要Docker）
go test ./tests/integration/... -v -tags=integration

# 运行API测试
go test ./tests/api/... -v
```

3. **访问服务**
- API网关: http://localhost:8080
- 健康检查: http://localhost:8080/health
- MySQL管理: http://localhost:8083 (phpMyAdmin)
- Redis管理: http://localhost:8084 (Redis Commander)
- Grafana: http://localhost:3000 (admin/admin)
- Prometheus: http://localhost:9090

### 部署到生产环境

1. **配置环境变量**
```bash
export ENVIRONMENT=production
export DB_PASSWORD=your_secure_password
export REDIS_PASSWORD=your_redis_password
export JWT_SECRET=your_jwt_secret
```

2. **执行部署脚本**
```bash
# 授予执行权限
chmod +x scripts/deploy.sh

# 执行完整部署
./scripts/deploy.sh --build --migrate --test --cleanup
```

3. **验证部署**
```bash
# 检查服务健康状态
curl http://your-domain.com/health

# 检查API接口
curl -H "X-API-Key: your-api-key" http://your-domain.com/v1/protocols
```

## 测试策略

### 单元测试
- **测试范围**: 业务逻辑、数据访问层、工具函数
- **Mock策略**: 使用testify/mock模拟外部依赖
- **覆盖率要求**: >80%代码覆盖率

### 集成测试
- **测试范围**: 服务间集成、数据库操作、缓存操作
- **测试环境**: 使用Testcontainers创建临时数据库和缓存
- **数据隔离**: 每个测试用例使用独立的数据集

### API测试
- **测试范围**: 所有API端点、认证授权、错误处理
- **测试数据**: 使用预定义的测试数据集
- **断言策略**: 验证响应结构、状态码、业务逻辑

### 性能测试
- **测试工具**: k6
- **测试场景**: 模拟真实用户行为
- **性能指标**: 响应时间、吞吐量、错误率

## 监控指标

### 业务指标
- 请求量、成功率、响应时间
- 用户资产查询统计
- 协议数据同步状态

### 系统指标
- CPU、内存、磁盘使用率
- 数据库连接数、查询性能
- Redis缓存命中率、内存使用

### 外部服务指标
- 外部服务可用性
- API调用成功率
- 响应时间分布

## 告警规则

### 关键告警（Critical）
- 服务不可用超过1分钟
- 数据库连接失败
- Redis服务不可用

### 警告告警（Warning）
- 错误率超过5%
- 响应时间超过2秒（P95）
- 缓存命中率低于80%
- 磁盘使用率超过85%

## CI/CD流程

### 开发分支（develop）
1. 代码提交触发CI
2. 运行lint和单元测试
3. 构建Docker镜像
4. 部署到开发环境
5. 运行集成测试

### 主分支（main）
1. 合并到main触发CD
2. 运行完整测试套件
3. 安全扫描
4. 部署到预发布环境
5. 性能测试
6. 蓝绿部署到生产环境

## 故障恢复

### 自动恢复
- 容器健康检查自动重启
- 数据库连接自动重试
- 外部服务熔断降级

### 手动恢复
```bash
# 查看服务日志
docker-compose logs -f api-gateway

# 重启服务
docker-compose restart api-gateway

# 回滚部署
./scripts/deploy.sh --rollback

# 数据库恢复
docker-compose exec mysql mysqldump -u root -p defi_asset_service > backup.sql
```

## 安全考虑

### 数据安全
- 敏感信息环境变量管理
- 数据库连接加密
- API请求认证授权

### 网络安全
- 容器网络隔离
- 防火墙规则配置
- DDoS防护

### 代码安全
- 依赖包安全扫描
- 代码漏洞扫描
- 权限最小化原则

## 扩展性设计

### 水平扩展
- 无状态API网关支持多实例
- 数据库读写分离
- Redis集群模式

### 垂直扩展
- 资源配额管理
- 连接池配置优化
- 缓存策略调整

## 维护指南

### 日常维护
- 监控告警响应
- 日志分析
- 性能优化

### 定期维护
- 安全补丁更新
- 数据库备份
- 系统升级

## 故障排查

### 常见问题
1. **服务启动失败**
   - 检查端口冲突
   - 验证环境变量配置
   - 查看容器日志

2. **数据库连接问题**
   - 检查网络连通性
   - 验证凭据配置
   - 查看数据库日志

3. **API响应缓慢**
   - 检查外部服务状态
   - 分析数据库查询性能
   - 监控系统资源使用

### 调试工具
```bash
# 进入容器
docker-compose exec api-gateway sh

# 查看实时日志
docker-compose logs -f --tail=100

# 性能分析
curl http://localhost:8080/debug/pprof/
```

## 贡献指南

1. Fork项目
2. 创建功能分支
3. 提交代码变更
4. 编写测试用例
5. 提交Pull Request

## 许可证

本项目采用MIT许可证。详见LICENSE文件。

## 联系方式

如有问题或建议，请通过以下方式联系：
- 项目Issue: [GitHub Issues]
- 邮件: [contact@example.com]
- 文档: [项目Wiki]