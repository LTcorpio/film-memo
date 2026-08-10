# 影视观看记录 (Film Memo)

个人观影记录管理系统：支持 Excel 批量导入、TMDB 元数据自动刮削、海报本地化存储，提供海报网格与列表两种浏览模式，内置亮色/暗色/系统三种主题与只读模式。

## 技术栈

| 层 | 技术 |
|---|---|
| 前端 | React 18 + Vite 5 |
| 后端 | Express 4 (ESM) |
| 数据库 | SQLite (better-sqlite3) |
| 元数据 | [TMDB API](https://www.themoviedb.org/settings/api) |
| 数据导入 | ExcelJS |

## 项目结构

```
film-memo/
├── client/                 # 前端
│   ├── src/
│   │   ├── components/     # UI 组件（FilmCard / FilmDetail / MetaSearch ...）
│   │   ├── App.jsx         # 主应用
│   │   ├── api.js          # API 请求封装
│   │   └── styles.css      # 全局样式（CSS 变量主题系统）
│   ├── public/icon/        # 平台 LOGO（B站/爱奇艺/腾讯视频 ...）
│   └── vite.config.js      # Vite 配置（dev 代理 /api 和 /images → localhost:4000）
├── server/                 # 后端
│   ├── index.js            # Express 服务 + REST API
│   ├── db.js               # SQLite 连接与建表
│   └── tmdb.js             # TMDB API 客户端
├── scripts/
│   ├── import-excel.js     # Excel → SQLite 批量导入
│   └── scrape-metadata.js  # 按 IMDb 号批量刮削 TMDB 元数据
├── Dockerfile              # 多阶段构建（前端 build + 后端运行）
├── .env.example            # 环境变量模板
└── package.json
```

## 快速开始

### 环境要求

- Node.js >= 18
- npm

### 1. 克隆 & 安装依赖

```bash
git clone https://github.com/LTcorpio/film-memo.git
cd film-memo
npm install
cd client && npm install && cd ..
```

### 2. 配置环境变量

```bash
cp .env.example .env
```

编辑 `.env`，至少填写 TMDB 凭证（二选一）：

```env
# 推荐：v4 Read Access Token
TMDB_ACCESS_TOKEN=your_token_here
# 或：v3 API Key
TMDB_API_KEY=your_key_here
```

| 变量 | 说明 | 默认值 |
|---|---|---|
| `TMDB_ACCESS_TOKEN` | TMDB v4 Bearer Token（推荐） | — |
| `TMDB_API_KEY` | TMDB v3 API Key | — |
| `PORT` | 后端服务端口 | `4000` |
| `EXCEL_PATH` | Excel 数据源路径 | `~/Desktop/影视观看记录.xlsx` |
| `DB_PATH` | SQLite 数据库文件路径 | `data/films.db` |
| `IMAGES_DIR` | 本地图片存储目录 | `data/images/` |

### 3. 导入数据（可选）

如果有 Excel 格式的观影记录，可批量导入：

```bash
npm run import
```

### 4. 批量刮削元数据（可选）

为已导入但缺少元数据的影片批量刮削 TMDB 数据：

```bash
npm run scrape              # 仅处理缺少元数据的影片
npm run scrape -- --force   # 强制重新刮削所有
npm run scrape -- --id 6    # 仅处理指定 ID
```

### 5. 启动开发服务

```bash
npm run dev
```

该命令会同时启动：
- 后端 API 服务（`localhost:4000`）
- Vite 前端开发服务（`localhost:5173`，自动代理 `/api` 和 `/images` 到后端）

浏览器访问 http://localhost:5173

### 6. 生产部署

```bash
npm run build       # 构建前端静态文件到 client/dist/
npm start           # 启动后端，同时静态托管前端
```

访问 http://localhost:4000

## Docker 部署

```bash
# 确保 .env 中已配置 TMDB 凭证
docker compose up -d --build
```

- 服务端口：`8686`
- 数据持久化：`docker-data/db/`（数据库）、`docker-data/images/`（图片）
- 停止：`docker compose down`（数据保留）

## 功能概览

### 浏览与筛选
- **海报模式**：网格卡片展示，海报 + 标题 + 分类 + 年份 + 评分
- **列表模式**：紧凑表格，含 IMDb/豆瓣 ID 列、评分列
- **多维筛选**：观看年份 / 上映年份 / 观看平台 / 分类 / 缺失值（无 IMDb / 无豆瓣 ID）/ 名称搜索
- **分类卡片**：点击分类快速筛选，支持「无元数据」筛选

### 元数据管理
- **TMDB 刮削**：通过 IMDb 号搜索 TMDB，一键填充标题、简介、导演、演员、类型等
- **电视剧支持**：搜索时选择季（Season），支持「全剧总览」
- **图片本地化**：海报与背景图下载到本地，支持上传替换、重新下载、删除
- **批量刮削**：命令行脚本按 IMDb 号批量处理

### 详情页
- 海报 + 背景图 + 标题 + 原始标题 + 标签（分类/年份/时长/类型/评分）
- 简介展开/折叠
- 观看记录：年份 / 日期 / 集数 / 平台 / 地点 / IMDb / 豆瓣链接
- 备注字段保留换行格式

### 评分管理
- 批量维护 IMDb 与豆瓣 ID
- 评分数据源刷新

### 主题与模式
- **三主题**：亮色 / 暗色 / 跟随系统
- **只读模式**：右上角锁定切换器，开启后禁用所有增删改操作（新增、编辑、刮削、删除、右键菜单全部隐藏）

### API 端点

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/api/films` | 影片列表（支持筛选参数） |
| `GET` | `/api/films/:id` | 影片详情 |
| `GET` | `/api/filters` | 筛选项（年份/平台/分类） |
| `GET` | `/api/stats` | 统计数据 |
| `POST` | `/api/films` | 新增观影记录 |
| `PUT` | `/api/films/:id` | 编辑观影记录 |
| `DELETE` | `/api/films/:id` | 删除观影记录 |
| `GET` | `/api/meta/search` | 搜索 TMDB 元数据 |
| `GET` | `/api/meta/seasons` | 获取电视剧季列表 |
| `POST` | `/api/films/:id/metadata` | 保存元数据 |
| `PUT` | `/api/films/:id/metadata` | 编辑元数据 |
| `DELETE` | `/api/films/:id/metadata` | 删除元数据 |
| `POST` | `/api/films/:id/image` | 上传图片 |
| `POST` | `/api/films/:id/scrape-image` | 从 TMDB 下载图片 |
| `DELETE` | `/api/films/:id/image` | 删除本地图片 |
| `POST` | `/api/ratings/refresh` | 刷新评分数据源 |

## License

Private
