# DeFi资产展示服务 - 外部服务集成 Makefile

.PHONY: all build test clean lint format vet docker docker-compose help

# 变量定义
BINARY_NAME=defi-external-service
VERSION=1.0.0
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD)
GO_VERSION=$(shell go version)
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT) -X main.GoVersion=$(GO_VERSION)"

# 默认目标
all: build test

# 构建项目
build:
	@echo "构建 $(BINARY_NAME)..."
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./example/main.go
	@echo "构建完成: bin/$(BINARY_NAME)"

# 运行测试
test:
	@echo "运行单元测试..."
	go test ./client/... -v -cover
	go test ./retry/... -v -cover
	go test ./rate_limit/... -v -cover
	go test ./circuit_breaker/... -v -cover

# 运行集成测试
test-integration:
	@echo "运行集成测试..."
	@if [ -f docker-compose.yml ]; then \
		docker-compose up -d mysql redis; \
		sleep 10; \
		go test ./integration/... -v -tags=integration; \
		docker-compose down; \
	else \
		echo "docker-compose.yml 不存在，跳过集成测试"; \
	fi

# 运行性能测试
bench:
	@echo "运行性能测试..."
	go test ./client/... -bench=. -benchmem -benchtime=5s
	go test ./retry/... -bench=. -benchmem -benchtime=5s
	go test ./rate_limit/... -bench=. -benchmem -benchtime=5s

# 代码质量检查
lint:
	@echo "代码检查..."
	golangci-lint run ./...

# 代码格式化
format:
	@echo "代码格式化..."
	go fmt ./...
	gofmt -s -w .

# 代码静态分析
vet:
	@echo "静态分析..."
	go vet ./...

# 生成依赖图
deps:
	@echo "生成依赖图..."
	go mod graph | dot -Tpng -o deps.png

# 清理构建文件
clean:
	@echo "清理..."
	rm -rf bin/
	rm -rf coverage/
	rm -f deps.png
	go clean -cache -testcache

# 运行示例
run-example:
	@echo "运行示例..."
	go run ./example/main.go

# Docker构建
docker-build:
	@echo "构建Docker镜像..."
	docker build -t $(BINARY_NAME):$(VERSION) -t $(BINARY_NAME):latest .

# Docker运行
docker-run:
	@echo "运行Docker容器..."
	docker run --rm -p 8080:8080 --name $(BINARY_NAME) $(BINARY_NAME):latest

# Docker Compose启动
docker-compose-up:
	@echo "启动Docker Compose..."
	docker-compose up -d

# Docker Compose停止
docker-compose-down:
	@echo "停止Docker Compose..."
	docker-compose down

# 生成文档
docs:
	@echo "生成文档..."
	godoc -http=:6060 &
	open http://localhost:6060/pkg/defi-asset-service/external-service-integration/ || true

# 代码覆盖率
coverage:
	@echo "生成代码覆盖率报告..."
	mkdir -p coverage
	go test ./... -coverprofile=coverage/coverage.out
	go tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "覆盖率报告: coverage/coverage.html"

# 安全检查
security:
	@echo "安全检查..."
	gosec ./...
	nancy go.sum

# 更新依赖
update-deps:
	@echo "更新依赖..."
	go get -u ./...
	go mod tidy

# 验证依赖
verify-deps:
	@echo "验证依赖..."
	go mod verify
	go mod download

# 发布准备
release: clean build test lint vet security
	@echo "发布准备完成: $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "构建时间: $(BUILD_TIME)"

# 帮助信息
help:
	@echo "DeFi资产展示服务 - 外部服务集成 Makefile"
	@echo ""
	@echo "可用命令:"
	@echo "  make build        - 构建项目"
	@echo "  make test         - 运行单元测试"
	@echo "  make test-integration - 运行集成测试"
	@echo "  make bench        - 运行性能测试"
	@echo "  make lint         - 代码检查"
	@echo "  make format       - 代码格式化"
	@echo "  make vet          - 静态分析"
	@echo "  make deps         - 生成依赖图"
	@echo "  make clean        - 清理构建文件"
	@echo "  make run-example  - 运行示例"
	@echo "  make docker-build - 构建Docker镜像"
	@echo "  make docker-run   - 运行Docker容器"
	@echo "  make docker-compose-up - 启动Docker Compose"
	@echo "  make docker-compose-down - 停止Docker Compose"
	@echo "  make docs         - 生成文档"
	@echo "  make coverage     - 生成代码覆盖率报告"
	@echo "  make security     - 安全检查"
	@echo "  make update-deps  - 更新依赖"
	@echo "  make verify-deps  - 验证依赖"
	@echo "  make release      - 发布准备"
	@echo "  make help         - 显示帮助信息"
	@echo ""
	@echo "环境变量:"
	@echo "  BINARY_NAME=$(BINARY_NAME)"
	@echo "  VERSION=$(VERSION)"
	@echo "  GIT_COMMIT=$(GIT_COMMIT)"

