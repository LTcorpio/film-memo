package tmdb

import "encoding/json"

// Credits 是 TMDB credits 响应的公共结构（movie/tv/season 通用）。
type Credits struct {
	Crew []CrewMember `json:"crew"`
	Cast []CastMember `json:"cast"`
}

// CrewMember 演职员（crew）条目。
type CrewMember struct {
	Job  string `json:"job"`
	Name string `json:"name"`
}

// CastMember 演员（cast）条目。
type CastMember struct {
	Name string `json:"name"`
}

// Genre 类型。
type Genre struct {
	Name string `json:"name"`
}

// ProductionCountry 制片国家。
type ProductionCountry struct {
	ISO31661 string `json:"iso_3166_1"`
	Name     string `json:"name"`
}

// SpokenLanguage 对白语言。
type SpokenLanguage struct {
	ISO6391     string `json:"iso_639_1"`
	Name        string `json:"name"`
	EnglishName string `json:"english_name"`
}

// ProductionCompany 制片公司。
type ProductionCompany struct {
	ID            *int64 `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

// KeywordResult 关键词。
type KeywordResult struct {
	Name string `json:"name"`
}

// ContentRating 电视内容分级条目。
type ContentRating struct {
	ISO31661 string `json:"iso_3166_1"`
	Rating   string `json:"rating"`
}

// ReleaseDate 分级明细。
type ReleaseDate struct {
	Certification string `json:"certification"`
}

// ReleaseDatesResult 电影分级条目。
type ReleaseDatesResult struct {
	ISO31661     string        `json:"iso_3166_1"`
	ReleaseDates []ReleaseDate `json:"release_dates"`
}

// Creator tv 创建者。
type Creator struct {
	Name string `json:"name"`
}

// ExternalIDs 外部 ID。
type ExternalIDs struct {
	ImdbID string `json:"imdb_id"`
}

// KeywordsResp keywords 响应。
type KeywordsResp struct {
	Results []KeywordResult `json:"results"`
}

// ContentRatingsResp content_ratings 响应。
type ContentRatingsResp struct {
	Results []ContentRating `json:"results"`
}

// ReleaseDatesResp release_dates 响应。
type ReleaseDatesResp struct {
	Results []ReleaseDatesResult `json:"results"`
}

// Details 是 TMDB movie/tv 详情（append_to_response 合并后的字段集合）。
// movie 专属字段（Title/Runtime/Budget/Revenue/ReleaseDate）与 tv 专属字段
// （Name/OriginalName/FirstAirDate/EpisodeRunTime/NumberOfSeasons/NumberOfEpisodes/CreatedBy）
// 都在此声明；另一类型缺失时为零值/nil。
type Details struct {
	ID   int64  `json:"id"`
	Name string `json:"name"` // tv
	OriginalName string `json:"original_name"` // tv
	FirstAirDate string `json:"first_air_date"` // tv
	EpisodeRunTime []int64 `json:"episode_run_time"` // tv
	NumberOfSeasons *int64 `json:"number_of_seasons"` // tv
	NumberOfEpisodes *int64 `json:"number_of_episodes"` // tv
	CreatedBy []Creator `json:"created_by"` // tv

	Title         string `json:"title"` // movie
	OriginalTitle string `json:"original_title"` // movie
	ReleaseDate   string `json:"release_date"` // movie
	Runtime       *int64 `json:"runtime"` // movie
	Budget        *int64 `json:"budget"` // movie
	Revenue       *int64 `json:"revenue"` // movie

	Overview           string                 `json:"overview"`
	PosterPath         string                 `json:"poster_path"`
	BackdropPath       string                 `json:"backdrop_path"`
	Genres             []Genre                `json:"genres"`
	ProductionCountries []ProductionCountry    `json:"production_countries"`
	VoteAverage        *float64               `json:"vote_average"`
	VoteCount          *int64                 `json:"vote_count"`
	Status             string                 `json:"status"`
	Tagline            string                 `json:"tagline"`
	OriginalLanguage   string                 `json:"original_language"`
	SpokenLanguages    []SpokenLanguage       `json:"spoken_languages"`
	OriginCountry      []string               `json:"origin_country"`
	ProductionCompanies []ProductionCompany    `json:"production_companies"`
	Homepage           string                 `json:"homepage"`
	ExternalIDs        ExternalIDs            `json:"external_ids"`
	Credits            Credits                `json:"credits"`
	Keywords           KeywordsResp           `json:"keywords"`
	ContentRatings     ContentRatingsResp     `json:"content_ratings"` // tv
	ReleaseDates       ReleaseDatesResp       `json:"release_dates"`   // movie
}

// Found 是 findByImdb 的匹配结果。
type Found struct {
	TmdbID    int64
	MediaType string // "movie" / "tv"
}

// SearchResult 是 searchByName 的单条候选。
type SearchResult struct {
	TmdbID        int64   `json:"tmdb_id"`
	MediaType     string  `json:"media_type"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	ReleaseYear   *string `json:"release_year"`
	PosterPath    string  `json:"poster_path"`
	Overview      string  `json:"overview"`
}

// Season 是 getSeasons 返回的季摘要。
type Season struct {
	SeasonNumber  *int64 `json:"season_number"`
	Name          string `json:"name"`
	AirDate       string `json:"air_date"`
	EpisodeCount  int64  `json:"episode_count"`
	PosterPath    string `json:"poster_path"`
	Overview      string `json:"overview"`
}

// SeasonDetails 是 getSeasonDetails 返回的季详情（含 credits）。
type SeasonDetails struct {
	PosterPath string          `json:"poster_path"`
	AirDate    string          `json:"air_date"`
	Overview   string          `json:"overview"`
	Episodes   []json.RawMessage `json:"episodes"` // 仅需长度
	Credits    Credits         `json:"credits"`
}
