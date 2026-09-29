package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"film-memo/internal/db"
	"film-memo/internal/imdb"
	"film-memo/internal/model"
	"film-memo/internal/tmdb"
)

// --- 输出/输入辅助类型 ---

// searchOut 是 /api/meta/search 的单条结果（追加 posterUrl）。
type searchOut struct {
	tmdb.SearchResult
	PosterURL *string `json:"posterUrl"`
}

// seasonOut 是 /api/meta/seasons 的单条结果（追加 posterUrl/year）。
type seasonOut struct {
	tmdb.Season
	PosterURL *string `json:"posterUrl"`
	Year      *string `json:"year"`
}

// saveMetaBody 是 POST /api/films/:id/metadata 的请求体。
type saveMetaBody struct {
	TmdbID    int64       `json:"tmdbId"`
	MediaType string      `json:"mediaType"`
	Season    interface{} `json:"season"`
}

// sourceSummary 是单个评分数据源本次拉取的统计。
type sourceSummary struct {
	Source       string                   `json:"source"`
	Total        int64                    `json:"total"`   // 该源覆盖的影片数
	Updated      int64                    `json:"updated"` // 成功写入数
	Skipped      int64                    `json:"skipped"` // 跳过数（数据源无此条目）
	Failed       int64                    `json:"failed"`  // 失败数
	LastSyncedAt *string                  `json:"lastSyncedAt"`
	Message      *string                  `json:"message"`
	Errors       []map[string]interface{} `json:"errors"`
}

// refreshSummary 是 POST /api/ratings/refresh 的响应（按数据源分列）。
type refreshSummary struct {
	Total   int64           `json:"total"` // 本次涉及的影片数
	Sources []sourceSummary `json:"sources"`
}

// maxReportedErrors 单个数据源最多回传的错误明细条数。
const maxReportedErrors = 10

// sourceStat 累计单个数据源本次拉取的统计。
type sourceStat struct {
	source  string
	total   int64
	updated int64
	skipped int64
	failed  int64
	message *string
	errors  []map[string]interface{}
}

func newSourceStat(source string) *sourceStat {
	return &sourceStat{source: source, errors: []map[string]interface{}{}}
}

// fail 记录一条失败明细（超出上限后仅计数）。
func (s *sourceStat) fail(filmID int64, name string, err error) {
	s.failed++
	if len(s.errors) < maxReportedErrors {
		s.errors = append(s.errors, map[string]interface{}{"id": filmID, "name": name, "error": err.Error()})
	}
}

func (s *sourceStat) out(lastSyncedAt string) sourceSummary {
	return sourceSummary{
		Source: s.source, Total: s.total, Updated: s.updated, Skipped: s.skipped, Failed: s.failed,
		LastSyncedAt: &lastSyncedAt, Message: s.message, Errors: s.errors,
	}
}

func (s *sourceStat) toSync(lastSyncedAt string) db.RatingSync {
	return db.RatingSync{
		Source: s.source, LastSyncedAt: &lastSyncedAt, Total: s.total,
		Updated: s.updated, Skipped: s.skipped, Failed: s.failed, Message: s.message,
	}
}

// --- 影片列表/详情/增删改 ---

