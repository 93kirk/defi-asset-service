# DeFi资产展示服务 Makefile

.PHONY: help build run test clean docker-up docker-down migrate lint

# 默认目标
help:
	@echo "DeFi资产展示服务 - 构建和部署工具"
	@echo ""
	@echo "可用命令:"
	@echo "  build       编译项目"
	@echo "  run         运行开发服务器"
	@echo "  test        运行测试"
	@echo "  clean       清理构建文件"
	@echo "  docker-up   启动Docker服务"
	@echo "  docker-down 停止Docker服务"
	@echo "  migrate     运行数据库迁移"
	@echo "  lint        代码检查"
	@echo "  swagger     生成Swagger文档"
	@echo "  deploy      部署到生产环境"

# 编译项目
build:
	@echo "编译项目..."
	go build -o bin/api ./cmd/api

# 运行开发服务器
run:
	@echo "启动开发服务器..."
	go run cmd/api/main.go

# 运行测试
test:
	@echo "运行测试..."
	go test ./... -v

# 清理构建文件
clean:
	@echo "清理构建文件..."
	rm -rf bin/
	rm -rf coverage.out

# 启动Docker服务
docker-up:
	@echo "启动Docker服务..."
	docker-compose up -d

# 停止Docker服务
docker-down:
	@echo "停止Docker服务..."
	docker-compose down

# 重启Docker服务
docker-restart: docker-down docker-up

# 查看日志
logs:
	docker-compose logs -f api

# 运行数据库迁移
migrate:
	@echo "运行数据库迁移..."
	@if [ -f migrations/init.sql ]; then \
		echo "执行初始化脚本..."; \
		mysql -h localhost -u defi_user -pdefi_password defi_asset_service < migrations/init.sql; \
	else \
		echo "迁移文件不存在"; \
	fi

# 代码检查
lint:
	@echo "运行代码检查..."
	golangci-lint run ./...

# 格式化代码
fmt:
	@echo "格式化代码..."
	go fmt ./...

# 生成Swagger文档
swagger:
	@echo "生成Swagger文档..."
	swag init -g cmd/api/main.go -o docs/swagger

# 部署到生产环境
deploy:
	@echo "部署到生产环境..."
	@echo "1. 构建Docker镜像..."
	docker build -t defi-asset-service:latest .
	@echo "2. 推送镜像到仓库..."
	# docker push your-registry/defi-asset-service:latest
	@echo "3. 在生产服务器上拉取并运行..."
	# ssh user@server "docker pull your-registry/defi-asset-service:latest && docker-compose up -d"
	@echo "部署完成!"

# 健康检查
health:
	@echo "检查服务健康状态..."
	curl -f http://localhost:8080/health || echo "服务未运行"

# 性能测试
bench:
	@echo "运行性能测试..."
	ab -n 1000 -c 100 http://localhost:8080/v1/health

# 数据库备份
backup:
	@echo "备份数据库..."
	@TIMESTAMP=$$(date +%Y%m%d_%H%M%S); \
	mysqldump -h localhost -u defi_user -pdefi_password defi_asset_service > backup/defi_asset_service_$$TIMESTAMP.sql; \
	echo "备份完成: backup/defi_asset_service_$$TIMESTAMP.sql"

# 数据库恢复
restore:
	@echo "恢复数据库..."
	@if [ -z "$(FILE)" ]; then \
		echo "请指定备份文件: make restore FILE=backup/xxx.sql"; \
		exit 1; \
	fi; \
	mysql -h localhost -u defi_user -pdefi_password defi_asset_service < $(FILE); \
	echo "恢复完成!"

# 监控
monitor:
	@echo "打开监控面板..."
	@echo "Grafana: http://localhost:3000 (admin/admin)"
	@echo "Prometheus: http://localhost:9090"

# 进入容器
shell:
	docker-compose exec api sh

# 查看服务状态
status:
	@echo "服务状态:"
	docker-compose ps
	@echo ""
	@echo "资源使用:"
	docker stats --no-stream

# 更新依赖
update:
	@echo "更新Go依赖..."
	go get -u ./...
	go mod tidy

# 代码覆盖率
coverage:
	@echo "生成代码覆盖率报告..."
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告已生成: coverage.html"