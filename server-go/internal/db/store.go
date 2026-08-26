package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"film-memo/internal/model"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// nowISO 返回当前 UTC 时间的 ISO 字符串（对应 JS new Date().toISOString()）。
func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00")
}

// --- 字段白名单与类型分类（v2：影视级 films / 观看级 viewings 两组） ---

var filmFieldWhiteList = []string{
	"category", "name", "imdb_id", "douban_id", "production_countries_raw",
	"release_year", "total_episodes",
}

var filmIntFields = map[string]bool{
	"release_year": true, "total_episodes": true,
}

var viewingFieldWhiteList = []string{
	"watch_year", "start_date", "end_date", "platforms_raw", "location", "notes",
}

var viewingIntFields = map[string]bool{
	"watch_year": true,
}

var metaFieldWhiteList = []string{
	"title", "original_title", "overview", "runtime", "vote_average", "vote_count",
	"genres", "production_countries", "media_type", "directors", "cast", "release_date", "status", "tagline",
	"original_language", "spoken_languages", "origin_country", "production_companies",
	"writers", "cinematographers", "composers", "producers", "keywords",
	"number_of_seasons", "number_of_episodes", "budget", "revenue",
	"content_rating", "homepage",
}

var metaJSONFields = map[string]bool{
	"genres": true, "production_countries": true, "directors": true, "cast": true,
	"spoken_languages": true, "origin_country": true, "production_companies": true,
	"writers": true, "cinematographers": true, "composers": true, "producers": true, "keywords": true,
}

var metaIntFields = map[string]bool{
	"runtime": true, "vote_count": true, "number_of_seasons": true,
	"number_of_episodes": true, "budget": true, "revenue": true,
}

var metaFloatFields = map[string]bool{"vote_average": true}

// --- 值转换 ---

func toInt64(v interface{}) interface{} {
	switch x := v.(type) {
	case nil:
		return nil
	case float64:
		return int64(x)
	case int64:
		return x
	case int:
		return int64(x)
	case string:
		if x == "" {
			return nil
		}
		n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		if err != nil {
			return nil
		}
		return n
	}
	return nil
}

func toFloat64(v interface{}) interface{} {
	switch x := v.(type) {
	case nil:
		return nil
	case float64:
		return x
	case int64:
		return float64(x)
	case int:
		return float64(x)
	case string:
		if x == "" {
			return nil
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		if err != nil {
			return nil
		}
		return f
	}
	return nil
}

func toText(v interface{}) interface{} {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		return x
	}
	return nil
}

func toArrayJSON(v interface{}) interface{} {
	switch x := v.(type) {
	case nil:
		return nil
	case []interface{}:
		b, _ := json.Marshal(x)
		return string(b)
	case string:
		return x
	}
	return nil
}

// filmValue 把请求体值转为 films 表列值（按字段类型转换）。
func filmValue(k string, v interface{}) interface{} {
	if filmIntFields[k] {
		return toInt64(v)
	}
	return toText(v)
}

// viewingValue 把请求体值转为 viewings 表列值（按字段类型转换）。
func viewingValue(k string, v interface{}) interface{} {
	if viewingIntFields[k] {
		return toInt64(v)
	}
	return toText(v)
}

// imgLocalColumn 把 poster/backdrop 映射为 poster_local/backdrop_local 列名。
func imgLocalColumn(imgType string) (string, bool) {
	switch imgType {
	case "poster":
		return "poster_local", true
	case "backdrop":
		return "backdrop_local", true
	}
	return "", false
}

// --- 查询 ---

