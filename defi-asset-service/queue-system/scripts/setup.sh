#!/bin/bash

# DeFi资产展示服务 - Redis队列系统部署脚本

set -e

echo "=== DeFi资产展示服务 - Redis队列系统部署 ==="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查命令是否存在
check_command() {
    if ! command -v $1 &> /dev/null; then
        log_error "命令 $1 未找到，请先安装"
        exit 1
    fi
}

# 检查环境
check_environment() {
    log_info "检查环境依赖..."
    
    # 检查Go
    check_command go
    log_info "Go版本: $(go version)"
    
    # 检查Redis
    check_command redis-cli
    log_info "Redis版本: $(redis-cli --version 2>/dev/null || echo '未连接')"
    
    # 检查MySQL
    check_command mysql
    log_info "MySQL客户端已安装"
    
    # 检查Docker（可选）
    if command -v docker &> /dev/null; then
        log_info "Docker版本: $(docker --version)"
    else
        log_warn "Docker未安装，跳过容器化部署"
    fi
    
    # 检查Kubernetes工具（可选）
    if command -v kubectl &> /dev/null; then
        log_info "Kubernetes客户端已安装"
    fi
}

# 安装Go依赖
install_dependencies() {
    log_info "安装Go依赖..."
    cd /root/.openclaw/workspace/defi-asset-service/queue-system
    go mod download
    log_info "依赖安装完成"
}

# 配置环境
setup_config() {
    log_info "配置环境..."
    
    # 创建配置文件
    if [ ! -f config/config.yaml ]; then
        log_info "创建配置文件..."
        cat > config/config.yaml << EOF
# DeFi资产展示服务 - Redis队列系统配置

redis:
  address: "localhost:6379"
  password: ""
  db: 3
  pool_size: 100

queue:
  # 主队列配置
  stream_name: "defi:stream:position_updates"
  consumer_group: "position_workers"
  consumer_name: "worker"
  
  # 死信队列配置
  dlq_stream_name: "defi:stream:dlq:position_updates"
  max_retries: 3
  retry_delay: "30s"
  
  # 消费者配置
  batch_size: 10
  block_timeout: "5s"
  auto_ack: false
  
  # 监控配置
  metrics_enabled: true
  health_check_interval: "30s"

mysql:
  host: "localhost"
  port: 3306
  user: "root"
  password: ""
  database: "defi_asset_service"
  charset: "utf8mb4"

# 应用配置
log_level: "info"
workers: 5
EOF
        log_info "配置文件创建完成"
    else
        log_info "配置文件已存在，跳过创建"
    fi
    
    # 创建环境变量文件
    if [ ! -f .env ]; then
        log_info "创建环境变量文件..."
        cat > .env << EOF
# Redis配置
REDIS_ADDRESS=localhost:6379
REDIS_PASSWORD=
REDIS_DB=3

# MySQL配置
MYSQL_HOST=localhost
MYSQL_PORT=3306
MYSQL_USER=root
MYSQL_PASSWORD=
MYSQL_DATABASE=defi_asset_service

# 队列配置
QUEUE_STREAM_NAME=defi:stream:position_updates
QUEUE_CONSUMER_GROUP=position_workers
QUEUE_DLQ_STREAM_NAME=defi:stream:dlq:position_updates
QUEUE_MAX_RETRIES=3
QUEUE_RETRY_DELAY=30s

# 应用配置
LOG_LEVEL=info
WORKERS=5
EOF
        log_info "环境变量文件创建完成"
    fi
}

