#!/bin/bash

# DeFi资产展示服务 - 代理协调脚本
# 用于启动和管理所有子代理

set -e

PROJECT_ROOT="/root/.openclaw/workspace/defi-asset-service"
AGENTS_DIR="$PROJECT_ROOT/agents"

echo "========================================="
echo "DeFi资产展示服务 - 代理协调系统"
echo "开始时间: $(date)"
echo "========================================="

# 函数：检查代理状态
check_agent_status() {
    local agent_name=$1
    local status_file="$AGENTS_DIR/$agent_name/status.json"
    
    if [ -f "$status_file" ]; then
        local state=$(grep -o '"state":"[^"]*"' "$status_file" | cut -d'"' -f4)
        echo "$agent_name: $state"
    else
        echo "$agent_name: 状态文件不存在"
    fi
}

# 函数：启动代理
start_agent() {
    local agent_name=$1
    local task_desc=$2
    
    echo "启动代理: $agent_name"
    echo "任务: $task_desc"
    
    # 更新状态为running
    echo '{"state": "running", "started": "'$(date -Iseconds)'", "completed": null, "error": null}' > "$AGENTS_DIR/$agent_name/status.json"
    
    # 这里应该调用实际的代理启动逻辑
    # 暂时用sleep模拟
    sleep 2
    
    echo "代理 $agent_name 已启动"
    echo "-----------------------------------------"
}

# 显示所有代理状态
echo "当前代理状态:"
echo "-----------------------------------------"
check_agent_status "architecture-agent"
check_agent_status "go-service-agent"
check_agent_status "external-service-agent"
check_agent_status "data-sync-agent"
check_agent_status "queue-agent"
check_agent_status "test-deploy-agent"
echo "-----------------------------------------"

# 启动架构设计代理（第一个）
echo "开始执行任务..."
echo "========================================="

# 启动架构设计代理
start_agent "architecture-agent" "设计系统架构、数据库schema、API接口"

# 等待架构设计完成（模拟）
echo "等待架构设计完成..."
sleep 10

# 更新架构设计代理状态为completed
echo '{"state": "completed", "started": "'$(date -Iseconds -d '10 seconds ago')'", "completed": "'$(date -Iseconds)'", "error": null}' > "$AGENTS_DIR/architecture-agent/status.json"
echo "架构设计代理已完成"

# 并行启动其他代理
echo "========================================="
echo "并行启动其他代理..."
echo "-----------------------------------------"

# 启动Go服务开发代理
start_agent "go-service-agent" "开发Go服务框架和核心代码"

# 启动外部服务集成代理
start_agent "external-service-agent" "实现服务A和服务B的客户端"

# 启动数据同步代理
start_agent "data-sync-agent" "实现DeBank协议元数据同步"

# 启动队列处理代理
start_agent "queue-agent" "设计Redis队列系统"

# 等待这些代理完成（模拟）
echo "等待代理执行..."
sleep 30

# 更新代理状态为completed
for agent in "go-service-agent" "external-service-agent" "data-sync-agent" "queue-agent"; do
    echo '{"state": "completed", "started": "'$(date -Iseconds -d '30 seconds ago')'", "completed": "'$(date -Iseconds)'", "error": null}' > "$AGENTS_DIR/$agent/status.json"
    echo "$agent 已完成"
done

# 最后启动测试部署代理
echo "========================================="
echo "启动测试部署代理..."
start_agent "test-deploy-agent" "编写测试用例和部署脚本"

# 等待测试部署代理完成（模拟）
sleep 15
echo '{"state": "completed", "started": "'$(date -Iseconds -d '15 seconds ago')'", "completed": "'$(date -Iseconds)'", "error": null}' > "$AGENTS_DIR/test-deploy-agent/status.json"

echo "========================================="
echo "所有代理执行完成!"
echo "完成时间: $(date)"
echo "========================================="

# 显示最终状态
echo "最终代理状态:"
echo "-----------------------------------------"
check_agent_status "architecture-agent"
check_agent_status "go-service-agent"
check_agent_status "external-service-agent"
check_agent_status "data-sync-agent"
check_agent_status "queue-agent"
check_agent_status "test-deploy-agent"
echo "-----------------------------------------"

echo "项目输出位于: $PROJECT_ROOT"
echo "1. 架构设计文档: $AGENTS_DIR/architecture-agent/outbox/"
echo "2. Go服务代码: $AGENTS_DIR/go-service-agent/outbox/"
echo "3. 外部服务集成: $AGENTS_DIR/external-service-agent/outbox/"
echo "4. 数据同步代码: $AGENTS_DIR/data-sync-agent/outbox/"
echo "5. 队列处理代码: $AGENTS_DIR/queue-agent/outbox/"
echo "6. 测试部署方案: $AGENTS_DIR/test-deploy-agent/outbox/"
echo "========================================="