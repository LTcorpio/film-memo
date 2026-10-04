# ============================================================
# 影迹（film-memo）—— 本地开发快捷命令
# 用法：项目根目录执行 `make`（默认打印帮助）
#
# 范围：只负责「启动」。后端固定为 Go 实现（server-go/），
#       仓库里早期的 Node 版 server/ 与根 package.json 的 npm run dev 不在本文件内。
# 端口 / 数据目录一律沿用根 .env（后端 config 会自下而上找到项目根再加载 .env），
# 本文件不覆盖任何配置：后端 4000、前端 5173（vite.config.js 已把 /api、/images
# 代理到 localhost:4000）。
# ============================================================

# 注：本项目路径含空格，不能用 $(abspath)（Make 3.81 会把路径写坏），
#     $(CURDIR) 是原样字符串，配合 shell 侧引号使用安全
ROOT       := $(CURDIR)/
SERVER_DIR := server-go
CLIENT_DIR := client

# go 有时不在非交互式 shell 的 PATH 里（本机 homebrew 兜底路径）
GO  ?= $(shell command -v go 2>/dev/null || echo /opt/homebrew/bin/go)
NPM ?= npm

# ---- Docker 部署：镜像在本项目构建，容器在统一 compose 栈内重建 ----
# 本机私有部署配置（compose 栈目录）放 deploy.mk，该文件不入库：
#   cp deploy.mk.example deploy.mk   # 再按本机路径填写 COMPOSE_DIR
# 也可在命令行临时覆盖：
#   make deploy COMPOSE_DIR="/path/to/compose"
-include deploy.mk

COMPOSE_DIR  ?=
FILM_IMAGE   ?= film-memo:latest
FILM_SERVICE ?= film-memo
DOCKER       ?= docker

# ---- 启动前检查（只检查，不安装任何东西）----
CHECK_GO  = command -v $(GO) >/dev/null 2>&1 || { printf '\033[31m✗ 未找到 go\033[0m，可用 GO=/path/to/go 指定\n'; exit 1; }
CHECK_NPM = [ -d "$(ROOT)$(CLIENT_DIR)/node_modules" ] || { printf '\033[31m✗ 前端依赖未安装\033[0m：请先执行 cd $(CLIENT_DIR) && npm install\n'; exit 1; }
CHECK_DOCKER  = command -v $(DOCKER) >/dev/null 2>&1 || { printf '\033[31m✗ 未找到 docker\033[0m\n'; exit 1; }

.DEFAULT_GOAL := help
.PHONY: help dev backend frontend deploy image restart

help:
	@printf '\n\033[1m影迹（film-memo）—— 本地开发命令\033[0m\n\n'
	@printf '  make dev      同时启动 Go 后端 + 前端（Ctrl-C 一并退出）\n'
	@printf '  make backend  仅启动 Go 后端（%s/，localhost:4000）\n' '$(SERVER_DIR)'
	@printf '  make frontend 仅启动前端（%s/，localhost:5173）\n' '$(CLIENT_DIR)'
	@printf '\n  端口与数据目录沿用根目录 .env（DB_PATH / IMAGES_DIR）\n\n'
	@printf '\n\033[1mDocker 部署命令\033[0m\n\n'
	@printf '  make deploy   一键部署：构建镜像 + 在 compose 栈内重建容器\n'
	@printf '  make image    仅构建镜像（%s）\n' '$(FILM_IMAGE)'
	@printf '  make restart  仅重建容器（需在 deploy.mk 或命令行给出 COMPOSE_DIR）\n'

## 仅启动 Go 后端
backend:
	@$(CHECK_GO)
	@cd "$(ROOT)$(SERVER_DIR)" && $(GO) run ./cmd/server

## 仅启动前端
frontend:
	@$(CHECK_NPM)
	@cd "$(ROOT)$(CLIENT_DIR)" && $(NPM) run dev

## 同时启动后端与前端；Ctrl-C 时先让后端优雅停机（WAL checkpoint 落盘）再退出
dev:
	@$(CHECK_GO)
	@$(CHECK_NPM)
	@printf '\033[36m▶ 同时启动 Go 后端 + 前端（Ctrl-C 一并退出）\033[0m\n'
	@sh -c ' \
	  bpid=""; guard=""; \
	  cleanup() { \
	    [ -n "$$bpid" ] || return 0; \
	    p="$$bpid"; bpid=""; \
	    printf "\n\033[33m⏹  正在停止后端（PID %s，等待优雅停机）…\033[0m\n" "$$p"; \
	    kill -INT "$$p" 2>/dev/null; \
	    ( sleep 5; kill -TERM "$$p" 2>/dev/null; sleep 2; kill -KILL "$$p" 2>/dev/null ) & guard=$$!; \
	    wait "$$p" 2>/dev/null; \
	    kill "$$guard" 2>/dev/null; \
	    printf "\033[33m   后端已退出\033[0m\n"; \
	  }; \
	  trap cleanup INT TERM EXIT; \
	  ( cd "$(ROOT)$(SERVER_DIR)" && exec $(GO) run ./cmd/server ) & bpid=$$!; \
	  ( cd "$(ROOT)$(CLIENT_DIR)" && exec $(NPM) run dev )'

# ============================================================
# Docker 部署
# ============================================================

## 一键部署：先在本项目构建镜像，再到统一 compose 栈内用新镜像重建容器
deploy: image restart

## 仅构建镜像（在当前项目路径执行 docker build）
image:
	@$(CHECK_DOCKER)
	@printf '\033[36m▶ 构建镜像 %s（上下文 %s）\033[0m\n' '$(FILM_IMAGE)' '$(ROOT)'
	@$(DOCKER) build -t $(FILM_IMAGE) "$(ROOT)"

## 仅在 compose 栈内重建容器（up -d 会检测到新镜像并重建，restart 不会换镜像）
restart:
	@$(CHECK_DOCKER)
	@[ -n "$(COMPOSE_DIR)" ] || { printf '\033[31m✗ 未设置 COMPOSE_DIR\033[0m：请先 cp deploy.mk.example deploy.mk 并填写，或执行 make deploy COMPOSE_DIR=/path/to/compose\n'; exit 1; }
	@[ -f "$(COMPOSE_DIR)/docker-compose.yml" ] || { printf '\033[31m✗ 未找到 compose 文件：%s/docker-compose.yml\033[0m\n' '$(COMPOSE_DIR)'; exit 1; }
	@printf '\033[36m▶ 重建容器 %s（%s）\033[0m\n' '$(FILM_SERVICE)' '$(COMPOSE_DIR)'
	@cd "$(COMPOSE_DIR)" && $(DOCKER) compose up -d $(FILM_SERVICE)
