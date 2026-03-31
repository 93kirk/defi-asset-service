#!/bin/bash

# DeFi数据同步系统部署脚本
# 版本: v1.0

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查命令是否存在
check_command() {
    if ! command -v $1 &> /dev/null; then
        log_error "命令 $1 未安装，请先安装"
        exit 1
    fi
}

# 显示帮助
show_help() {
    echo "DeFi数据同步系统部署脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help          显示帮助信息"
    echo "  -e, --environment   部署环境 (dev|staging|prod)"
    echo "  -c, --config        配置文件路径"
    echo "  -d, --docker        使用Docker部署"
    echo "  -k, --k8s           使用Kubernetes部署"
    echo "  -b, --build         构建应用"
    echo "  -t, --test          运行测试"
    echo "  -m, --migrate       运行数据库迁移"
    echo ""
    echo "示例:"
    echo "  $0 -e dev -d          # 开发环境Docker部署"
    echo "  $0 -e prod -k         # 生产环境K8s部署"
    echo "  $0 -b -t              # 构建并测试"
}

# 解析参数
ENVIRONMENT="dev"
USE_DOCKER=false
USE_K8S=false
BUILD_APP=false
RUN_TESTS=false
RUN_MIGRATE=false
CONFIG_FILE=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -e|--environment)
            ENVIRONMENT="$2"
            shift 2
            ;;
        -c|--config)
            CONFIG_FILE="$2"
            shift 2
            ;;
        -d|--docker)
            USE_DOCKER=true
            shift
            ;;
        -k|--k8s)
            USE_K8S=true
            shift
            ;;
        -b|--build)
            BUILD_APP=true
            shift
            ;;
        -t|--test)
            RUN_TESTS=true
            shift
            ;;
        -m|--migrate)
            RUN_MIGRATE=true
            shift
            ;;
        *)
            log_error "未知选项: $1"
            show_help
            exit 1
            ;;
    esac
done

# 检查必需命令
check_command git
check_command go

if $USE_DOCKER; then
    check_command docker
    check_command docker-compose
fi

if $USE_K8S; then
    check_command kubectl
    check_command helm
fi

# 加载环境配置
load_environment_config() {
    local env=$1
    local config_file=${CONFIG_FILE:-"config/config.$env.yaml"}
    
    if [ ! -f "$config_file" ]; then
        log_warning "配置文件 $config_file 不存在，使用默认配置"
        cp config/config.example.yaml "$config_file"
        sed -i "s/environment:.*/environment: \"$env\"/" "$config_file"
    fi
    
    log_info "使用环境: $env, 配置文件: $config_file"
    
    # 导出环境变量
    export ENVIRONMENT=$env
    export CONFIG_FILE=$config_file
}

# 构建应用
build_application() {
    log_info "开始构建应用..."
    
    # 清理旧构建
    rm -rf bin/ dist/
    
    # 下载依赖
    log_info "下载Go依赖..."
    go mod download
    
    # 运行测试
    if $RUN_TESTS; then
        log_info "运行测试..."
        go test ./internal/... -v
        if [ $? -ne 0 ]; then
            log_error "测试失败"
            exit 1
        fi
    fi
    
    # 构建二进制文件
    log_info "构建二进制文件..."
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/data-sync ./cmd/data-sync
    
    # 构建Docker镜像
    if $USE_DOCKER; then
        log_info "构建Docker镜像..."
        docker build -t defi-data-sync:latest -t defi-data-sync:$ENVIRONMENT .
    fi
    
    log_success "应用构建完成"
}

