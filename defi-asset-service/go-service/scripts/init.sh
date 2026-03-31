#!/bin/bash

# DeFi资产展示服务 - 项目初始化脚本

set -e

echo "=== DeFi资产展示服务初始化 ==="

# 1. 下载Go依赖
echo "1. 下载Go依赖..."
go mod download

# 2. 创建必要的目录
echo "2. 创建目录结构..."
mkdir -p bin logs backup

# 3. 复制环境变量文件
echo "3. 设置环境变量..."
if [ ! -f .env ]; then
    cp .env.example .env
    echo "请编辑 .env 文件配置数据库和Redis连接信息"
fi

# 4. 创建配置文件
echo "4. 检查配置文件..."
if [ ! -f configs/config.yaml ]; then
    echo "配置文件 configs/config.yaml 不存在，请确保已创建"
fi

# 5. 检查Docker
echo "5. 检查Docker环境..."
if command -v docker &> /dev/null; then
    echo "Docker已安装"
else
    echo "警告: Docker未安装，将无法使用容器化部署"
fi

# 6. 尝试编译
echo "6. 尝试编译项目..."
if go build -o bin/api ./cmd/api; then
    echo "编译成功!"
else
    echo "编译失败，请检查错误信息"
    exit 1
fi

# 7. 生成Swagger文档（如果安装了swag）
echo "7. 生成API文档..."
if command -v swag &> /dev/null; then
    swag init -g cmd/api/main.go -o docs/swagger
    echo "Swagger文档已生成"
else
    echo "跳过Swagger文档生成（swag命令未安装）"
fi

echo ""
echo "=== 初始化完成 ==="
echo ""
echo "下一步操作:"
echo "1. 编辑 .env 文件配置数据库连接"
echo "2. 启动依赖服务: make docker-up"
echo "3. 初始化数据库: make migrate"
echo "4. 运行应用: make run"
echo ""
echo "常用命令:"
echo "  make run        - 启动开发服务器"
echo "  make docker-up  - 启动Docker服务"
echo "  make test       - 运行测试"
echo "  make lint       - 代码检查"
echo ""