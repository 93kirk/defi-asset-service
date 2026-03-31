# DeFi数据同步系统 Makefile
# 版本: v1.0

.PHONY: help build test run clean docker docker-run docker-down migrate lint format

# 默认目标
.DEFAULT_GOAL := help

# 帮助信息
help:
	@echo "DeFi数据同步系统 - 构建和部署工具"
	@echo ""
	@echo "可用命令:"
	@echo "  make build       构建应用"
	@echo "  make test        运行测试"
	@echo "  make run         运行应用"
	@echo "  make clean       清理构建文件"
	@echo "  make docker      构建Docker镜像"
	@echo "  make docker-run  使用Docker Compose运行"
	@echo "  make docker-down 停止Docker服务"
	@echo "  make migrate     运行数据库迁移"
	@echo "  make lint        代码检查"
	@echo "  make format      代码格式化"
	@echo "  make deploy      部署到指定环境"
	@echo "  make monitor     监控服务状态"
	@echo "  make logs        查看日志"
	@echo ""

# 变量定义
APP_NAME := defi-data-sync
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION := $(shell go version | awk '{print $$3}')
LDFLAGS := -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.goVersion=$(GO_VERSION)

# 构建应用
build:
	@echo "构建 $(APP_NAME) $(VERSION)..."
	@mkdir -p bin
	@CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -a -installsuffix cgo -o bin/$(APP_NAME) ./cmd/data-sync
	@echo "构建完成: bin/$(APP_NAME)"

# 运行测试
test:
	@echo "运行测试..."
	@go test ./internal/... -v -coverprofile=coverage.out
	@go tool cover -func=coverage.out

# 运行应用
run: build
	@echo "启动 $(APP_NAME)..."
	@./bin/$(APP_NAME)

# 清理构建文件
clean:
	@echo "清理构建文件..."
	@rm -rf bin/ dist/ coverage.out
	@go clean

# 构建Docker镜像
docker:
	@echo "构建Docker镜像..."
	@docker build -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .

# 使用Docker Compose运行
docker-run:
	@echo "启动Docker服务..."
	@docker-compose up -d
	@echo "服务已启动，使用 'make logs' 查看日志"

# 停止Docker服务
docker-down:
	@echo "停止Docker服务..."
	@docker-compose down
	@echo "服务已停止"

# 运行数据库迁移
migrate:
	@echo "运行数据库迁移..."
	@if [ -f "init.sql" ]; then \
		echo "执行初始化脚本..."; \
		mysql -h localhost -u root -ppassword defi_asset_service < init.sql; \
	fi
	@if [ -d "migrations" ]; then \
		for migration in migrations/*.sql; do \
			echo "执行迁移: $$migration"; \
			mysql -h localhost -u root -ppassword defi_asset_service < $$migration; \
		done \
	fi
	@echo "数据库迁移完成"

# 代码检查
lint:
	@echo "运行代码检查..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint 未安装，跳过代码检查"; \
		echo "安装命令: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# 代码格式化
format:
	@echo "格式化代码..."
	@go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		echo "goimports 未安装，跳过导入格式化"; \
		echo "安装命令: go install golang.org/x/tools/cmd/goimports@latest"; \
	fi

# 部署到指定环境
deploy: build
	@echo "部署 $(APP_NAME) 到 $(ENVIRONMENT) 环境..."
	@if [ -z "$(ENVIRONMENT)" ]; then \
		echo "请指定环境: make deploy ENVIRONMENT=dev|staging|prod"; \
		exit 1; \
	fi
	@./scripts/deploy.sh -e $(ENVIRONMENT) -b

# 监控服务状态
monitor:
	@echo "监控服务状态..."
	@if command -v watch >/dev/null 2>&1; then \
		watch -n 5 'curl -s http://localhost:8080/health | jq . 2>/dev/null || echo "服务未响应"'; \
	else \
		echo "watch 命令未安装，使用简单检查"; \
		curl -s http://localhost:8080/health | jq . 2>/dev/null || echo "服务未响应"; \
	fi

# 查看日志
logs:
	@echo "查看服务日志..."
	@if docker-compose ps | grep -q "Up"; then \
		docker-compose logs -f; \
	elif [ -f "logs/data-sync.log" ]; then \
		tail -f logs/data-sync.log; \
	else \
		echo "未找到日志文件"; \
	fi

# 生成API文档
docs:
	@echo "生成API文档..."
	@if command -v swag >/dev/null 2>&1; then \
		swag init -g cmd/data-sync/main.go -o docs/swagger; \
		echo "API文档已生成到 docs/swagger"; \
	else \
		echo "swag 未安装，跳过文档生成"; \
		echo "安装命令: go install github.com/swaggo/swag/cmd/swag@latest"; \
	fi

# 性能测试
bench:
	@echo "运行性能测试..."
	@go test ./internal/... -bench=. -benchmem

# 安全检查
security:
	@echo "运行安全检查..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec 未安装，跳过安全检查"; \
		echo "安装命令: go install github.com/securego/gosec/v2/cmd/gosec@latest"; \
	fi

# 依赖检查
deps:
	@echo "检查依赖..."
	@go mod tidy
	@go mod verify
	@go list -m all | head -20

# 发布版本
release: test lint security build docker
	@echo "发布版本 $(VERSION)..."
	@docker tag $(APP_NAME):latest $(APP_NAME):$(VERSION)
	@echo "Docker镜像已标记: $(APP_NAME):$(VERSION)"
	@echo "发布命令:"
	@echo "  docker push $(APP_NAME):$(VERSION)"
	@echo "  docker push $(APP_NAME):latest"

# 开发环境设置
dev-setup:
	@echo "设置开发环境..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install golang.org/x/tools/cmd/goimports@latest
	@go install github.com/swaggo/swag/cmd/swag@latest
	@go install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "开发工具已安装"

# 数据库备份
backup:
	@echo "备份数据库..."
	@mkdir -p backups
	@mysqldump -h localhost -u root -ppassword defi_asset_service > backups/defi_asset_service_$(shell date +%Y%m%d_%H%M%S).sql
	@echo "数据库备份完成: backups/defi_asset_service_$(shell date +%Y%m%d_%H%M%S).sql"

# 数据库恢复
restore:
	@echo "恢复数据库..."
	@if [ -z "$(BACKUP_FILE)" ]; then \
		echo "请指定备份文件: make restore BACKUP_FILE=backups/xxx.sql"; \
		exit 1; \
	fi
	@mysql -h localhost -u root -ppassword defi_asset_service < $(BACKUP_FILE)
	@echo "数据库恢复完成"

# 查看系统状态
status:
	@echo "系统状态:"
	@echo "  应用版本: $(VERSION)"
	@echo "  构建时间: $(BUILD_TIME)"
	@echo "  Go版本: $(GO_VERSION)"
	@echo ""
	@echo "服务状态:"
	@if docker-compose ps | grep -q "Up"; then \
		docker-compose ps; \
	else \
		echo "  Docker服务未运行"; \
	fi
	@echo ""
	@echo "数据库连接:"
	@if mysql -h localhost -u root -ppassword -e "SELECT 1" >/dev/null 2>&1; then \
		echo "  MySQL: 正常"; \
	else \
		echo "  MySQL: 异常"; \
	fi
	@if redis-cli ping >/dev/null 2>&1; then \
		echo "  Redis: 正常"; \
	else \
		echo "  Redis: 异常"; \
	fi