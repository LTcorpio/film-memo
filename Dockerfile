# ===== Stage 1: 构建前端 =====
FROM node:20-bookworm-slim AS client-build
WORKDIR /app/client
COPY client/package*.json ./
RUN npm ci
COPY client/ ./
RUN npm run build

# ===== Stage 2: 构建后端依赖（better-sqlite3 需编译） =====
FROM node:20-bookworm-slim AS deps-build
WORKDIR /app
# better-sqlite3 是原生模块，需要 python3/make/g++ 编译
RUN apt-get update \
    && apt-get install -y --no-install-recommends python3 make g++ \
    && rm -rf /var/lib/apt/lists/*
COPY package*.json ./
RUN npm ci --omit=dev

# ===== Stage 3: 运行时 =====
FROM node:20-bookworm-slim AS runtime
WORKDIR /app
ENV NODE_ENV=production

# 复制已构建的后端依赖（含编译好的 better-sqlite3 二进制）
COPY --from=deps-build /app/node_modules ./node_modules
# 复制前端构建产物（由 server/index.js 静态托管）
COPY --from=client-build /app/client/dist ./client/dist
# 复制后端代码与脚本
COPY package.json ./
COPY server/ ./server/
COPY scripts/ ./scripts/

# 容器内数据目录（通过 volume 挂载到宿主机，便于持久化）
RUN mkdir -p /app/data/images

ENV PORT=8686
EXPOSE 8686

CMD ["node", "server/index.js"]
