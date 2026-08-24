// Package model 定义数据结构与前端 JSON 整形逻辑。
// 对应 JS 版 index.js 的 shapeFilm / META_COLS / localImageUrl。
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

// FilmsCols 显式列出 films 表列，顺序固定，避免依赖 ADD COLUMN 后的物理顺序。
const FilmsCols = `f.id, f.watch_year, f.category, f.name, f.imdb_id, f.douban_id,
  f.production_countries_raw, f.release_year, f.start_date, f.end_date,
  f.total_episodes, f.platforms_raw, f.location, f.notes`

// FilmRow 是 films LEFT JOIN film_metadata 的扫描结构，字段顺序与
// (FilmsCols + MetaCols) 严格一致。所有可空字段用指针，NULL→nil。
type FilmRow struct {
	// films 列
	ID                  int64   `db:"id"`
	WatchYear           *int64  `db:"watch_year"`
	Category            *string `db:"category"`
	Name                *string `db:"name"`
	ImdbID              *string `db:"imdb_id"`
	DoubanID            *string `db:"douban_id"`
	ProductionCountries *string `db:"production_countries_raw"`
	ReleaseYear         *int64  `db:"release_year"`
	StartDate           *string `db:"start_date"`
	EndDate             *string `db:"end_date"`
	TotalEpisodes       *int64  `db:"total_episodes"`
	PlatformsRaw        *string `db:"platforms_raw"`
	Location            *string `db:"location"`
	Notes               *string `db:"notes"`
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
		&r.ID, &r.WatchYear, &r.Category, &r.Name, &r.ImdbID, &r.DoubanID,
		&r.ProductionCountries, &r.ReleaseYear, &r.StartDate, &r.EndDate,
		&r.TotalEpisodes, &r.PlatformsRaw, &r.Location, &r.Notes,
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

// --- 前端输出结构（键名严格对齐 JS shapeFilm 输出） ---

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

// Film 是前端影片对象（shapeFilm 输出）。
type Film struct {
	ID                    int64        `json:"id"`
	WatchYear             *int64       `json:"watchYear"`
	Category              string       `json:"category"`
	Name                  string       `json:"name"`
	ImdbID                string       `json:"imdbId"`
	DoubanID              string       `json:"doubanId"`
	ReleaseYear           *int64       `json:"releaseYear"`
	StartDate             string       `json:"startDate"`
	EndDate               string       `json:"endDate"`
	TotalEpisodes         *int64       `json:"totalEpisodes"`
	Platforms             []string     `json:"platforms"`
	ProductionCountries   []string     `json:"productionCountries"`
	ProductionCountriesRaw string      `json:"productionCountriesRaw"`
	Location              string       `json:"location"`
	Notes                 string       `json:"notes"`
	HasMetadata           bool         `json:"hasMetadata"`
	Metadata              *MetadataOut `json:"metadata"`
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

func strPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ShapeFilm 把扫描行整理为前端友好对象（对应 shapeFilm）。
func ShapeFilm(r *FilmRow) Film {
	// 平台：按逗号拆分
	platforms := []string{}
	if r.PlatformsRaw != nil && *r.PlatformsRaw != "" {
		for _, p := range strings.Split(*r.PlatformsRaw, ",") {
			s := strings.TrimSpace(p)
			if s != "" {
				platforms = append(platforms, s)
			}
		}
	}

	// 制片国家：优先元数据，否则用 Excel 字段按 "/" 拆分
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

	genres := parseJSONStrings(r.MGenres)
	directors := parseJSONStrings(r.MDirectors)
	cast := parseJSONStrings(r.MCast)
	writers := parseJSONStrings(r.MWriters)
	cinematographers := parseJSONStrings(r.MCinematographers)
	composers := parseJSONStrings(r.MComposers)
	producers := parseJSONStrings(r.MProducers)
	keywords := parseJSONStrings(r.MKeywords)
	originCountry := parseJSONStrings(r.MOriginCountry)
	spokenLanguages := parseSpokenLanguages(r.MSpokenLanguages)
	productionCompanies := parseProductionCompanies(r.MProductionCompanies)

	// 图片：本地优先，远程兜底
	posterURL := localImageURL(r.MPosterLocal)
	if posterURL == nil {
		posterURL = ImageURL(r.MPosterPath, "w500")
	}
	backdropURL := localImageURL(r.MBackdropLocal)
	if backdropURL == nil {
		backdropURL = ImageURL(r.MBackdropPath, "w1280")
	}

	f := Film{
		ID:                     r.ID,
		WatchYear:              r.WatchYear,
		Category:               strPtr(r.Category),
		Name:                   strPtr(r.Name),
		ImdbID:                 strPtr(r.ImdbID),
		DoubanID:               strPtr(r.DoubanID),
		ReleaseYear:            r.ReleaseYear,
		StartDate:              strPtr(r.StartDate),
		EndDate:                strPtr(r.EndDate),
		TotalEpisodes:          r.TotalEpisodes,
		Platforms:              platforms,
		ProductionCountries:    countries,
		ProductionCountriesRaw: strPtr(r.ProductionCountries),
		Location:               strPtr(r.Location),
		Notes:                  strPtr(r.Notes),
		HasMetadata:            r.MTmdbID != nil && *r.MTmdbID != 0,
		Metadata:               nil,
	}

	if r.MFilmID != nil {
		f.Metadata = &MetadataOut{
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
			Genres:              genres,
			Runtime:             r.MRuntime,
			VoteAverage:         r.MVoteAverage,
			VoteCount:           r.MVoteCount,
			Directors:           directors,
			Cast:                cast,
			ReleaseDate:         strPtr(r.MReleaseDate),
			Status:              strPtr(r.MStatus),
			Tagline:             strPtr(r.MTagline),
			UpdatedAt:           strPtr(r.MUpdatedAt),
			OriginalLanguage:    strPtr(r.MOriginalLanguage),
			SpokenLanguages:     spokenLanguages,
			OriginCountry:       originCountry,
			ProductionCompanies: productionCompanies,
			Writers:             writers,
			Cinematographers:    cinematographers,
			Composers:           composers,
			Producers:           producers,
			Keywords:            keywords,
			NumberOfSeasons:     r.MNumberOfSeasons,
			NumberOfEpisodes:    r.MNumberOfEpisodes,
			Budget:              r.MBudget,
			Revenue:             r.MRevenue,
			ContentRating:       strPtr(r.MContentRating),
			Homepage:            strPtr(r.MHomepage),
		}
	}
	return f
}
