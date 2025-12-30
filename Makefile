.PHONY: help run build test clean tidy install-tools swagger lint fmt pre-commit \
       bench-init bench-clean bench-generate bench-run bench-all bench-stats

help: ## 显示帮助信息
	@echo "可用命令:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

run: ## 运行应用
	go run cmd/server/main.go

build: ## 编译应用
	go build -o bin/server cmd/server/main.go

test: ## 运行测试
	go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...

test-coverage: test ## 运行测试并生成覆盖率报告
	go tool cover -html=coverage.txt -o coverage.html

clean: ## 清理构建产物
	rm -rf bin/
	rm -f coverage.txt coverage.html

tidy: ## 整理依赖
	go mod tidy

install-tools: ## 安装开发工具
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	go install github.com/air-verse/air@v1.52.3

swagger: ## 生成 Swagger 文档
	swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal

lint: ## 运行代码检查
	golangci-lint run ./...

lint-fix: ## 运行代码检查并自动修复
	golangci-lint run --fix ./...

fmt: ## 格式化代码
	go fmt ./...
	goimports -w -local github.com/d60-Lab/gin-template .

pre-commit: ## 运行 pre-commit 检查所有文件
	pre-commit run --all-files

pre-commit-install: ## 安装 pre-commit hooks
	pre-commit install
	pre-commit install --hook-type commit-msg

docker-build: ## 构建 Docker 镜像
	docker build -t gin-template:latest .

docker-run: ## 运行 Docker 容器
	docker run -p 8080:8080 gin-template:latest

dev: ## 开发模式运行（使用 air 热重载）
	air

init-db: ## 初始化数据库
	createdb gin_template || true

ci: lint test build ## 运行 CI 流程（lint + test + build）

verify: fmt lint test ## 提交前验证（格式化 + lint + 测试）

# ============================================
# Benchmark 相关命令
# ============================================

# 数据库配置 (可通过环境变量覆盖)
DB_USER ?= root
DB_PASS ?= 123456
DB_HOST ?= 127.0.0.1
DB_PORT ?= 3306
DB_NAME ?= gin_template

MYSQL_CMD = mysql -u$(DB_USER) -p$(DB_PASS) -h$(DB_HOST) -P$(DB_PORT)

bench-init: ## 初始化benchmark数据库（创建库和表）
	@echo "🔧 初始化数据库..."
	@$(MYSQL_CMD) -e "DROP DATABASE IF EXISTS $(DB_NAME); CREATE DATABASE $(DB_NAME);"
	@$(MYSQL_CMD) $(DB_NAME) < migrations/001_permission_comparison_schema.sql
	@echo "✅ 数据库初始化完成"

bench-clean: ## 清空benchmark测试数据（保留表结构）
	@echo "🗑️  清空数据库表..."
	@$(MYSQL_CMD) $(DB_NAME) -e "\
		SET FOREIGN_KEY_CHECKS=0; \
		DELETE FROM document_reads; \
		DELETE FROM relation_tuples; \
		DELETE FROM document_permissions_mysql; \
		DELETE FROM documents; \
		DELETE FROM customer_followers; \
		DELETE FROM customers; \
		DELETE FROM management_relations; \
		DELETE FROM user_departments; \
		DELETE FROM departments; \
		DELETE FROM users; \
		SET FOREIGN_KEY_CHECKS=1;" 2>/dev/null || (echo "⚠️  表不存在，先初始化..." && $(MAKE) bench-init)
	@echo "✅ 数据库表已清空"

bench-reset: ## 重置数据库（删除并重建）
	@echo "🔄 重置数据库..."
	@$(MAKE) bench-init
	@echo "✅ 数据库已重置"

bench-generate: ## 生成benchmark测试数据
	@echo "🎲 生成测试数据..."
	go run cmd/production-test/main.go generate

bench-run: ## 运行benchmark测试
	@echo "⚡ 运行性能测试..."
	go run cmd/production-test/main.go benchmark

bench-all: bench-clean bench-generate bench-run ## 完整benchmark流程（清空+生成+测试）
	@echo ""
	@echo "╔════════════════════════════════════════════════════════════╗"
	@echo "║              🎉 Benchmark 完成!                           ║"
	@echo "║         结果保存在 ./benchmark-results-production         ║"
	@echo "╚════════════════════════════════════════════════════════════╝"

bench-stats: ## 查看数据库统计信息
	@echo "📊 数据库统计:"
	@$(MYSQL_CMD) $(DB_NAME) -e "\
		SELECT 'users' as table_name, COUNT(*) as count FROM users \
		UNION ALL SELECT 'departments', COUNT(*) FROM departments \
		UNION ALL SELECT 'customers', COUNT(*) FROM customers \
		UNION ALL SELECT 'documents', COUNT(*) FROM documents \
		UNION ALL SELECT 'customer_followers', COUNT(*) FROM customer_followers \
		UNION ALL SELECT 'document_reads', COUNT(*) FROM document_reads \
		UNION ALL SELECT 'document_permissions_mysql', COUNT(*) FROM document_permissions_mysql \
		UNION ALL SELECT 'relation_tuples', COUNT(*) FROM relation_tuples \
		ORDER BY table_name;"
