# film-memo 后端（Go 版）

个人观影记录系统的 Go 语言后端实现，与原 Node.js（Express + better-sqlite3）版本**功能、API 契约、数据库结构完全一致**，仅替换技术栈。前端 `client/` 无需任何改动。

## 设计要点

- **纯 Go，无 cgo**：SQLite 驱动使用 [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite)（纯 Go 实现），数据库文件与 better-sqlite3 创建的 `.db` 完全兼容，可直接复用现有数据。
- **零 Web 框架**：HTTP 路由使用 Go 1.22+ 标准库 `net/http` 新版 `ServeMux`（支持方法 + 路径参数），CORS / JSON / 静态托管均为手写中间件。
- **Excel 读取**：[`github.com/xuri/excelize/v2`](https://github.com/qax-os/excelize)（纯 Go），对应原 `exceljs`。
- **中文排序**：`golang.org/x/text/collate`（`SimplifiedChinese`），对应原 `localeCompare('zh-Hans')`。

## 目录结构

```
server-go/
├── cmd/
│   ├── server/         # HTTP 服务入口（对应 node server/index.js）
│   ├── import/         # Excel → SQLite 导入（对应 npm run import）
│   └── scrape/         # TMDB 批量刮削（对应 npm run scrape）
├── internal/
│   ├── config/         # 环境变量 / 项目根定位 / .env 加载
│   ├── db/             # 连接、建表、旧库迁移、全部数据访问
│   ├── tmdb/           # TMDB API 客户端 + normalizeDetails 归一化
│   ├── model/          # FilmRow 扫描结构 / Film 输出结构 / ShapeFilm
│   ├── image/          # 本地图片下载 / 保存 / 删除
│   ├── api/            # 路由、中间件、17 个 handler
│   └── scripts/        # Excel 导入与批量刮削逻辑
├── go.mod / go.sum
├── Dockerfile          # Go 版多阶段构建
└── README.md
```

## 环境变量

与原版完全一致，读取项目根 `.env`（自动定位 `server-go/` 的上级目录）：

| 变量 | 说明 | 默认值 |
|---|---|---|
| `TMDB_ACCESS_TOKEN` | TMDB v4 Bearer Token（推荐） | — |
| `TMDB_API_KEY` | TMDB v3 API Key | — |
| `PORT` | 服务端口 | `4000` |
| `EXCEL_PATH` | Excel 数据源路径 | `~/Desktop/影视观看记录.xlsx` |
| `DB_PATH` | SQLite 数据库路径 | `data/films.db` |
| `IMAGES_DIR` | 本地图片目录 | `data/images/` |

## 构建与运行

```bash
cd server-go

# 构建（产出 bin/server、bin/import、bin/scrape）
go build -o bin/server ./cmd/server
go build -o bin/import ./cmd/import
go build -o bin/scrape ./cmd/scrape

# 启动 HTTP 服务
./bin/server
# 等价于原 npm start / npm run server

# Excel 导入
./bin/import
# 等价于原 npm run import

# TMDB 批量刮削
./bin/scrape              # 仅处理缺少元数据的影片
./bin/scrape --force      # 强制重新刮削
./bin/scrape --id 6       # 仅处理指定 id
# 等价于原 npm run scrape [-- --force | --id N]
```

开发联调（前端 Vite + Go 后端）：

```bash
# 终端 1：Go 后端
cd server-go && ./bin/server
# 终端 2：前端（vite.config.js 已代理 /api 与 /images 到 localhost:4000）
cd client && npm run dev
```

API 端点清单见项目根 `README.md`，全部 17 个端点行为与原版一致。

## Docker

项目根 `Dockerfile` 已适配 Go 后端（前端 build → Go 编译静态二进制 → 精简运行时三阶段）：

```bash
docker build -t film-memo-go .
docker run -d -p 8686:8686 \
  -v "$PWD/docker-data/db:/app/data" \
  -v "$PWD/docker-data/images:/app/data/images" \
  --env-file .env \
  -e DB_PATH=/app/data/films.db \
  -e IMAGES_DIR=/app/data/images \
  film-memo-go
```

Go 版镜像运行时仅含静态二进制 + 前端构建产物，无需 Node.js 运行时，体积更小、构建更简单（无原生模块编译）。

## 与原 Node 版的关系

本目录与原 `server/`（Node 版）**并存**，互不影响。两者读写同一份 SQLite 数据库与图片目录，可随时切换启用哪一版：

- 用 Go 版：`cd server-go && ./bin/server`
- 用 Node 版：`npm start`

迁移建议：先用 Go 版连接现有数据库进行只读验证（浏览 / 筛选 / 统计），确认无误后再用于写入操作。