// handleListFilms GET /api/films：每条观看记录一条目（附带影视与元数据信息）。
func (s *Server) handleListFilms(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := db.Filter{
		WatchYear:   parseYear(q.Get("watchYear")),
		ReleaseYear: parseYear(q.Get("releaseYear")),
		Platform:    q.Get("platform"),
		Category:    q.Get("category"),
		Q:           q.Get("q"),
		Missing:     q.Get("missing"),
	}
	rows, err := s.db.ListFilms(f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]model.Entry, 0, len(rows))
	for i := range rows {
		out = append(out, model.ShapeEntry(&rows[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGetFilm GET /api/films/:id：影视详情（含全部观看记录）。
func (s *Server) handleGetFilm(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	row, err := s.db.GetFilm(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if row == nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	viewings, err := s.db.ListViewingsByFilm(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model.ShapeFilm(row, viewings))
}

// handleCreateFilm POST /api/films：按 豆瓣>IMDb>名称 匹配已有影视并追加观看记录，否则新建影视。
func (s *Server) handleCreateFilm(w http.ResponseWriter, r *http.Request) {
	body := map[string]interface{}{}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	nameStr := ""
	if v, ok := body["name"]; ok {
		if s2, ok2 := v.(string); ok2 {
			nameStr = s2
		}
	}
	if strings.TrimSpace(nameStr) == "" {
		writeError(w, http.StatusBadRequest, "名称不能为空")
		return
	}
	newID, err := s.db.CreateFilm(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	row, err := s.db.GetFilm(newID)
	if err != nil || row == nil {
		writeError(w, http.StatusInternalServerError, "load failed")
		return
	}
	viewings, err := s.db.ListViewingsByFilm(newID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load failed")
		return
	}
	writeJSON(w, http.StatusOK, model.ShapeFilm(row, viewings))
}

// handleUpdateFilm PUT /api/films/:id：编辑影视信息（films 表字段）。
func (s *Server) handleUpdateFilm(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	exists, err := s.db.FilmExists(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "film not found")
		return
	}
	body := map[string]interface{}{}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	updated, err := s.db.UpdateFilm(id, body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "updated": updated})
}

// handleUpdateViewing PUT /api/viewings/:id：编辑单条观看记录。
func (s *Server) handleUpdateViewing(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	exists, err := s.db.ViewingExists(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "viewing not found")
		return
	}
	body := map[string]interface{}{}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	updated, err := s.db.UpdateViewing(id, body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "updated": updated})
}

// handleDeleteViewing DELETE /api/viewings/:id：删除单条观看记录；
// 若为该影视最后一条，则连同影视与元数据（含本地图片）一并删除。
func (s *Server) handleDeleteViewing(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	found, filmDeleted, locals, err := s.db.DeleteViewing(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "viewing not found")
		return
	}
	if filmDeleted {
		if locals.Poster != nil {
			s.images.Remove(*locals.Poster)
		}
		if locals.Backdrop != nil {
			s.images.Remove(*locals.Backdrop)
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "filmDeleted": filmDeleted})
}

// handleDeleteFilm DELETE /api/films/:id：删除整部影视（含全部观看记录与元数据）。
func (s *Server) handleDeleteFilm(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	exists, err := s.db.FilmExists(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "film not found")
		return
	}
	locals, err := s.db.DeleteFilm(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if locals.Poster != nil {
		s.images.Remove(*locals.Poster)
	}
	if locals.Backdrop != nil {
		s.images.Remove(*locals.Backdrop)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// --- 筛选与统计 ---

// handleFilters GET /api/filters
func (s *Server) handleFilters(w http.ResponseWriter, r *http.Request) {
	f, err := s.db.Filters()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// handleStats GET /api/stats
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	st, err := s.db.Stats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// --- 元数据搜索与季列表 ---

// handleMetaSearch GET /api/meta/search
func (s *Server) handleMetaSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"configured": s.tmdb.Configured(),
			"results":    []searchOut{},
		})
		return
	}
	if !s.tmdb.Configured() {
		writeError(w, http.StatusBadRequest, "TMDB 未配置，请在 .env 设置 TMDB_ACCESS_TOKEN 或 TMDB_API_KEY")
		return
	}
	results, err := s.tmdb.SearchByName(r.Context(), q, r.URL.Query().Get("category"))
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	out := make([]searchOut, 0, len(results))
	for _, r2 := range results {
		pp := r2.PosterPath
		out = append(out, searchOut{SearchResult: r2, PosterURL: model.ImageURL(&pp, "w185")})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"configured": true, "results": out})
}

