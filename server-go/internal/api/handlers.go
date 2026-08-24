package api

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"film-memo/internal/db"
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

// refreshSummary 是 POST /api/ratings/refresh 的响应。
type refreshSummary struct {
	Total   int64                    `json:"total"`
	Updated int64                    `json:"updated"`
	Skipped int64                    `json:"skipped"`
	Failed  int64                    `json:"failed"`
	Errors  []map[string]interface{} `json:"errors"`
}

// --- 影片列表/详情/增删改 ---

// handleListFilms GET /api/films
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
	out := make([]model.Film, 0, len(rows))
	for i := range rows {
		out = append(out, model.ShapeFilm(&rows[i]))
	}
	writeJSON(w, http.StatusOK, out)
}

// handleGetFilm GET /api/films/:id
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
	writeJSON(w, http.StatusOK, model.ShapeFilm(row))
}

// handleCreateFilm POST /api/films
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
	newID, err := s.db.InsertFilm(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	row, err := s.db.GetFilm(newID)
	if err != nil || row == nil {
		writeError(w, http.StatusInternalServerError, "load failed")
		return
	}
	writeJSON(w, http.StatusOK, model.ShapeFilm(row))
}

// handleUpdateFilm PUT /api/films/:id
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

// handleDeleteFilm DELETE /api/films/:id
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
		"ok":           true,
		"tmdbId":       meta.TmdbID,
		"imdbId":       meta.ImdbID,
		"posterLocal":  posterLocal,
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

// handleRefreshRatings POST /api/ratings/refresh
func (s *Server) handleRefreshRatings(w http.ResponseWriter, r *http.Request) {
	if !s.tmdb.Configured() {
		writeError(w, http.StatusBadRequest, "TMDB 未配置")
		return
	}
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
	f := buildFilterFromMap(merged)
	rows, err := s.db.ListFilmRefsForRatings(f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	summary := refreshSummary{Total: int64(len(rows)), Errors: []map[string]interface{}{}}
	ctx := r.Context()
	for _, rr := range rows {
		if rr.TmdbID == nil || rr.MediaType == "" {
			summary.Skipped++
			continue
		}
		details, err := s.tmdb.GetDetails(ctx, *rr.TmdbID, rr.MediaType)
		if err != nil {
			summary.Failed++
			if len(summary.Errors) < 10 {
				summary.Errors = append(summary.Errors, map[string]interface{}{"id": rr.ID, "name": rr.Name, "error": err.Error()})
			}
			continue
		}
		if details == nil {
			summary.Skipped++
			continue
		}
		if err := s.db.UpdateRatings(rr.ID, details.VoteAverage, details.VoteCount, nowISO()); err != nil {
			summary.Failed++
			if len(summary.Errors) < 10 {
				summary.Errors = append(summary.Errors, map[string]interface{}{"id": rr.ID, "name": rr.Name, "error": err.Error()})
			}
			continue
		}
		summary.Updated++
	}
	writeJSON(w, http.StatusOK, summary)
}
