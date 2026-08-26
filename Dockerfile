# ===== Stage 1: 构建前端 =====
FROM node:20-bookworm-slim AS client-build
WORKDIR /app/client
COPY client/package*.json ./
RUN npm ci
COPY client/ ./
RUN npm run build

# ===== Stage 2: 构建后端（纯 Go，无 cgo，无需编译工具链） =====
FROM golang:1.26-bookworm AS go-build
WORKDIR /app/server-go
# 先拷依赖清单，利用层缓存
COPY server-go/go.mod server-go/go.sum ./
RUN go mod download
# 再拷源码编译
COPY server-go/ ./
# modernc.org/sqlite 为纯 Go，CGO_ENABLED=0 可静态编译，产物更小、无动态链接。
# -tags timetzdata 将时区数据库编入二进制，避免运行时镜像（alpine/debian 均缺 tzdata）再装包。
RUN CGO_ENABLED=0 go build -trimpath -tags timetzdata -ldflags="-s -w" -o /out/server ./cmd/server

# ===== Stage 3: 运行时（alpine，纯静态二进制，镜像更小、攻击面更小） =====
# 选用 alpine 的前提：后端为 CGO_ENABLED=0 静态编译，不依赖 glibc，
# 故 musl libc 完全兼容，无 cgo 兼容性风险。
FROM alpine:3.20 AS runtime
WORKDIR /app
ENV PORT=8686
ENV DB_PATH=/app/data/films.db
ENV IMAGES_DIR=/app/data/images

# CA 证书：alpine 官方镜像默认已含 ca-certificates（apk 自身依赖 HTTPS 拉包），
# 此处保留为防御性保险——若将来改用更精简的基础镜像，仍需显式安装，否则
# Go 用默认 http.Client 调 TMDB HTTPS 会因 x509: unknown authority 失败，刮削无结果。
RUN apk add --no-cache ca-certificates

# 后端二进制（自包含，无需任何运行时依赖）
COPY --from=go-build /out/server /app/server
# 前端构建产物（由 Go 后端静态托管 + SPA 兜底）
COPY --from=client-build /app/client/dist /app/client/dist

# 容器内数据目录（通过 volume 挂载到宿主机，便于持久化）
RUN mkdir -p /app/data/images

EXPOSE 8686
CMD ["/app/server"]
