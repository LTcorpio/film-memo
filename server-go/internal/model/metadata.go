package model

// Metadata 是 TMDB 归一化后用于写入 film_metadata 表的结构（对应 tmdb.js normalizeDetails 返回值）。
// JSON 字段（genres/directors 等）以 JSON 字符串形式存储，与 JS 版 JSON.stringify 一致。
type Metadata struct {
	ImdbID              string
	TmdbID              int64
	MediaType           string
	Title               string
	OriginalTitle       string
	Overview            string
	PosterPath          string
	BackdropPath        string
	Genres              string // JSON
	ProductionCountries string // JSON
	Runtime             *int64
	VoteAverage         *float64
	VoteCount           *int64
	Directors           string // JSON
	Cast                string // JSON
	ReleaseDate         string
	Status              string
	Tagline             string
	OriginalLanguage    string
	SpokenLanguages     string // JSON
	OriginCountry       string // JSON
	ProductionCompanies string // JSON
	Writers             string // JSON
	Cinematographers    string // JSON
	Composers           string // JSON
	Producers           string // JSON
	Keywords            string // JSON
	NumberOfSeasons     *int64
	NumberOfEpisodes    *int64
	Budget              *int64
	Revenue             *int64
	ContentRating       string
	Homepage            string
	UpdatedAt           string
}