// handleMetaSeasons GET /api/meta/seasons
func (s *Server) handleMetaSeasons(w http.ResponseWriter, r *http.Request) {
	tmdbID, err := strconv.ParseInt(r.URL.Query().Get("tmdbId"), 10, 64)
	if err != nil || tmdbID == 0 {
		writeError(w, http.StatusBadRequest, "需要 tmdbId")
		return
	}
	if !s.tmdb.Configured() {
		writeError(w, http.StatusBadRequest, "TMDB 未配置")
		return
	}
	seasons, err := s.tmdb.GetSeasons(r.Context(), tmdbID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	out := make([]seasonOut, 0, len(seasons))
	for _, s2 := range seasons {
		pp := s2.PosterPath
		var year *string
		if len(s2.AirDate) >= 4 {
			y := s2.AirDate[:4]
			year = &y
		}
		out = append(out, seasonOut{Season: s2, PosterURL: model.ImageURL(&pp, "w185"), Year: year})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"configured": true, "seasons": out})
}

// --- 元数据写入 ---

// handleSaveMetadata POST /api/films/:id/metadata
func (s *Server) handleSaveMetadata(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	film, err := s.db.GetFilmBasic(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if film == nil {
		writeError(w, http.StatusNotFound, "film not found")
		return
	}
	if !s.tmdb.Configured() {
		writeError(w, http.StatusBadRequest, "TMDB 未配置")
		return
	}
	var body saveMetaBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if body.TmdbID == 0 || body.MediaType == "" {
		writeError(w, http.StatusBadRequest, "需要 tmdbId 与 mediaType")
		return
	}

	ctx := r.Context()
	details, err := s.tmdb.GetDetails(ctx, body.TmdbID, body.MediaType)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if details == nil {
		writeError(w, http.StatusNotFound, "TMDB 无详情")
		return
	}

	// TV 剧集：若指定了具体季，拉取该季详情覆盖 details 对应字段
	if body.MediaType == "tv" {
		if n, ok := seasonToNumber(body.Season); ok {
			sd, err := s.tmdb.GetSeasonDetails(ctx, body.TmdbID, n)
			if err == nil && sd != nil {
				details.Credits = sd.Credits
				if sd.PosterPath != "" {
					details.PosterPath = sd.PosterPath
				}
				if sd.AirDate != "" {
					details.FirstAirDate = sd.AirDate
				}
				if strings.TrimSpace(sd.Overview) != "" {
					details.Overview = sd.Overview
				}
				if len(sd.Episodes) > 0 {
					eps := int64(len(sd.Episodes))
					details.NumberOfEpisodes = &eps
				}
			}
		}
	}

	meta := tmdb.NormalizeDetails(details, body.MediaType)
	if meta == nil {
		writeError(w, http.StatusNotFound, "TMDB 无详情")
		return
	}

	existing, err := s.db.GetExistingLocals(film.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	newPoster := s.images.DownloadTmdb(ctx, meta.PosterPath, fmt.Sprintf("%d-poster", film.ID), true)
	newBackdrop := s.images.DownloadTmdb(ctx, meta.BackdropPath, fmt.Sprintf("%d-backdrop", film.ID), true)
	if newPoster != nil && existing.Poster != nil && *existing.Poster != *newPoster {
		s.images.Remove(*existing.Poster)
	}
	if newBackdrop != nil && existing.Backdrop != nil && *existing.Backdrop != *newBackdrop {
		s.images.Remove(*existing.Backdrop)
	}
	posterLocal := newPoster
	if posterLocal == nil {
		posterLocal = existing.Poster
	}
	backdropLocal := newBackdrop
	if backdropLocal == nil {
		backdropLocal = existing.Backdrop
	}

	if err := s.db.UpsertMetadata(film.ID, meta, posterLocal, backdropLocal); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if film.ImdbID == "" && meta.ImdbID != "" {
		_ = s.db.UpdateImdbID(film.ID, meta.ImdbID)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":            true,
		"tmdbId":        meta.TmdbID,
		"imdbId":        meta.ImdbID,
		"posterLocal":   posterLocal,
		"backdropLocal": backdropLocal,
	})
}

// handleUpdateMetadata PUT /api/films/:id/metadata
func (s *Server) handleUpdateMetadata(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	exists, err := s.db.FilmExists(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "film not found")
		return
	}
	body := map[string]interface{}{}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	updated, err := s.db.UpdateMetadataFields(id, body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "updated": updated})
}