// ListFilms 列表查询（GET /api/films）：每条观看记录一行，附带影视与元数据信息。
func (d *DB) ListFilms(f Filter) ([]model.EntryRow, error) {
	where, args := d.buildWhere(f)
	q := "SELECT " + model.ListCols +
		" FROM viewings v JOIN films f ON f.id = v.film_id LEFT JOIN film_metadata m ON m.film_id = f.id"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY v.watch_year DESC, v.start_date DESC NULLS LAST, v.id DESC"

	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.EntryRow{}
	for rows.Next() {
		var r model.EntryRow
		if err := rows.Scan(r.ScanPtrs()...); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetFilm 单条影视详情（GET /api/films/:id，影视级，不含观看记录）。
func (d *DB) GetFilm(id int64) (*model.FilmRow, error) {
	q := "SELECT " + model.FilmsCols + ", " + model.MetaCols +
		" FROM films f LEFT JOIN film_metadata m ON m.film_id = f.id WHERE f.id = ?"
	var r model.FilmRow
	err := d.db.QueryRow(q, id).Scan(r.ScanPtrs()...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

// ListViewingsByFilm 取某影视的全部观看记录（按观看年份/开始日期/插入顺序升序）。
func (d *DB) ListViewingsByFilm(filmID int64) ([]model.ViewingRow, error) {
	rows, err := d.db.Query(
		"SELECT "+model.ViewingsCols+" FROM viewings WHERE film_id = ? ORDER BY watch_year ASC NULLS LAST, start_date ASC NULLS LAST, id ASC",
		filmID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ViewingRow{}
	for rows.Next() {
		var r model.ViewingRow
		if err := rows.Scan(r.ScanPtrs()...); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Filters 筛选项（GET /api/filters）：观看年份/平台来自 viewings，其余来自 films。
func (d *DB) Filters() (*Filters, error) {
	out := &Filters{
		WatchYears:   []int64{},
		ReleaseYears: []int64{},
		Categories:   []string{},
		Platforms:    []string{},
	}

	// watchYears（观看记录的观看年份）
	rows, err := d.db.Query("SELECT DISTINCT watch_year AS v FROM viewings WHERE watch_year IS NOT NULL ORDER BY v DESC")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v sql.NullInt64
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return nil, err
		}
		if v.Valid {
			out.WatchYears = append(out.WatchYears, v.Int64)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// releaseYears
	rows, err = d.db.Query("SELECT DISTINCT release_year AS v FROM films WHERE release_year IS NOT NULL ORDER BY v DESC")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v sql.NullInt64
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return nil, err
		}
		if v.Valid {
			out.ReleaseYears = append(out.ReleaseYears, v.Int64)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// categories
	rows, err = d.db.Query("SELECT DISTINCT category AS v FROM films WHERE category IS NOT NULL ORDER BY v")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return nil, err
		}
		out.Categories = append(out.Categories, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// platforms（观看记录的平台）
	rows, err = d.db.Query("SELECT platforms_raw FROM viewings WHERE platforms_raw IS NOT NULL")
	if err != nil {
		return nil, err
	}
	platSet := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return nil, err
		}
		for _, p := range strings.Split(v, ",") {
			s := strings.TrimSpace(p)
			if s != "" {
				platSet[s] = true
			}
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	platforms := make([]string, 0, len(platSet))
	for k := range platSet {
		platforms = append(platforms, k)
	}
	// 按 zh-Hans 排序（对应 JS localeCompare('zh-Hans')）
	coll := collate.New(language.SimplifiedChinese)
	coll.SortStrings(platforms)
	out.Platforms = platforms

	return out, nil
}

// Stats 概览统计（GET /api/stats）：以观看记录为计数单位（与列表展示一致）。
func (d *DB) Stats() (*Stats, error) {
	out := &Stats{
		ByCategory:  []CatStat{},
		ByWatchYear: []YearStat{},
	}
	if err := d.db.QueryRow("SELECT COUNT(*) FROM viewings").Scan(&out.Total); err != nil {
		return nil, err
	}
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM viewings v
		JOIN films f ON f.id = v.film_id
		LEFT JOIN film_metadata m ON m.film_id = f.id WHERE m.tmdb_id IS NOT NULL`).Scan(&out.WithMetadata); err != nil {
		return nil, err
	}
	out.WithoutMetadata = out.Total - out.WithMetadata
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM viewings v
		JOIN films f ON f.id = v.film_id WHERE f.imdb_id IS NULL OR TRIM(f.imdb_id) = ''`).Scan(&out.WithoutImdb); err != nil {
		return nil, err
	}
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM viewings v
		JOIN films f ON f.id = v.film_id WHERE f.douban_id IS NULL OR TRIM(f.douban_id) = ''`).Scan(&out.WithoutDouban); err != nil {
		return nil, err
	}

	rows, err := d.db.Query(`SELECT f.category AS k, COUNT(*) AS c
		FROM viewings v JOIN films f ON f.id = v.film_id GROUP BY f.category ORDER BY c DESC`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var k sql.NullString
		var c int64
		if err := rows.Scan(&k, &c); err != nil {
			rows.Close()
			return nil, err
		}
		var kp *string
		if k.Valid {
			s := k.String
			kp = &s
		}
		out.ByCategory = append(out.ByCategory, CatStat{K: kp, C: c})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = d.db.Query("SELECT watch_year AS k, COUNT(*) AS c FROM viewings WHERE watch_year IS NOT NULL GROUP BY watch_year ORDER BY k")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var k int64
		var c int64
		if err := rows.Scan(&k, &c); err != nil {
			rows.Close()
			return nil, err
		}
		kk := k
		out.ByWatchYear = append(out.ByWatchYear, YearStat{K: &kk, C: c})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// FilmExists 判断影视是否存在。
func (d *DB) FilmExists(id int64) (bool, error) {
	var one int
	err := d.db.QueryRow("SELECT 1 FROM films WHERE id = ?", id).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ViewingExists 判断观看记录是否存在。
func (d *DB) ViewingExists(id int64) (bool, error) {
	var one int
	err := d.db.QueryRow("SELECT 1 FROM viewings WHERE id = ?", id).Scan(&one)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetFilmBasic 取影片基础信息（保存元数据时用）。
func (d *DB) GetFilmBasic(id int64) (*FilmBasic, error) {
	var b FilmBasic
	var name sql.NullString
	var category, imdbID sql.NullString
	err := d.db.QueryRow("SELECT id, name, category, imdb_id FROM films WHERE id = ?", id).Scan(&b.ID, &name, &category, &imdbID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if name.Valid {
		b.Name = name.String
	}
	if category.Valid {
		b.Category = category.String
	}
	if imdbID.Valid {
		b.ImdbID = imdbID.String
	}
	return &b, nil
}

// GetExistingLocals 取某影片已有的本地图片文件名。
func (d *DB) GetExistingLocals(filmID int64) (ImageLocals, error) {
	var poster, backdrop sql.NullString
	err := d.db.QueryRow("SELECT poster_local, backdrop_local FROM film_metadata WHERE film_id = ?", filmID).Scan(&poster, &backdrop)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ImageLocals{}, nil
		}
		return ImageLocals{}, err
	}
	out := ImageLocals{}
	if poster.Valid {
		s := poster.String
		out.Poster = &s
	}
	if backdrop.Valid {
		s := backdrop.String
		out.Backdrop = &s
	}
	return out, nil
}

// GetMetaPaths 取某影片元数据中的 TMDB 远程图片路径。
func (d *DB) GetMetaPaths(filmID int64) (MetaPaths, error) {
	var poster, backdrop sql.NullString
	err := d.db.QueryRow("SELECT poster_path, backdrop_path FROM film_metadata WHERE film_id = ?", filmID).Scan(&poster, &backdrop)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MetaPaths{}, nil
		}
		return MetaPaths{}, err
	}
	out := MetaPaths{}
	if poster.Valid {
		s := poster.String
		out.PosterPath = &s
	}
	if backdrop.Valid {
		s := backdrop.String
		out.BackdropPath = &s
	}
	return out, nil
}

// GetMetaLocal 取某影片指定类型的本地图片文件名。
func (d *DB) GetMetaLocal(filmID int64, imgType string) (*string, error) {
	col, ok := imgLocalColumn(imgType)
	if !ok {
		return nil, fmt.Errorf("invalid image type: %s", imgType)
	}
	q := fmt.Sprintf("SELECT %s AS f FROM film_metadata WHERE film_id = ?", col)
	var s sql.NullString
	err := d.db.QueryRow(q, filmID).Scan(&s)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if !s.Valid {
		return nil, nil
	}
	v := s.String
	return &v, nil
}

// ListFilmsForScrape 列出待刮削的影片（imdb_id 非空；force=false 时排除已有元数据；onlyID 限定单条）。
func (d *DB) ListFilmsForScrape(force bool, onlyID *int64) ([]FilmBasic, error) {
	where := []string{"imdb_id IS NOT NULL"}
	args := []interface{}{}
	if !force {
		where = append(where, "id NOT IN (SELECT film_id FROM film_metadata)")
	}
	if onlyID != nil {
		where = append(where, "id = ?")
		args = append(args, *onlyID)
	}
	q := "SELECT id, name, category, imdb_id FROM films WHERE " + strings.Join(where, " AND ") + " ORDER BY id"
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FilmBasic{}
	for rows.Next() {
		var b FilmBasic
		var name, category, imdbID sql.NullString
		if err := rows.Scan(&b.ID, &name, &category, &imdbID); err != nil {
			return nil, err
		}
		if name.Valid {
			b.Name = name.String
		}
		if category.Valid {
			b.Category = category.String
		}
		if imdbID.Valid {
			b.ImdbID = imdbID.String
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListFilmRefsForRatings 评分刷新用的影片引用列表（同一影视去重，仅含有观看记录的影视）。
func (d *DB) ListFilmRefsForRatings(f Filter) ([]FilmRef, error) {
	where, args := d.buildWhere(f)
	q := "SELECT DISTINCT f.id, f.name, m.tmdb_id AS tmdb, m.media_type AS mt FROM films f JOIN viewings v ON v.film_id = f.id LEFT JOIN film_metadata m ON m.film_id = f.id"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FilmRef{}
	for rows.Next() {
		var r FilmRef
		var tmdb sql.NullInt64
		var mt sql.NullString
		if err := rows.Scan(&r.ID, &r.Name, &tmdb, &mt); err != nil {
			return nil, err
		}
		if tmdb.Valid {
			v := tmdb.Int64
			r.TmdbID = &v
		}
		if mt.Valid {
			r.MediaType = mt.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- 写入 ---

// CreateFilm 新增观影记录（POST /api/films）：按 豆瓣 > IMDb > 名称（忽略大小写）匹配
// 已有影视（与迁移/导入的去重判定一致），命中且无 ID 冲突时追加一条观看记录并回填
// 影视级缺失字段，否则新建影视。返回影视 id。
func (d *DB) CreateFilm(body map[string]interface{}) (filmID int64, err error) {
	nameStr := ""
	if v, ok := body["name"].(string); ok {
		nameStr = v
	}
	nameStr = strings.TrimSpace(nameStr)
	if nameStr == "" {
		return 0, fmt.Errorf("名称不能为空")
	}

	tx, err := d.db.Begin()
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	newImdb, _ := body["imdb_id"].(string)
	newImdb = strings.TrimSpace(newImdb)
	newDouban, _ := body["douban_id"].(string)
	newDouban = strings.TrimSpace(newDouban)

	// 匹配已有影视：豆瓣 > IMDb > 名称。
	// 豆瓣号命中即同一影视；按 IMDb / 名称匹配时任一 ID 冲突（双方均非空且不同）
	// 视为不同影视（多季综艺各季豆瓣条目不同，据此保持独立）。
	var existingID sql.NullInt64
	var existingImdb, existingDouban sql.NullString

	if newDouban != "" {
		if err = tx.QueryRow("SELECT id FROM films WHERE TRIM(douban_id) = TRIM(?)", newDouban).Scan(&existingID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
		err = nil
	}
	if !existingID.Valid && newImdb != "" {
		if err = tx.QueryRow("SELECT id, douban_id FROM films WHERE TRIM(imdb_id) = TRIM(?) COLLATE NOCASE", newImdb).Scan(&existingID, &existingDouban); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
		err = nil
		if existingID.Valid && newDouban != "" && existingDouban.Valid && existingDouban.String != "" && newDouban != existingDouban.String {
			existingID = sql.NullInt64{} // 豆瓣冲突：不同影视
		}
	}
	if !existingID.Valid {
		if err = tx.QueryRow("SELECT id, imdb_id, douban_id FROM films WHERE TRIM(name) = TRIM(?) COLLATE NOCASE", nameStr).Scan(&existingID, &existingImdb, &existingDouban); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return 0, err
		}
		err = nil
		imdbConflict := existingID.Valid && newImdb != "" && existingImdb.Valid && existingImdb.String != "" && newImdb != existingImdb.String
		doubanConflict := existingID.Valid && newDouban != "" && existingDouban.Valid && existingDouban.String != "" && newDouban != existingDouban.String
		if imdbConflict || doubanConflict {
			existingID = sql.NullInt64{}
		}
	}

	if existingID.Valid {
		// 复用已有影视：回填影视级缺失字段（原值为 NULL 且新值非空时生效）
		sets := []string{}
		args := []interface{}{}
		for _, k := range filmFieldWhiteList {
			if k == "name" {
				continue
			}
			v, ok := body[k]
			if !ok || v == nil {
				continue
			}
			sets = append(sets, fmt.Sprintf("%s = COALESCE(%s, ?)", k, k))
			args = append(args, filmValue(k, v))
		}
		if len(sets) > 0 {
			args = append(args, existingID.Int64)
			if _, err = tx.Exec("UPDATE films SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...); err != nil {
				return 0, err
			}
		}
		filmID = existingID.Int64
	} else {
		// 新建影视
		cols := []string{"name"}
		args := []interface{}{nameStr}
		for _, k := range filmFieldWhiteList {
			if k == "name" {
				continue
			}
			if v, ok := body[k]; ok {
				cols = append(cols, k)
				args = append(args, filmValue(k, v))
			}
		}
		placeholders := make([]string, len(cols))
		for i := range placeholders {
			placeholders[i] = "?"
		}
		var res sql.Result
		if res, err = tx.Exec("INSERT INTO films ("+strings.Join(cols, ", ")+") VALUES ("+strings.Join(placeholders, ", ")+")", args...); err != nil {
			return 0, err
		}
		if filmID, err = res.LastInsertId(); err != nil {
			return 0, err
		}
	}

	// 插入观看记录
	vcols := []string{"film_id"}
	vargs := []interface{}{filmID}
	for _, k := range viewingFieldWhiteList {
		if v, ok := body[k]; ok {
			vcols = append(vcols, k)
			vargs = append(vargs, viewingValue(k, v))
		}
	}
	vph := make([]string, len(vcols))
	for i := range vph {
		vph[i] = "?"
	}
	if _, err = tx.Exec("INSERT INTO viewings ("+strings.Join(vcols, ", ")+") VALUES ("+strings.Join(vph, ", ")+")", vargs...); err != nil {
		return 0, err
	}

	err = tx.Commit()
	return filmID, err
}

// UpdateFilm 编辑影视信息（films 表字段），返回是否有字段更新（PUT /api/films/:id）。
func (d *DB) UpdateFilm(id int64, fields map[string]interface{}) (bool, error) {
	sets := []string{}
	args := []interface{}{}
	for _, k := range filmFieldWhiteList {
		v, ok := fields[k]
		if !ok {
			continue
		}
		sets = append(sets, fmt.Sprintf("%s = ?", k))
		args = append(args, filmValue(k, v))
	}
	if len(sets) == 0 {
		return false, nil
	}
	args = append(args, id)
	q := "UPDATE films SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	if _, err := d.db.Exec(q, args...); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateViewing 编辑观看记录（viewings 表字段），返回是否有字段更新（PUT /api/viewings/:id）。
func (d *DB) UpdateViewing(id int64, fields map[string]interface{}) (bool, error) {
	sets := []string{}
	args := []interface{}{}
	for _, k := range viewingFieldWhiteList {
		v, ok := fields[k]
		if !ok {
			continue
		}
		sets = append(sets, fmt.Sprintf("%s = ?", k))
		args = append(args, viewingValue(k, v))
	}
	if len(sets) == 0 {
		return false, nil
	}
	args = append(args, id)
	q := "UPDATE viewings SET " + strings.Join(sets, ", ") + " WHERE id = ?"
	if _, err := d.db.Exec(q, args...); err != nil {
		return false, err
	}
	return true, nil
}

// UpsertMetadata 插入或替换元数据（POST /api/films/:id/metadata、批量刮削）。
func (d *DB) UpsertMetadata(filmID int64, meta *model.Metadata, posterLocal, backdropLocal *string) error {
	const q = `INSERT OR REPLACE INTO film_metadata
    (film_id, imdb_id, tmdb_id, media_type, title, original_title, overview,
     poster_path, backdrop_path, poster_local, backdrop_local,
     genres, production_countries, runtime, vote_average, vote_count,
     directors, cast, release_date, status, tagline,
     original_language, spoken_languages, origin_country, production_companies,
     writers, cinematographers, composers, producers, keywords,
     number_of_seasons, number_of_episodes, budget, revenue,
     content_rating, homepage, updated_at)
    VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`
	args := []interface{}{
		filmID,
		nullIfEmpty(meta.ImdbID),
		meta.TmdbID,
		meta.MediaType,
		meta.Title,
		meta.OriginalTitle,
		nullIfEmpty(meta.Overview),
		nullIfEmpty(meta.PosterPath),
		nullIfEmpty(meta.BackdropPath),
		valStr(posterLocal),
		valStr(backdropLocal),
		meta.Genres,
		meta.ProductionCountries,
		valInt64(meta.Runtime),
		valFloat64(meta.VoteAverage),
		valInt64(meta.VoteCount),
		meta.Directors,
		meta.Cast,
		nullIfEmpty(meta.ReleaseDate),
		nullIfEmpty(meta.Status),
		nullIfEmpty(meta.Tagline),
		nullIfEmpty(meta.OriginalLanguage),
		meta.SpokenLanguages,
		meta.OriginCountry,
		meta.ProductionCompanies,
		meta.Writers,
		meta.Cinematographers,
		meta.Composers,
		meta.Producers,
		meta.Keywords,
		valInt64(meta.NumberOfSeasons),
		valInt64(meta.NumberOfEpisodes),
		valInt64(meta.Budget),
		valInt64(meta.Revenue),
		nullIfEmpty(meta.ContentRating),
		nullIfEmpty(meta.Homepage),
		meta.UpdatedAt,
	}
	_, err := d.db.Exec(q, args...)
	return err
}

// UpdateImdbID 回填 films.imdb_id（仅当原值为空时由调用方判断）。
func (d *DB) UpdateImdbID(filmID int64, imdbID string) error {
	_, err := d.db.Exec("UPDATE films SET imdb_id = ? WHERE id = ?", imdbID, filmID)
	return err
}

// EnsureMetaRow 若该影片尚无元数据行则插入空行。
func (d *DB) EnsureMetaRow(filmID int64) error {
	var id int64
	err := d.db.QueryRow("SELECT film_id FROM film_metadata WHERE film_id = ?", filmID).Scan(&id)
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		_, err := d.db.Exec("INSERT INTO film_metadata (film_id) VALUES (?)", filmID)
		return err
	}
	return err
}

// UpdateMetadataFields 编辑元数据字段，返回是否有字段更新（PUT /api/films/:id/metadata）。
func (d *DB) UpdateMetadataFields(filmID int64, fields map[string]interface{}) (bool, error) {
	sets := []string{}
	args := []interface{}{}
	for _, k := range metaFieldWhiteList {
		v, ok := fields[k]
		if !ok {
			continue
		}
		var val interface{}
		switch {
		case metaJSONFields[k]:
			val = toArrayJSON(v)
		case metaIntFields[k]:
			val = toInt64(v)
		case metaFloatFields[k]:
			val = toFloat64(v)
		default:
			val = toText(v)
		}
		sets = append(sets, fmt.Sprintf("%s = ?", k))
		args = append(args, val)
	}
	if len(sets) == 0 {
		return false, nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, nowISO())
	args = append(args, filmID)
	if err := d.EnsureMetaRow(filmID); err != nil {
		return false, err
	}
	q := "UPDATE film_metadata SET " + strings.Join(sets, ", ") + " WHERE film_id = ?"
	if _, err := d.db.Exec(q, args...); err != nil {
		return false, err
	}
	return true, nil
}

// SetImage 设置某类型本地图片（确保元数据行存在后更新）。
func (d *DB) SetImage(filmID int64, imgType, file string) error {
	col, ok := imgLocalColumn(imgType)
	if !ok {
		return fmt.Errorf("invalid image type: %s", imgType)
	}
	if err := d.EnsureMetaRow(filmID); err != nil {
		return err
	}
	q := fmt.Sprintf("UPDATE film_metadata SET %s = ?, updated_at = ? WHERE film_id = ?", col)
	_, err := d.db.Exec(q, file, nowISO(), filmID)
	return err
}

// ClearImageLocal 清空某类型本地图片（DELETE /api/films/:id/image）。
func (d *DB) ClearImageLocal(filmID int64, imgType string) error {
	col, ok := imgLocalColumn(imgType)
	if !ok {
		return fmt.Errorf("invalid image type: %s", imgType)
	}
	q := fmt.Sprintf("UPDATE film_metadata SET %s = NULL WHERE film_id = ?", col)
	_, err := d.db.Exec(q, filmID)
	return err
}

// DeleteMetadata 删除元数据行，返回原本地图片文件名供调用方删图。
func (d *DB) DeleteMetadata(filmID int64) (ImageLocals, error) {
	locals, err := d.GetExistingLocals(filmID)
	if err != nil {
		return ImageLocals{}, err
	}
	_, err = d.db.Exec("DELETE FROM film_metadata WHERE film_id = ?", filmID)
	if err != nil {
		return ImageLocals{}, err
	}
	return locals, nil
}

// DeleteFilm 删除整部影视（含全部观看记录与元数据行），返回原本地图片文件名供删图。
func (d *DB) DeleteFilm(filmID int64) (ImageLocals, error) {
	locals, err := d.GetExistingLocals(filmID)
	if err != nil {
		return ImageLocals{}, err
	}
	if _, err := d.db.Exec("DELETE FROM film_metadata WHERE film_id = ?", filmID); err != nil {
		return ImageLocals{}, err
	}
	if _, err := d.db.Exec("DELETE FROM viewings WHERE film_id = ?", filmID); err != nil {
		return ImageLocals{}, err
	}
	if _, err := d.db.Exec("DELETE FROM films WHERE id = ?", filmID); err != nil {
		return ImageLocals{}, err
	}
	return locals, nil
}

// DeleteViewing 删除单条观看记录（DELETE /api/viewings/:id）。
// 若为该影视最后一条，则连同影视与元数据一并删除；
// 返回（是否删除了整个影视, 待删除的本地图片）。found=false 表示记录不存在。
func (d *DB) DeleteViewing(id int64) (found, filmDeleted bool, locals ImageLocals, err error) {
	tx, err := d.db.Begin()
	if err != nil {
		return false, false, ImageLocals{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	var filmID int64
	err = tx.QueryRow("SELECT film_id FROM viewings WHERE id = ?", id).Scan(&filmID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, false, ImageLocals{}, nil
		}
		return false, false, ImageLocals{}, err
	}

	if _, err = tx.Exec("DELETE FROM viewings WHERE id = ?", id); err != nil {
		return false, false, ImageLocals{}, err
	}

	var remaining int64
	if err = tx.QueryRow("SELECT COUNT(*) FROM viewings WHERE film_id = ?", filmID).Scan(&remaining); err != nil {
		return false, false, ImageLocals{}, err
	}

	if remaining == 0 {
		// 最后一条观看记录被删除：连同影视与元数据一并删除
		var poster, backdrop sql.NullString
		if err = tx.QueryRow("SELECT poster_local, backdrop_local FROM film_metadata WHERE film_id = ?", filmID).Scan(&poster, &backdrop); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return false, false, ImageLocals{}, err
		}
		err = nil
		if poster.Valid {
			s := poster.String
			locals.Poster = &s
		}
		if backdrop.Valid {
			s := backdrop.String
			locals.Backdrop = &s
		}
		if _, err = tx.Exec("DELETE FROM film_metadata WHERE film_id = ?", filmID); err != nil {
			return false, false, ImageLocals{}, err
		}
		if _, err = tx.Exec("DELETE FROM films WHERE id = ?", filmID); err != nil {
			return false, false, ImageLocals{}, err
		}
		if err = tx.Commit(); err != nil {
			return false, false, ImageLocals{}, err
		}
		return true, true, locals, nil
	}

	if err = tx.Commit(); err != nil {
		return false, false, ImageLocals{}, err
	}
	return true, false, ImageLocals{}, nil
}

// UpdateRatings 刷新评分（POST /api/ratings/refresh 内单条更新）。
func (d *DB) UpdateRatings(filmID int64, voteAverage *float64, voteCount *int64, updatedAt string) error {
	_, err := d.db.Exec("UPDATE film_metadata SET vote_average = ?, vote_count = ?, updated_at = ? WHERE film_id = ?",
		valFloat64(voteAverage), valInt64(voteCount), updatedAt, filmID)
	return err
}
