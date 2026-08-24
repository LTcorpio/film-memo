package db

// Filter 列表/筛选与评分刷新共用的筛选条件（对应 GET /api/films 与 POST /api/ratings/refresh 的查询参数）。
type Filter struct {
	WatchYear   *int64
	ReleaseYear *int64
	Platform    string
	Category    string // "__no_meta__" 表示无元数据筛选
	Q           string
	Missing     string // "imdb" / "douban"
}

// Filters 是 GET /api/filters 的响应。
type Filters struct {
	WatchYears   []int64  `json:"watchYears"`
	ReleaseYears []int64  `json:"releaseYears"`
	Categories   []string `json:"categories"`
	Platforms    []string `json:"platforms"`
}

// CatStat 分类统计项。
type CatStat struct {
	K *string `json:"k"`
	C int64   `json:"c"`
}

// YearStat 年份统计项。
type YearStat struct {
	K *int64 `json:"k"`
	C int64  `json:"c"`
}

// Stats 是 GET /api/stats 的响应。
type Stats struct {
	Total           int64      `json:"total"`
	WithMetadata    int64      `json:"withMetadata"`
	WithoutMetadata int64      `json:"withoutMetadata"`
	WithoutImdb     int64      `json:"withoutImdb"`
	WithoutDouban   int64      `json:"withoutDouban"`
	ByCategory      []CatStat  `json:"byCategory"`
	ByWatchYear     []YearStat `json:"byWatchYear"`
}

// FilmRef 是评分刷新时遍历的影片引用。
type FilmRef struct {
	ID        int64
	Name      string
	TmdbID    *int64
	MediaType string
}

// FilmBasic 是保存元数据时所需的影片基础信息。
type FilmBasic struct {
	ID       int64
	Name     string
	Category string
	ImdbID   string
}

// ImageLocals 是某影片的本地图片文件名对。
type ImageLocals struct {
	Poster   *string
	Backdrop *string
}

// MetaPaths 是某影片元数据中的 TMDB 远程图片路径对。
type MetaPaths struct {
	PosterPath   *string
	BackdropPath *string
}