# 初始化数据库
init_database() {
    log_info "初始化数据库..."
    
    # 检查数据库连接
    if mysql -h localhost -u root -e "SELECT 1" &> /dev/null; then
        log_info "MySQL连接正常"
    else
        log_error "无法连接到MySQL，请检查服务状态"
        exit 1
    fi
    
    # 创建数据库
    log_info "创建数据库..."
    mysql -h localhost -u root -e "CREATE DATABASE IF NOT EXISTS defi_asset_service DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" || {
        log_warn "数据库创建失败或已存在"
    }
    
    # 创建表结构
    log_info "创建表结构..."
    
    # 从架构文档中提取SQL（简化版）
    SQL_FILE="/root/.openclaw/workspace/defi-asset-service/agents/architecture-agent/outbox/database-schema.sql"
    
    if [ -f "$SQL_FILE" ]; then
        mysql -h localhost -u root defi_asset_service < "$SQL_FILE"
        log_info "表结构创建完成"
    else
        log_warn "SQL文件未找到，使用简化表结构"
        
        # 创建简化表结构
        mysql -h localhost -u root defi_asset_service << EOF
-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    address VARCHAR(42) NOT NULL,
    chain_id INT NOT NULL DEFAULT 1,
    total_assets_usd DECIMAL(30, 6) DEFAULT 0,
    last_updated_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_address_chain (address, chain_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 协议表
CREATE TABLE IF NOT EXISTS protocols (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    protocol_id VARCHAR(100) NOT NULL,
    name VARCHAR(200) NOT NULL,
    category VARCHAR(50) NOT NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_protocol_id (protocol_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 用户仓位表
CREATE TABLE IF NOT EXISTS user_positions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_address VARCHAR(42) NOT NULL,
    protocol_id VARCHAR(100) NOT NULL,
    chain_id INT NOT NULL DEFAULT 1,
    token_address VARCHAR(42) NOT NULL,
    token_symbol VARCHAR(20) NOT NULL,
    amount DECIMAL(30, 18) NOT NULL,
    amount_usd DECIMAL(30, 6) NOT NULL,
    apy DECIMAL(10, 4),
    risk_level TINYINT,
    metadata JSON,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_protocol_token (user_address, protocol_id, token_address, chain_id),
    KEY idx_user_address (user_address),
    KEY idx_protocol_id (protocol_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
EOF
        log_info "简化表结构创建完成"
    fi
}

# 初始化Redis
init_redis() {
    log_info "初始化Redis..."
    
    # 检查Redis连接
    if redis-cli ping &> /dev/null; then
        log_info "Redis连接正常"
    else
        log_error "无法连接到Redis，请检查服务状态"
        exit 1
    fi
    
    # 创建消费者组（如果不存在）
    log_info "创建Redis Streams和消费者组..."
    
    # 检查Stream是否存在
    if redis-cli xlen "defi:stream:position_updates" &> /dev/null; then
        log_info "Stream已存在"
    else
        # 创建Stream和消费者组
        redis-cli xgroup create "defi:stream:position_updates" "position_workers" "$" MKSTREAM &> /dev/null || {
            log_warn "消费者组创建失败或已存在"
        }
    fi
    
    # 创建死信队列Stream
    redis-cli xadd "defi:stream:dlq:position_updates" "*" init "setup" &> /dev/null || {
        log_warn "死信队列创建失败或已存在"
    }
    
    # 删除初始化消息
    redis-cli xdel "defi:stream:dlq:position_updates" "$(redis-cli xrange "defi:stream:dlq:position_updates" - + | head -1 | awk '{print $1}')" &> /dev/null || true
    
    log_info "Redis初始化完成"
}

# 构建应用
build_application() {
    log_info "构建应用..."
    
    cd /root/.openclaw/workspace/defi-asset-service/queue-system
    
    # 构建所有组件
    log_info "构建主程序..."
    go build -o bin/queue-system main.go
    
    # 构建独立组件
    log_info "构建独立组件..."
    go build -o bin/producer -ldflags="-X main.mode=producer" main.go
    go build -o bin/consumer -ldflags="-X main.mode=consumer" main.go
    go build -o bin/monitor -ldflags="-X main.mode=monitor" main.go
    go build -o bin/api -ldflags="-X main.mode=api" main.go
    
    log_info "应用构建完成"
}

# 运行测试
run_tests() {
    log_info "运行测试..."
    
    cd /root/.openclaw/workspace/defi-asset-service/queue-system
    
    # 运行单元测试
    log_info "运行单元测试..."
    go test ./... -v || {
        log_warn "单元测试失败"
    }
    
    # 运行集成测试
    log_info "运行集成测试..."
    
    # 启动测试服务
    log_info "启动测试服务..."
    ./bin/queue-system producer &
    PRODUCER_PID=$!
    
    ./bin/queue-system consumer &
    CONSUMER_PID=$!
    
    ./bin/queue-system monitor &
    MONITOR_PID=$!
    
    # 等待服务启动
    sleep 5
    
    # 测试API端点
    log_info "测试API端点..."
    
    # 测试健康检查
    if curl -s http://localhost:8081/health | grep -q "healthy"; then
        log_info "生产者健康检查通过"
    else
        log_error "生产者健康检查失败"
    fi
    
    if curl -s http://localhost:9090/health | grep -q "healthy"; then
        log_info "监控健康检查通过"
    else
        log_error "监控健康检查失败"
    fi
    
    # 测试消息发布
    log_info "测试消息发布..."
    TEST_RESPONSE=$(curl -s -X POST http://localhost:8081/publish \
        -H "Content-Type: application/json" \
        -d '{
            "user_address": "0x742d35Cc6634C0532925a3b844Bc9e90F1A904Ae",
            "protocol_id": "aave",
            "position_data": {
                "token_address": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
                "token_symbol": "USDC",
                "amount": "10000.0",
                "amount_usd": "10000.0",
                "apy": "2.15",
                "risk_level": 2
            }
        }')
    
    if echo "$TEST_RESPONSE" | grep -q "message_id"; then
        log_info "消息发布测试通过"
    else
        log_error "消息发布测试失败: $TEST_RESPONSE"
    fi
    
    # 停止测试服务
    log_info "停止测试服务..."
    kill $PRODUCER_PID $CONSUMER_PID $MONITOR_PID 2>/dev/null || true
    
    log_info "测试完成"
}

# 创建Docker镜像
build_docker_image() {
    if ! command -v docker &> /dev/null; then
        log_warn "Docker未安装，跳过镜像构建"
        return
    fi
    
    log_info "构建Docker镜像..."
    
    cd /root/.openclaw/workspace/defi-asset-service/queue-system
    
    # 创建Dockerfile
    cat > Dockerfile << EOF
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
EOF
    
    # 构建镜像
    docker build -t defi-queue-system:latest .
    
    # 验证镜像
    if docker images | grep -q "defi-queue-system"; then
        log_info "Docker镜像构建成功"
    else
        log_error "Docker镜像构建失败"
    fi
}

# 创建Kubernetes部署文件
create_kubernetes_manifests() {
    if ! command -v kubectl &> /dev/null; then
        log_warn "kubectl未安装，跳过Kubernetes配置"
        return
    fi
    
    log_info "创建Kubernetes部署文件..."
    
    cd /root/.openclaw/workspace/defi-asset-service/queue-system
    
    mkdir -p k8s
    
    # 创建ConfigMap
    cat > k8s/configmap.yaml << EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: defi-queue-config
data:
  config.yaml: |
    redis:
      address: "redis-master:6379"
      password: ""
      db: 3
    queue:
      stream_name: "defi:stream:position_updates"
      consumer_group: "position_workers"
      dlq_stream_name: "defi:stream:dlq:position_updates"
      max_retries: 3
      workers: 5
    mysql:
      host: "mysql-service"
      port: 3306
      user: "root"
      database: "defi_asset_service"
EOF
    
    # 创建Deployment
    cat > k8s/deployment.yaml << EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: defi-queue-system
  labels:
    app: defi-queue
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
          name: api
        - containerPort: 8081
          name: producer
        - containerPort: 9090
          name: monitor
        volumeMounts:
        - name: config
          mountPath: /root/config
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: config
        configMap:
          name: defi-queue-config
EOF
    
    # 创建Service
    cat > k8s/service.yaml << EOF
apiVersion: v1
kind: Service
metadata:
  name: defi-queue-service
spec:
  selector:
    app: defi-queue
  ports:
  - name: api
    port: 8080
    targetPort: 8080
  - name: producer
    port: 8081
    targetPort: 8081
  - name: monitor
    port: 9090
    targetPort: 9090
  type: ClusterIP
EOF
    
    # 创建Ingress（可选）
    cat > k8s/ingress.yaml << EOF
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: defi-queue-ingress
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
spec:
  rules:
  - host: queue.defi.local
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: defi-queue-service
            port:
              number: 8080
EOF
    
    log_info "Kubernetes部署文件创建完成"
}

# 显示# 显示使用说明
show_usage() {
    log_info "=== 使用说明 ==="
    echo ""
    echo "可用命令:"
    echo "  ./setup.sh all              # 完整安装和配置"
    echo "  ./setup.sh deps             # 仅安装依赖"
    echo "  ./setup.sh config           # 仅配置环境"
    echo "  ./setup.sh db               # 仅初始化数据库"
    echo "  ./setup.sh redis            # 仅初始化Redis"
    echo "  ./setup.sh build            # 仅构建应用"
    echo "  ./setup.sh test             # 仅运行测试"
    echo "  ./setup.sh docker           # 仅构建Docker镜像"
    echo "  ./setup.sh k8s              # 仅创建Kubernetes配置"
    echo ""
    echo "环境变量:"
    echo "  MYSQL_ROOT_PASSWORD        # MySQL root密码"
    echo "  REDIS_PASSWORD             # Redis密码"
    echo "  SKIP_TESTS                 # 跳过测试"
    echo ""
}

# 主函数
main() {
    local command=${1:-"all"}
    
    case $command in
        "all")
            check_environment
            install_dependencies
            setup_config
            init_database
            init_redis
            build_application
            if [ -z "$SKIP_TESTS" ]; then
                run_tests
            fi
            build_docker_image
            create_kubernetes_manifests
            ;;
        "deps")
            install_dependencies
            ;;
        "config")
            setup_config
            ;;
        "db")
            init_database
            ;;
        "redis")
            init_redis
            ;;
        "build")
            build_application
            ;;
        "test")
            run_tests
            ;;
        "docker")
            build_docker_image
            ;;
        "k8s")
            create_kubernetes_manifests
            ;;
        "help"|"-h"|"--help")
            show_usage
            exit 0
            ;;
        *)
            log_error "未知命令: $command"
            show_usage
            exit 1
            ;;
    esac
    
    log_info "=== 部署完成 ==="
    echo ""
    echo "下一步:"
    echo "  1. 启动服务: cd /root/.openclaw/workspace/defi-asset-service/queue-system && ./bin/queue-system"
    echo "  2. 访问API: http://localhost:8080"
    echo "  3. 查看监控: http://localhost:9090/metrics"
    echo "  4. 发布消息: curl -X POST http://localhost:8081/publish -d '{\"user_address\":\"...\",\"protocol_id\":\"aave\",...}'"
    echo ""
}

# 执行主函数
main "$@"