# 运行数据库迁移
run_migrations() {
    log_info "运行数据库迁移..."
    
    # 检查数据库连接
    if [ -z "$DB_HOST" ]; then
        log_error "数据库主机未设置"
        exit 1
    fi
    
    # 运行初始化脚本
    if [ -f "init.sql" ]; then
        log_info "执行数据库初始化..."
        mysql -h $DB_HOST -u $DB_USER -p$DB_PASSWORD $DB_NAME < init.sql
    fi
    
    # 运行迁移脚本（如果有）
    if [ -d "migrations" ]; then
        log_info "执行数据库迁移..."
        for migration in migrations/*.sql; do
            log_info "执行迁移: $migration"
            mysql -h $DB_HOST -u $DB_USER -p$DB_PASSWORD $DB_NAME < $migration
        done
    fi
    
    log_success "数据库迁移完成"
}

# Docker部署
deploy_with_docker() {
    log_info "使用Docker Compose部署..."
    
    # 创建网络（如果不存在）
    docker network create defi-network 2>/dev/null || true
    
    # 启动服务
    docker-compose -f docker-compose.yml up -d
    
    # 等待服务启动
    log_info "等待服务启动..."
    sleep 30
    
    # 检查服务状态
    if docker-compose -f docker-compose.yml ps | grep -q "Up"; then
        log_success "Docker服务启动成功"
        
        # 显示服务信息
        echo ""
        echo "服务状态:"
        docker-compose -f docker-compose.yml ps
        
        echo ""
        echo "访问地址:"
        echo "  - 数据同步服务: http://localhost:8080"
        echo "  - 健康检查: http://localhost:8080/health"
        echo "  - 监控指标: http://localhost:9091/metrics"
        echo "  - Grafana: http://localhost:3000 (admin/admin)"
        echo "  - Adminer: http://localhost:8081"
        echo ""
    else
        log_error "Docker服务启动失败"
        docker-compose -f docker-compose.yml logs
        exit 1
    fi
}

# Kubernetes部署
deploy_with_kubernetes() {
    log_info "使用Kubernetes部署..."
    
    # 创建命名空间
    kubectl create namespace defi-system 2>/dev/null || true
    
    # 创建配置
    if [ -f "k8s/configmap.yaml" ]; then
        kubectl apply -f k8s/configmap.yaml -n defi-system
    fi
    
    if [ -f "k8s/secret.yaml" ]; then
        kubectl apply -f k8s/secret.yaml -n defi-system
    fi
    
    # 部署应用
    if [ -f "k8s/deployment.yaml" ]; then
        kubectl apply -f k8s/deployment.yaml -n defi-system
    fi
    
    if [ -f "k8s/service.yaml" ]; then
        kubectl apply -f k8s/service.yaml -n defi-system
    fi
    
    if [ -f "k8s/ingress.yaml" ]; then
        kubectl apply -f k8s/ingress.yaml -n defi-system
    fi
    
    # 等待部署完成
    log_info "等待部署完成..."
    kubectl wait --for=condition=available --timeout=300s deployment/defi-data-sync -n defi-system
    
    # 显示部署状态
    log_success "Kubernetes部署完成"
    echo ""
    echo "部署状态:"
    kubectl get all -n defi-system
    
    echo ""
    echo "Pod日志:"
    kubectl logs -l app=defi-data-sync -n defi-system --tail=10
}

# 直接部署
deploy_directly() {
    log_info "直接部署应用..."
    
    # 检查二进制文件
    if [ ! -f "bin/data-sync" ]; then
        log_error "二进制文件不存在，请先构建应用"
        exit 1
    fi
    
    # 停止现有进程
    if pgrep -f "data-sync" > /dev/null; then
        log_info "停止现有进程..."
        pkill -f "data-sync"
        sleep 5
    fi
    
    # 启动应用
    log_info "启动应用..."
    nohup ./bin/data-sync > logs/data-sync.log 2>&1 &
    
    # 检查进程
    sleep 5
    if pgrep -f "data-sync" > /dev/null; then
        log_success "应用启动成功"
        echo ""
        echo "进程ID: $(pgrep -f "data-sync")"
        echo "日志文件: logs/data-sync.log"
        echo "访问地址: http://localhost:8080"
    else
        log_error "应用启动失败"
        tail -20 logs/data-sync.log
        exit 1
    fi
}

# 健康检查
health_check() {
    log_info "执行健康检查..."
    
    local max_retries=30
    local retry_count=0
    
    while [ $retry_count -lt $max_retries ]; do
        if curl -s http://localhost:8080/health > /dev/null; then
            log_success "健康检查通过"
            return 0
        fi
        
        retry_count=$((retry_count + 1))
        log_info "健康检查失败，重试 $retry_count/$max_retries..."
        sleep 5
    done
    
    log_error "健康检查失败，服务未响应"
    return 1
}

# 监控部署
monitor_deployment() {
    log_info "开始监控部署..."
    
    # 检查服务状态
    health_check
    
    # 显示监控信息
    echo ""
    echo "监控信息:"
    echo "  - 服务状态: $(curl -s http://localhost:8080/health | jq -r .status 2>/dev/null || echo "未知")"
    echo "  - 同步状态: $(curl -s http://localhost:8080/api/v1/sync/status | jq -r .last_sync 2>/dev/null || echo "未知")"
    echo "  - 协议数量: $(curl -s http://localhost:8080/api/v1/protocols | jq -r '.total' 2>/dev/null || echo "未知")"
    
    # 显示日志
    echo ""
    echo "最近日志:"
    tail -10 logs/data-sync.log 2>/dev/null || echo "日志文件不存在"
}

# 主函数
main() {
    log_info "开始部署DeFi数据同步系统"
    log_info "部署环境: $ENVIRONMENT"
    
    # 加载环境配置
    load_environment_config $ENVIRONMENT
    
    # 构建应用
    if $BUILD_APP || [ "$USE_DOCKER" = false ] && [ "$USE_K8S" = false ]; then
        build_application
    fi
    
    # 运行数据库迁移
    if $RUN_MIGRATE; then
        run_migrations
    fi
    
    # 选择部署方式
    if $USE_DOCKER; then
        deploy_with_docker
    elif $USE_K8S; then
        deploy_with_kubernetes
    else
        deploy_directly
    fi
    
    # 监控部署
    monitor_deployment
    
    log_success "部署完成!"
}

# 执行主函数
main "$@"