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

# ---- 启动前检查（只检查，不安装任何东西）----
CHECK_GO  = command -v $(GO) >/dev/null 2>&1 || { printf '\033[31m✗ 未找到 go\033[0m，可用 GO=/path/to/go 指定\n'; exit 1; }
CHECK_NPM = [ -d "$(ROOT)$(CLIENT_DIR)/node_modules" ] || { printf '\033[31m✗ 前端依赖未安装\033[0m：请先执行 cd $(CLIENT_DIR) && npm install\n'; exit 1; }

.DEFAULT_GOAL := help
.PHONY: help dev backend frontend

help:
	@printf '\n\033[1m影迹（film-memo）—— 本地开发命令\033[0m\n\n'
	@printf '  make dev      同时启动 Go 后端 + 前端（Ctrl-C 一并退出）\n'
	@printf '  make backend  仅启动 Go 后端（%s/，localhost:4000）\n' '$(SERVER_DIR)'
	@printf '  make frontend 仅启动前端（%s/，localhost:5173）\n' '$(CLIENT_DIR)'
	@printf '\n  端口与数据目录沿用根目录 .env（DB_PATH / IMAGES_DIR）\n\n'

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