# Dockerfile
Dockerfile:
	@echo "创建Dockerfile..."
	@cat > Dockerfile << 'EOF'
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 安装必要的工具
RUN apk add --no-cache git ca-certificates tzdata

# 复制go.mod和go.sum
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./example/main.go

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# 从构建阶段复制二进制文件
COPY --from=builder /app/main .

# 复制配置文件
COPY config/config_template.yaml ./config.yaml

# 暴露端口
EXPOSE 8080

# 运行应用
CMD ["./main"]
EOF

# docker-compose.yml
docker-compose.yml:
	@echo "创建docker-compose.yml..."
	@cat > docker-compose.yml << 'EOF'
version: '3.8'

services:
  # MySQL数据库
  mysql:
    image: mysql:8.0
    container_name: defi-mysql
    environment:
      MYSQL_ROOT_PASSWORD: root
      MYSQL_DATABASE: defi_asset_service
      MYSQL_USER: defi_user
      MYSQL_PASSWORD: defi_password
    ports:
      - "3306:3306"
    volumes:
      - mysql_data:/var/lib/mysql
      - ./database-schema.sql:/docker-entrypoint-initdb.d/init.sql
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "localhost"]
      timeout: 20s
      retries: 10

  # Redis缓存
  redis:
    image: redis:7.0-alpine
    container_name: defi-redis
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    command: redis-server --appendonly yes
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

  # 应用服务
  app:
    build: .
    container_name: defi-app
    depends_on:
      mysql:
        condition: service_healthy
      redis:
        condition: service_healthy
    environment:
      DB_HOST: mysql
      DB_PORT: 3306
      DB_USERNAME: defi_user
      DB_PASSWORD: defi_password
      DB_NAME: defi_asset_service
      REDIS_HOST: redis
      REDIS_PORT: 6379
      SERVICE_A_API_KEY: ${SERVICE_A_API_KEY}
      SERVICE_B_API_KEY: ${SERVICE_B_API_KEY}
    ports:
      - "8080:8080"
    volumes:
      - ./config:/app/config
    command: ["./main"]

  # 监控服务 (可选)
  prometheus:
    image: prom/prometheus:latest
    container_name: defi-prometheus
    ports:
      - "9090:9090"
    volumes:
      - ./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
      - '--storage.tsdb.retention.time=200h'
      - '--web.enable-lifecycle'

  grafana:
    image: grafana/grafana:latest
    container_name: defi-grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana_data:/var/lib/grafana
      - ./monitoring/grafana/dashboards:/etc/grafana/provisioning/dashboards
      - ./monitoring/grafana/datasources:/etc/grafana/provisioning/datasources

volumes:
  mysql_data:
  redis_data:
  prometheus_data:
  grafana_data:
EOF

# .gitignore
gitignore:
	@echo "创建.gitignore..."
	@cat > .gitignore << 'EOF'
# 二进制文件
bin/
*.exe
*.exe~
*.dll
*.so
*.dylib

# 测试输出
*.test
*.out
coverage/

# 依赖
vendor/

# 开发环境
.env
.env.local
.env.development.local
.env.test.local
.env.production.local

# IDE文件
.vscode/
.idea/
*.swp
*.swo
*~

# 构建输出
dist/
build/
*.deps.png

# 操作系统文件
.DS_Store
Thumbs.db

# 日志文件
*.log
logs/

# 临时文件
tmp/
temp/

# 配置文件（不忽略模板）
!config/config_template.yaml
config/config.yaml
config/config.local.yaml

# Docker
.dockerignore
docker-compose.override.yml

# 文档生成
docs/site/
EOF