// handleDeleteMetadata DELETE /api/films/:id/metadata
func (s *Server) handleDeleteMetadata(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	locals, err := s.db.DeleteMetadata(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if locals.Poster != nil {
		s.images.Remove(*locals.Poster)
	}
	if locals.Backdrop != nil {
		s.images.Remove(*locals.Backdrop)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// --- 图片上传/刮削/删除 ---

// handleUploadImage POST /api/films/:id/image（raw body: image/*）
func (s *Server) handleUploadImage(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	exists, err := s.db.FilmExists(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "film not found")
		return
	}
	typ := imgType(r)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "空图片数据")
		return
	}
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/jpeg"
	}
	ext := extFromContentType(ct)

	old, _ := s.db.GetMetaLocal(id, typ)
	if old != nil {
		s.images.Remove(*old)
	}
	file := fmt.Sprintf("%d-%s-%d%s", id, typ, time.Now().UnixMilli(), ext)
	if err := s.images.SaveUpload(file, data); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.db.SetImage(id, typ, file); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "file": file, "url": "/images/" + file})
}

// handleScrapeImage POST /api/films/:id/scrape-image
func (s *Server) handleScrapeImage(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	exists, err := s.db.FilmExists(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "film not found")
		return
	}
	typ := imgType(r)
	paths, err := s.db.GetMetaPaths(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var path *string
	if typ == "backdrop" {
		path = paths.BackdropPath
	} else {
		path = paths.PosterPath
	}
	if path == nil {
		writeError(w, http.StatusBadRequest, "TMDB 无对应图片路径，请先刮削元数据")
		return
	}
	old, _ := s.db.GetMetaLocal(id, typ)
	if old != nil {
		s.images.Remove(*old)
	}
	file := s.images.DownloadTmdb(r.Context(), *path, fmt.Sprintf("%d-%s", id, typ), true)
	if file == nil {
		writeError(w, http.StatusBadGateway, "图片下载失败")
		return
	}
	if err := s.db.SetImage(id, typ, *file); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "file": *file, "url": "/images/" + *file})
}

