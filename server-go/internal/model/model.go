// Package model 定义数据结构与前端 JSON 整形逻辑。
// 对应 JS 版 index.js 的 shapeFilm / shapeEntry / META_COLS / localImageUrl。
package model

import (
	"encoding/json"
	"strings"
)

// ImageURL 拼接 TMDB 图片完整 URL，path 为空返回 nil（对应 tmdb.js imageUrl）。
func ImageURL(path *string, size string) *string {
	if path == nil || *path == "" {
		return nil
	}
	s := "https://image.tmdb.org/t/p/" + size + *path
	return &s
}

// localImageURL 本地图片访问路径，file 为空返回 nil（对应 localImageUrl）。
func localImageURL(file *string) *string {
	if file == nil || *file == "" {
		return nil
	}
	s := "/images/" + *file
	return &s
}

// MetaCols 对应 JS 版 META_COLS：显式列出 film_metadata 各列并带 m_ 前缀。
// 与 films 表显式列拼接后即 SELECT 列顺序，供 Scan 按序读取。
const MetaCols = `m.film_id AS m_film_id, m.imdb_id AS m_imdb_id, m.tmdb_id AS m_tmdb_id, m.media_type AS m_media_type,
  m.title AS m_title, m.original_title AS m_original_title, m.overview AS m_overview,
  m.poster_path AS m_poster_path, m.backdrop_path AS m_backdrop_path,
  m.poster_local AS m_poster_local, m.backdrop_local AS m_backdrop_local,
  m.genres AS m_genres, m.production_countries AS m_countries, m.runtime AS m_runtime,
  m.vote_average AS m_vote_average, m.vote_count AS m_vote_count,
  m.directors AS m_directors, m.cast AS m_cast, m.release_date AS m_release_date,
  m.status AS m_status, m.tagline AS m_tagline, m.updated_at AS m_updated_at,
  m.original_language AS m_original_language, m.spoken_languages AS m_spoken_languages,
  m.origin_country AS m_origin_country, m.production_companies AS m_production_companies,
  m.writers AS m_writers, m.cinematographers AS m_cinematographers,
  m.composers AS m_composers, m.producers AS m_producers, m.keywords AS m_keywords,
  m.number_of_seasons AS m_number_of_seasons, m.number_of_episodes AS m_number_of_episodes,
  m.budget AS m_budget, m.revenue AS m_revenue,
  m.content_rating AS m_content_rating, m.homepage AS m_homepage`

// FilmsCols 显式列出 films 表列（v2 影视级字段），顺序固定，避免依赖 ADD COLUMN 后的物理顺序。
const FilmsCols = `f.id, f.category, f.name, f.imdb_id, f.douban_id,
  f.production_countries_raw, f.release_year, f.total_episodes`

// ViewingsCols 显式列出 viewings 表列（无表别名，详情查询用）。
const ViewingsCols = `id, watch_year, start_date, end_date, platforms_raw, location, notes`

// ListCols 列表查询列：观看记录 + 影视 + 元数据（GET /api/films），顺序与 EntryRow.ScanPtrs 一致。
const ListCols = `v.id, v.watch_year, v.start_date, v.end_date, v.platforms_raw, v.location, v.notes, ` +
	FilmsCols + `, ` + MetaCols

