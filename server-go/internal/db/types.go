package db

// Filter 列表/筛选与评分刷新共用的筛选条件（对应 GET /api/films 与 POST /api/ratings/refresh 的查询参数）。
type Filter struct {
	WatchYear   *int64
	ReleaseYear *int64
	Platform    string
	Category    string // "__no_meta__" 表示无元数据筛选
	Q           string
	Missing     string  // "imdb" / "douban"：对应 ID 为空的筛选
	IDs         []int64 // 仅处理这些影视（评分管理里手动勾选的条目）
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
	Category  string
	ImdbID    string
	DoubanID  string
	TmdbID    *int64
	MediaType string
	// 两个来源各自是否需要重新拉取：缺评分（含无元数据行）时为 true。
	// 刷新时只处理 Need 的来源，避免每次全量重抓。
	DoubanNeed bool
	ImdbNeed   bool
}

// RatingSet 是单次评分写入的字段集合：nil 表示该来源本次未取到，保持原值不变。
type RatingSet struct {
	DoubanRating *float64
	ImdbRating   *float64
}

// Empty 返回该集合是否没有任何待写入字段。
func (s RatingSet) Empty() bool {
	return s.DoubanRating == nil && s.ImdbRating == nil
}

// RatingSync 记录单个评分数据源最近一次拉取的时间与结果（rating_sync 表一行）。
type RatingSync struct {
	Source       string  `json:"source"`       // 数据源：douban / imdb
	LastSyncedAt *string `json:"lastSyncedAt"` // 最近一次拉取时间
	Total        int64   `json:"total"`        // 该源覆盖的影片数
	Updated      int64   `json:"updated"`      // 成功写入数
	Skipped      int64   `json:"skipped"`      // 跳过数
	Failed       int64   `json:"failed"`       // 失败数
	Message      *string `json:"message"`      // 附加说明
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