// handleDeleteImage DELETE /api/films/:id/image
func (s *Server) handleDeleteImage(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	exists, err := s.db.FilmExists(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !exists {
		writeError(w, http.StatusNotFound, "film not found")
		return
	}
	typ := imgType(r)
	old, _ := s.db.GetMetaLocal(id, typ)
	if old != nil {
		s.images.Remove(*old)
	}
	if err := s.db.ClearImageLocal(id, typ); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// --- 评分刷新 ---

// doubanTVRe 判断类别是否应按电视剧取豆瓣评分（与 tmdb 包的 preferTV 口径一致）。
var doubanTVRe = regexp.MustCompile(`剧|综艺|动漫|纪录|动画`)

// doubanMediaType 按类别推断豆瓣接口路径；推断错误时客户端会在 404 后自动换类型重试。
func doubanMediaType(category string) string {
	if doubanTVRe.MatchString(category) {
		return "tv"
	}
	return "movie"
}

// filmRatingResult 是单部影片本次拉取的中间结果。
type filmRatingResult struct {
	ref           *db.FilmRef
	set           db.RatingSet
	imdbOutcome   string // updated / skipped / failed / ""（无 imdb_id）
	imdbErr       error
	doubanOutcome string
	doubanErr     error
}

// handleRefreshRatings POST /api/ratings/refresh
// 启动后台刷新任务并立即返回；实际抓取由 runRatingsJob 在 goroutine 中完成，
// 前端通过 GET /api/ratings/refresh/progress 轮询进度。
// 已有任务运行中时不新建任务（started=false），前端继续轮询既有进度即可。
func (s *Server) handleRefreshRatings(w http.ResponseWriter, r *http.Request) {
	// 合并 query + body（对应 JS {...req.query, ...req.body}）
	merged := map[string]interface{}{}
	for k, vs := range r.URL.Query() {
		if len(vs) > 0 {
			merged[k] = vs[0]
		}
	}
	if r.Body != nil {
		body := map[string]interface{}{}
		if err := decodeJSON(r, &body); err == nil {
			for k, v := range body {
				merged[k] = v
			}
		}
	}
	source, _ := merged["source"].(string)
	job, started := s.jobs.start(source, nowISO())
	if !started {
		// 已有任务运行中：不新建，前端继续轮询既有进度
		writeJSON(w, http.StatusOK, map[string]interface{}{"started": false, "running": true})
		return
	}
	// 后台任务不能复用 r.Context()：handler 一返回该 context 就会被取消
	ctx, cancel := context.WithCancel(context.Background())
	job.bindCancel(cancel)
	go s.runRatingsJob(ctx, buildFilterFromMap(merged), source, job)

	writeJSON(w, http.StatusOK, map[string]interface{}{"started": true, "running": true})
}

// runRatingsJob 执行实际的评分拉取：两个数据源互不阻塞，
// IMDb 走官方数据集本地索引（一次下载/校验，随后流式查询），豆瓣走移动端接口（限速串行）。
// 过程中更新 job 的阶段与进度，结果连同本次拉取时间写入 rating_sync 供前端展示「上次更新」。
func (s *Server) runRatingsJob(ctx context.Context, f db.Filter, source string, job *ratingJob) {
	rows, err := s.db.ListFilmRefsForRatings(f)
	if err != nil {
		msg := err.Error()
		job.finish(nil, nil, &msg, nowISO())
		return
	}

	doubanStat := newSourceStat("douban")
	imdbStat := newSourceStat("imdb")
	// 可选按来源刷新：source=douban / imdb 时只跑对应数据源，未跑的数据源不改动其同步记录
	runDouban, runImdb := source != "imdb", source != "douban"
	job.setTotal(progressUnits(rows, runDouban, runImdb))

	// IMDb：先确保数据集可用，再一次流式查询本次所需的全部 tconst。
	// 只收集「缺评分或缺评价人数」的影片，已有数据的条目不再重复抓取。
	imdbIDs := []string{}
	for i := range rows {
		if rows[i].ImdbID != "" && rows[i].ImdbNeed {
			imdbIDs = append(imdbIDs, rows[i].ImdbID)
		}
		if rows[i].DoubanID != "" && rows[i].DoubanNeed {
			doubanStat.total++
		}
	}
	imdbIndex := map[string]imdb.Rating{}
	var imdbErr error
	if len(imdbIDs) > 0 && runImdb {
		job.setPhase(phaseImdb)
		imdbStat.total = int64(len(imdbIDs))
		if meta, err := s.imdb.Ensure(ctx); err != nil {
			imdbErr = err
		} else {
			if meta.Offline {
				imdbStat.message = ptrString("网络不可用，本次使用本地缓存数据集")
			}
			if idx, err := s.imdb.LookupAll(ctx, imdbIDs); err != nil {
				imdbErr = err
			} else {
				imdbIndex = idx
			}
		}
	}
	if imdbErr != nil {
		imdbStat.message = ptrString(imdbErr.Error())
	}
	if len(imdbIDs) > 0 && runImdb {
		job.advance()
	}

	// 拉取阶段：逐部影片取两个来源的评分（豆瓣请求在客户端内部串行限速）
	job.setPhase(phaseDouban)
	results := make([]filmRatingResult, 0, len(rows))
	for i := range rows {
		if ctx.Err() != nil {
			break // 服务停机/任务取消：停止抓取，已取到的结果照常落库
		}
		rr := rows[i]
		res := filmRatingResult{ref: &rr}

		if rr.ImdbID != "" && rr.ImdbNeed {
			switch {
			case imdbErr != nil:
				res.imdbOutcome, res.imdbErr = "failed", imdbErr
			default:
				if rt, ok := imdbIndex[rr.ImdbID]; ok {
					v, c := rt.Average, rt.Votes
					res.set.ImdbRating, res.set.ImdbVotes = &v, &c
					res.imdbOutcome = "updated"
				} else {
					res.imdbOutcome = "skipped"
				}
			}
		}
		if rr.DoubanID != "" && runDouban && rr.DoubanNeed {
			rt, err := s.douban.GetRating(ctx, rr.DoubanID, doubanMediaType(rr.Category))
			switch {
			case err != nil:
				res.doubanOutcome, res.doubanErr = "failed", err
			case rt.Value <= 0:
				// 条目不存在或尚无评分：跳过，避免把 0 分写入库
				res.doubanOutcome = "skipped"
			default:
				v, c := rt.Value, rt.Count
				res.set.DoubanRating, res.set.DoubanVotes = &v, &c
				res.doubanOutcome = "updated"
			}
			job.advance() // 每完成一部影片的豆瓣抓取推进一格进度
		}
		results = append(results, res)
	}

	// 写库阶段：单条写入失败时，把该条已取到的来源改判为失败
	job.setPhase(phaseWrite)
	now := nowISO()
	for i := range results {
		res := &results[i]
		if res.set.Empty() {
			continue
		}
		if err := s.db.UpdateRatings(res.ref.ID, res.set, now); err != nil {
			if res.imdbOutcome == "updated" {
				res.imdbOutcome, res.imdbErr = "failed", err
			}
			if res.doubanOutcome == "updated" {
				res.doubanOutcome, res.doubanErr = "failed", err
			}
		}
	}

	// 统计阶段
	for i := range results {
		res := &results[i]
		switch res.imdbOutcome {
		case "updated":
			imdbStat.updated++
		case "skipped":
			imdbStat.skipped++
		case "failed":
			imdbStat.fail(res.ref.ID, res.ref.Name, res.imdbErr)
		}
		switch res.doubanOutcome {
		case "updated":
			doubanStat.updated++
		case "skipped":
			doubanStat.skipped++
		case "failed":
			doubanStat.fail(res.ref.ID, res.ref.Name, res.doubanErr)
		}
	}

	// 只落盘与返回本次实际运行的数据源，避免把未运行来源的统计写成 0
	ran := []*sourceStat{}
	if runDouban {
		ran = append(ran, doubanStat)
	}
	if runImdb {
		ran = append(ran, imdbStat)
	}
	sources := make([]sourceSummary, 0, len(ran))
	for _, st := range ran {
		if err := s.db.UpsertRatingSync(st.toSync(now)); err != nil {
			msg := err.Error()
			job.finish(nil, ran, &msg, nowISO())
			return
		}
		sources = append(sources, st.out(now))
	}

	// 本次实际涉及的影片数：至少有一个运行中的来源判定为缺失
	targets := int64(0)
	for i := range rows {
		if (runDouban && rows[i].DoubanID != "" && rows[i].DoubanNeed) ||
			(runImdb && rows[i].ImdbID != "" && rows[i].ImdbNeed) {
			targets++
		}
	}
	job.finish(&refreshSummary{
		Total:   targets,
		Sources: sources,
	}, ran, nil, nowISO())
}

// handleRatingProgress GET /api/ratings/refresh/progress
// 返回当前（或最近一次）评分刷新任务的进度快照；从未跑过任务时返回 running=false。
func (s *Server) handleRatingProgress(w http.ResponseWriter, r *http.Request) {
	j := s.jobs.current()
	if j == nil {
		writeJSON(w, http.StatusOK, jobProgress{Running: false})
		return
	}
	writeJSON(w, http.StatusOK, j.snapshot())
}

// handleStopRatings POST /api/ratings/refresh/stop
// 停止进行中的评分刷新任务：在当前影片抓取结束后退出循环，已取到的结果照常落库。
// 没有运行中的任务时返回 stopped=false。
func (s *Server) handleStopRatings(w http.ResponseWriter, r *http.Request) {
	j := s.jobs.current()
	if j == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"stopped": false, "running": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"stopped": j.cancelIfRunning(), "running": true})
}

// StopJobs 取消进行中的评分刷新任务并等待其收尾（停机时调用，
// 避免后台任务向随后关闭的数据库继续写入）。
func (s *Server) StopJobs() {
	j := s.jobs.current()
	if j == nil {
		return
	}
	if j.cancelIfRunning() {
		j.wait(3 * time.Second)
	}
}

// handleRatingsStatus GET /api/ratings/status
// 返回各评分数据源最近一次拉取的时间与结果，以及 IMDb 数据集本地缓存状态。
func (s *Server) handleRatingsStatus(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.ListRatingSync()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sources":     rows,
		"imdbDataset": s.imdb.Cache(),
	})
}