// FilmRow 是 films LEFT JOIN film_metadata 的扫描结构（影视级），字段顺序与
// (FilmsCols + MetaCols) 严格一致。所有可空字段用指针，NULL→nil。
type FilmRow struct {
	// films 列
	ID                    int64   `db:"id"`
	Category              *string `db:"category"`
	Name                  *string `db:"name"`
	ImdbID                *string `db:"imdb_id"`
	DoubanID              *string `db:"douban_id"`
	ProductionCountries   *string `db:"production_countries_raw"`
	ReleaseYear           *int64  `db:"release_year"`
	TotalEpisodes         *int64  `db:"total_episodes"`
	// film_metadata 列（m_ 前缀）
	MFilmID              *int64   `db:"m_film_id"`
	MImdbID              *string  `db:"m_imdb_id"`
	MTmdbID              *int64   `db:"m_tmdb_id"`
	MMediaType           *string  `db:"m_media_type"`
	MTitle               *string  `db:"m_title"`
	MOriginalTitle       *string  `db:"m_original_title"`
	MOverview            *string  `db:"m_overview"`
	MPosterPath          *string  `db:"m_poster_path"`
	MBackdropPath        *string  `db:"m_backdrop_path"`
	MPosterLocal         *string  `db:"m_poster_local"`
	MBackdropLocal       *string  `db:"m_backdrop_local"`
	MGenres              *string  `db:"m_genres"`
	MCountries           *string  `db:"m_countries"`
	MRuntime             *int64   `db:"m_runtime"`
	MVoteAverage         *float64 `db:"m_vote_average"`
	MVoteCount           *int64   `db:"m_vote_count"`
	MDirectors           *string  `db:"m_directors"`
	MCast                *string  `db:"m_cast"`
	MReleaseDate         *string  `db:"m_release_date"`
	MStatus              *string  `db:"m_status"`
	MTagline             *string  `db:"m_tagline"`
	MUpdatedAt           *string  `db:"m_updated_at"`
	MOriginalLanguage    *string  `db:"m_original_language"`
	MSpokenLanguages     *string  `db:"m_spoken_languages"`
	MOriginCountry       *string  `db:"m_origin_country"`
	MProductionCompanies *string  `db:"m_production_companies"`
	MWriters             *string  `db:"m_writers"`
	MCinematographers    *string  `db:"m_cinematographers"`
	MComposers           *string  `db:"m_composers"`
	MProducers           *string  `db:"m_producers"`
	MKeywords            *string  `db:"m_keywords"`
	MNumberOfSeasons     *int64   `db:"m_number_of_seasons"`
	MNumberOfEpisodes    *int64   `db:"m_number_of_episodes"`
	MBudget              *int64   `db:"m_budget"`
	MRevenue             *int64   `db:"m_revenue"`
	MContentRating       *string  `db:"m_content_rating"`
	MHomepage            *string  `db:"m_homepage"`
}

// ScanPtrs 返回按字段顺序排列的扫描目标指针切片，供 rows.Scan 使用。
// 顺序必须与 (FilmsCols + MetaCols) 完全一致。
func (r *FilmRow) ScanPtrs() []interface{} {
	return []interface{}{
		&r.ID, &r.Category, &r.Name, &r.ImdbID, &r.DoubanID,
		&r.ProductionCountries, &r.ReleaseYear, &r.TotalEpisodes,
		&r.MFilmID, &r.MImdbID, &r.MTmdbID, &r.MMediaType, &r.MTitle, &r.MOriginalTitle,
		&r.MOverview, &r.MPosterPath, &r.MBackdropPath, &r.MPosterLocal, &r.MBackdropLocal,
		&r.MGenres, &r.MCountries, &r.MRuntime, &r.MVoteAverage, &r.MVoteCount,
		&r.MDirectors, &r.MCast, &r.MReleaseDate, &r.MStatus, &r.MTagline, &r.MUpdatedAt,
		&r.MOriginalLanguage, &r.MSpokenLanguages, &r.MOriginCountry, &r.MProductionCompanies,
		&r.MWriters, &r.MCinematographers, &r.MComposers, &r.MProducers, &r.MKeywords,
		&r.MNumberOfSeasons, &r.MNumberOfEpisodes, &r.MBudget, &r.MRevenue,
		&r.MContentRating, &r.MHomepage,
	}
}

// ViewingRow 是 viewings 表的扫描结构。
type ViewingRow struct {
	ID           int64   `db:"id"`
	WatchYear    *int64  `db:"watch_year"`
	StartDate    *string `db:"start_date"`
	EndDate      *string `db:"end_date"`
	PlatformsRaw *string `db:"platforms_raw"`
	Location     *string `db:"location"`
	Notes        *string `db:"notes"`
}

// ScanPtrs 顺序与 ViewingsCols 完全一致。
func (r *ViewingRow) ScanPtrs() []interface{} {
	return []interface{}{&r.ID, &r.WatchYear, &r.StartDate, &r.EndDate, &r.PlatformsRaw, &r.Location, &r.Notes}
}

// EntryRow 是列表查询（viewings JOIN films LEFT JOIN film_metadata）的扫描结构，
// 顺序与 ListCols（ViewingsCols + FilmsCols + MetaCols）严格一致。
type EntryRow struct {
	ViewingRow
	FilmRow
}

// ScanPtrs 顺序与 ListCols 完全一致。
func (r *EntryRow) ScanPtrs() []interface{} {
	return append(r.ViewingRow.ScanPtrs(), r.FilmRow.ScanPtrs()...)
}

