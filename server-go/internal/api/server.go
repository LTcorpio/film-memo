// Package api 实现 HTTP 路由、中间件与全部业务 handler。
// 对应 JS 版 index.js 的 Express 应用与所有 REST 端点。
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"film-memo/internal/config"
	"film-memo/internal/db"
	"film-memo/internal/image"
	"film-memo/internal/tmdb"
)

// Server 聚合后端运行所需依赖。
type Server struct {
	db         *db.DB
	tmdb       *tmdb.Client
	images     *image.Store
	clientDist string
}

// New 创建 Server。若 client/dist 存在则启用 SPA 静态托管。
func New(cfg config.Config, d *db.DB, tc *tmdb.Client, imgs *image.Store) *Server {
	s := &Server{db: d, tmdb: tc, images: imgs}
	dist := filepath.Join(cfg.ProjectRoot, "client", "dist")
	if info, err := os.Stat(dist); err == nil && info.IsDir() {
		s.clientDist = dist
	}
	return s
}

// Handler 返回已注册路由并套用 CORS 中间件的 http.Handler。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// 影片 CRUD
	mux.HandleFunc("GET /api/films", s.handleListFilms)
	mux.HandleFunc("POST /api/films", s.handleCreateFilm)
	mux.HandleFunc("GET /api/films/{id}", s.handleGetFilm)
	mux.HandleFunc("PUT /api/films/{id}", s.handleUpdateFilm)
	mux.HandleFunc("DELETE /api/films/{id}", s.handleDeleteFilm)

	// 筛选与统计
	mux.HandleFunc("GET /api/filters", s.handleFilters)
	mux.HandleFunc("GET /api/stats", s.handleStats)

	// 元数据
	mux.HandleFunc("GET /api/meta/search", s.handleMetaSearch)
	mux.HandleFunc("GET /api/meta/seasons", s.handleMetaSeasons)
	mux.HandleFunc("POST /api/films/{id}/metadata", s.handleSaveMetadata)
	mux.HandleFunc("PUT /api/films/{id}/metadata", s.handleUpdateMetadata)
	mux.HandleFunc("DELETE /api/films/{id}/metadata", s.handleDeleteMetadata)

	// 图片
	mux.HandleFunc("POST /api/films/{id}/image", s.handleUploadImage)
	mux.HandleFunc("POST /api/films/{id}/scrape-image", s.handleScrapeImage)
	mux.HandleFunc("DELETE /api/films/{id}/image", s.handleDeleteImage)

	// 评分刷新
	mux.HandleFunc("POST /api/ratings/refresh", s.handleRefreshRatings)

	// 本地图片静态托管（maxAge 7d，immutable）
	if s.images != nil {
		fs := http.FileServer(http.Dir(s.images.Dir()))
		mux.Handle("GET /images/", s.cacheImages(http.StripPrefix("/images", fs)))
	}

	// 生产环境 SPA 兜底（对应 express.static + 非 api/images → index.html）
	mux.HandleFunc("GET /", s.handleSPA)

	return s.cors(mux)
}

// cors 允许跨域（对应 app.use(cors())）。
func (s *Server) cors(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

// cacheImages 为 /images 静态资源设置长缓存（对应 express static maxAge 7d immutable）。
func (s *Server) cacheImages(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
		h.ServeHTTP(w, r)
	})
}

// handleSPA 托管前端构建产物，未命中文件时回退 index.html。
func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	if s.clientDist == "" {
		http.NotFound(w, r)
		return
	}
	up := r.URL.Path
	if up == "/" {
		up = "/index.html"
	}
	clean := filepath.Clean("/" + up)
	p := filepath.Join(s.clientDist, clean)
	// 防路径穿越
	if rel, err := filepath.Rel(s.clientDist, p); err == nil && !strings.HasPrefix(rel, "..") {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			http.ServeFile(w, r, p)
			return
		}
	}
	http.ServeFile(w, r, filepath.Join(s.clientDist, "index.html"))
}

// --- 通用助手 ---

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// decodeJSON 解析请求体 JSON，限制 12MB（对应 express.json limit 12mb）。
func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(io.LimitReader(r.Body, 12*1024*1024)).Decode(v)
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

// imgType 从 query.type 解析图片类型，默认 poster（对应各 image handler）。
func imgType(r *http.Request) string {
	if r.URL.Query().Get("type") == "backdrop" {
		return "backdrop"
	}
	return "poster"
}

func parseYear(s string) *int64 {
	if s == "" {
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

// toInt64Ptr 把任意值转为 *int64（用于 ratings refresh 合并 query+body）。
func toInt64Ptr(v interface{}) *int64 {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case string:
		if x == "" {
			return nil
		}
		n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		if err != nil {
			return nil
		}
		return &n
	case float64:
		n := int64(x)
		return &n
	case int64:
		return &x
	case int:
		n := int64(x)
		return &n
	}
	return nil
}

func toStr(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

func ptrString(s string) *string { return &s }

// extFromContentType 把 Content-Type 映射为扩展名（对应 image handler 的 ext 映射）。
func extFromContentType(ct string) string {
	switch strings.ToLower(strings.TrimSpace(ct)) {
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	}
	return ".jpg"
}

// seasonToNumber 把 season 参数转为季号；nil/空字符串返回 false（对应 JS season != null && season !== ''）。
func seasonToNumber(v interface{}) (int, bool) {
	switch x := v.(type) {
	case nil:
		return 0, false
	case string:
		if x == "" {
			return 0, false
		}
		n, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return 0, false
		}
		return n, true
	case float64:
		return int(x), true
	}
	return 0, false
}

// buildFilterFromMap 从合并后的 query+body 构造筛选条件（ratings refresh 用）。
func buildFilterFromMap(m map[string]interface{}) db.Filter {
	f := db.Filter{}
	if v, ok := m["watchYear"]; ok {
		f.WatchYear = toInt64Ptr(v)
	}
	if v, ok := m["releaseYear"]; ok {
		f.ReleaseYear = toInt64Ptr(v)
	}
	if v, ok := m["platform"]; ok {
		f.Platform = toStr(v)
	}
	if v, ok := m["category"]; ok {
		f.Category = toStr(v)
	}
	if v, ok := m["q"]; ok {
		f.Q = toStr(v)
	}
	if v, ok := m["missing"]; ok {
		f.Missing = toStr(v)
	}
	return f
}