// --- 前端输出结构（键名严格对齐 JS shapeEntry / shapeFilm 输出） ---

// SpokenLanguage 对应 spoken_languages JSON 数组元素。
type SpokenLanguage struct {
	ISO         string `json:"iso"`
	Name        string `json:"name"`
	EnglishName string `json:"english_name"`
}

// ProductionCompany 对应 production_companies JSON 数组元素。
type ProductionCompany struct {
	ID            *int64 `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

// MetadataOut 是前端 metadata 子对象。
type MetadataOut struct {
	TmdbID              *int64              `json:"tmdbId"`
	MediaType           string              `json:"mediaType"`
	Title               string              `json:"title"`
	OriginalTitle       string              `json:"originalTitle"`
	Overview            string              `json:"overview"`
	PosterPath          string              `json:"posterPath"`
	BackdropPath        string              `json:"backdropPath"`
	PosterLocal         string              `json:"posterLocal"`
	BackdropLocal       string              `json:"backdropLocal"`
	PosterURL           *string             `json:"posterUrl"`
	BackdropURL         *string             `json:"backdropUrl"`
	Genres              []string            `json:"genres"`
	Runtime             *int64              `json:"runtime"`
	VoteAverage         *float64            `json:"voteAverage"`
	VoteCount           *int64              `json:"voteCount"`
	Directors           []string            `json:"directors"`
	Cast                []string            `json:"cast"`
	ReleaseDate         string              `json:"releaseDate"`
	Status              string              `json:"status"`
	Tagline             string              `json:"tagline"`
	UpdatedAt           string              `json:"updatedAt"`
	OriginalLanguage    string              `json:"originalLanguage"`
	SpokenLanguages     []SpokenLanguage    `json:"spokenLanguages"`
	OriginCountry       []string            `json:"originCountry"`
	ProductionCompanies []ProductionCompany `json:"productionCompanies"`
	Writers             []string            `json:"writers"`
	Cinematographers    []string            `json:"cinematographers"`
	Composers           []string            `json:"composers"`
	Producers           []string            `json:"producers"`
	Keywords            []string            `json:"keywords"`
	NumberOfSeasons     *int64              `json:"numberOfSeasons"`
	NumberOfEpisodes    *int64              `json:"numberOfEpisodes"`
	Budget              *int64              `json:"budget"`
	Revenue             *int64              `json:"revenue"`
	ContentRating       string              `json:"contentRating"`
	Homepage            string              `json:"homepage"`
}

// ViewingOut 观看记录（前端对象）。
type ViewingOut struct {
	ID        int64    `json:"id"`
	WatchYear *int64   `json:"watchYear"`
	StartDate string   `json:"startDate"`
	EndDate   string   `json:"endDate"`
	Platforms []string `json:"platforms"`
	Location  string   `json:"location"`
	Notes     string   `json:"notes"`
}

// Entry 是列表条目：一条观看记录 + 所属影视信息。
// 字段名与旧版 films 输出保持一致，卡片/表格视图无需改动；id 为观看记录 id。
type Entry struct {
	ID                     int64        `json:"id"`
	FilmID                 int64        `json:"filmId"`
	WatchYear              *int64       `json:"watchYear"`
	StartDate              string       `json:"startDate"`
	EndDate                string       `json:"endDate"`
	Platforms              []string     `json:"platforms"`
	Location               string       `json:"location"`
	Notes                  string       `json:"notes"`
	Category               string       `json:"category"`
	Name                   string       `json:"name"`
	ImdbID                 string       `json:"imdbId"`
	DoubanID               string       `json:"doubanId"`
	ReleaseYear            *int64       `json:"releaseYear"`
	TotalEpisodes          *int64       `json:"totalEpisodes"`
	ProductionCountries    []string     `json:"productionCountries"`
	ProductionCountriesRaw string       `json:"productionCountriesRaw"`
	HasMetadata            bool         `json:"hasMetadata"`
	Metadata               *MetadataOut `json:"metadata"`
}

// FilmDetail 是影视详情：影视级信息 + 该影视的全部观看记录（id 为影视 id）。
type FilmDetail struct {
	ID                     int64        `json:"id"`
	Name                   string       `json:"name"`
	Category               string       `json:"category"`
	ImdbID                 string       `json:"imdbId"`
	DoubanID               string       `json:"doubanId"`
	ReleaseYear            *int64       `json:"releaseYear"`
	TotalEpisodes          *int64       `json:"totalEpisodes"`
	ProductionCountries    []string     `json:"productionCountries"`
	ProductionCountriesRaw string       `json:"productionCountriesRaw"`
	HasMetadata            bool         `json:"hasMetadata"`
	Metadata               *MetadataOut `json:"metadata"`
	Viewings               []ViewingOut `json:"viewings"`
}

// parseJSONStrings 把 JSON 字符串数组解析为 []string，失败或空返回空切片。
func parseJSONStrings(raw *string) []string {
	out := []string{}
	if raw == nil || *raw == "" {
		return out
	}
	if err := json.Unmarshal([]byte(*raw), &out); err != nil {
		return out
	}
	return out
}

type rawCountry struct {
	ISO  string `json:"iso"`
	Name string `json:"name"`
}

// parseCountries 解析 production_countries JSON（[{iso,name}]），返回 name 列表。
func parseCountries(raw *string) []string {
	if raw == nil || *raw == "" {
		return nil
	}
	var cs []rawCountry
	if err := json.Unmarshal([]byte(*raw), &cs); err != nil {
		return nil
	}
	out := []string{}
	for _, c := range cs {
		if c.Name != "" {
			out = append(out, c.Name)
		}
	}
	return out
}

func parseSpokenLanguages(raw *string) []SpokenLanguage {
	out := []SpokenLanguage{}
	if raw == nil || *raw == "" {
		return out
	}
	_ = json.Unmarshal([]byte(*raw), &out)
	return out
}

func parseProductionCompanies(raw *string) []ProductionCompany {
	out := []ProductionCompany{}
	if raw == nil || *raw == "" {
		return out
	}
	_ = json.Unmarshal([]byte(*raw), &out)
	// 过滤掉无 name 的项（与 JS .filter(c => c.name) 一致）
	res := out[:0]
	for _, c := range out {
		if c.Name != "" {
			res = append(res, c)
		}
	}
	if res == nil {
		res = []ProductionCompany{}
	}
	return res
}

// parsePlatforms 按 "," 拆分平台原始字段。
func parsePlatforms(raw *string) []string {
	platforms := []string{}
	if raw == nil || *raw == "" {
		return platforms
	}
	for _, p := range strings.Split(*raw, ",") {
		s := strings.TrimSpace(p)
		if s != "" {
			platforms = append(platforms, s)
		}
	}
	return platforms
}

func strPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// shapeCountries 制片国家：优先元数据，否则用 Excel 字段按 "/" 拆分。
func shapeCountries(r *FilmRow) []string {
	countries := parseCountries(r.MCountries)
	if len(countries) == 0 && r.ProductionCountries != nil && *r.ProductionCountries != "" {
		for _, c := range strings.Split(*r.ProductionCountries, "/") {
			s := strings.TrimSpace(c)
			if s != "" {
				countries = append(countries, s)
			}
		}
	}
	if countries == nil {
		countries = []string{}
	}
	return countries
}

// shapeMetadata 整理元数据子对象（图片本地优先、远程兜底）。
func shapeMetadata(r *FilmRow) *MetadataOut {
	if r.MFilmID == nil {
		return nil
	}
	posterURL := localImageURL(r.MPosterLocal)
	if posterURL == nil {
		posterURL = ImageURL(r.MPosterPath, "w500")
	}
	backdropURL := localImageURL(r.MBackdropLocal)
	if backdropURL == nil {
		backdropURL = ImageURL(r.MBackdropPath, "w1280")
	}
	return &MetadataOut{
		TmdbID:              r.MTmdbID,
		MediaType:           strPtr(r.MMediaType),
		Title:               strPtr(r.MTitle),
		OriginalTitle:       strPtr(r.MOriginalTitle),
		Overview:            strPtr(r.MOverview),
		PosterPath:          strPtr(r.MPosterPath),
		BackdropPath:        strPtr(r.MBackdropPath),
		PosterLocal:         strPtr(r.MPosterLocal),
		BackdropLocal:       strPtr(r.MBackdropLocal),
		PosterURL:           posterURL,
		BackdropURL:         backdropURL,
		Genres:              parseJSONStrings(r.MGenres),
		Runtime:             r.MRuntime,
		VoteAverage:         r.MVoteAverage,
		VoteCount:           r.MVoteCount,
		Directors:           parseJSONStrings(r.MDirectors),
		Cast:                parseJSONStrings(r.MCast),
		ReleaseDate:         strPtr(r.MReleaseDate),
		Status:              strPtr(r.MStatus),
		Tagline:             strPtr(r.MTagline),
		UpdatedAt:           strPtr(r.MUpdatedAt),
		OriginalLanguage:    strPtr(r.MOriginalLanguage),
		SpokenLanguages:     parseSpokenLanguages(r.MSpokenLanguages),
		OriginCountry:       parseJSONStrings(r.MOriginCountry),
		ProductionCompanies: parseProductionCompanies(r.MProductionCompanies),
		Writers:             parseJSONStrings(r.MWriters),
		Cinematographers:    parseJSONStrings(r.MCinematographers),
		Composers:           parseJSONStrings(r.MComposers),
		Producers:           parseJSONStrings(r.MProducers),
		Keywords:            parseJSONStrings(r.MKeywords),
		NumberOfSeasons:     r.MNumberOfSeasons,
		NumberOfEpisodes:    r.MNumberOfEpisodes,
		Budget:              r.MBudget,
		Revenue:             r.MRevenue,
		ContentRating:       strPtr(r.MContentRating),
		Homepage:            strPtr(r.MHomepage),
	}
}

// ShapeViewing 把观看记录行整理为前端对象。
func ShapeViewing(v *ViewingRow) ViewingOut {
	return ViewingOut{
		ID:        v.ID,
		WatchYear: v.WatchYear,
		StartDate: strPtr(v.StartDate),
		EndDate:   strPtr(v.EndDate),
		Platforms: parsePlatforms(v.PlatformsRaw),
		Location:  strPtr(v.Location),
		Notes:     strPtr(v.Notes),
	}
}

// ShapeEntry 把列表扫描行整理为前端条目对象（对应 JS shapeEntry）。
func ShapeEntry(r *EntryRow) Entry {
	return Entry{
		ID:                     r.ViewingRow.ID,
		FilmID:                 r.FilmRow.ID,
		WatchYear:              r.ViewingRow.WatchYear,
		StartDate:              strPtr(r.ViewingRow.StartDate),
		EndDate:                strPtr(r.ViewingRow.EndDate),
		Platforms:              parsePlatforms(r.ViewingRow.PlatformsRaw),
		Location:               strPtr(r.ViewingRow.Location),
		Notes:                  strPtr(r.ViewingRow.Notes),
		Category:               strPtr(r.FilmRow.Category),
		Name:                   strPtr(r.FilmRow.Name),
		ImdbID:                 strPtr(r.FilmRow.ImdbID),
		DoubanID:               strPtr(r.FilmRow.DoubanID),
		ReleaseYear:            r.FilmRow.ReleaseYear,
		TotalEpisodes:          r.FilmRow.TotalEpisodes,
		ProductionCountries:    shapeCountries(&r.FilmRow),
		ProductionCountriesRaw: strPtr(r.FilmRow.ProductionCountries),
		HasMetadata:            r.FilmRow.MTmdbID != nil && *r.FilmRow.MTmdbID != 0,
		Metadata:               shapeMetadata(&r.FilmRow),
	}
}

// ShapeFilm 把影视级行与观看记录列表整理为前端详情对象（对应 JS shapeFilm）。
func ShapeFilm(r *FilmRow, viewings []ViewingRow) FilmDetail {
	vs := make([]ViewingOut, 0, len(viewings))
	for i := range viewings {
		vs = append(vs, ShapeViewing(&viewings[i]))
	}
	return FilmDetail{
		ID:                     r.ID,
		Name:                   strPtr(r.Name),
		Category:               strPtr(r.Category),
		ImdbID:                 strPtr(r.ImdbID),
		DoubanID:               strPtr(r.DoubanID),
		ReleaseYear:            r.ReleaseYear,
		TotalEpisodes:          r.TotalEpisodes,
		ProductionCountries:    shapeCountries(r),
		ProductionCountriesRaw: strPtr(r.ProductionCountries),
		HasMetadata:            r.MTmdbID != nil && *r.MTmdbID != 0,
		Metadata:               shapeMetadata(r),
		Viewings:               vs,
	}
}
