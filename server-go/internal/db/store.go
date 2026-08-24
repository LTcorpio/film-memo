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

// --- 字段白名单与类型分类 ---

var filmFieldWhiteList = []string{
	"watch_year", "category", "name", "imdb_id", "douban_id", "production_countries_raw",
	"release_year", "start_date", "end_date", "total_episodes", "platforms_raw", "location", "notes",
}

var filmIntFields = map[string]bool{
	"watch_year": true, "release_year": true, "total_episodes": true,
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

// ListFilms 列表查询（GET /api/films）。
func (d *DB) ListFilms(f Filter) ([]model.FilmRow, error) {
	where, args := d.buildWhere(f)
	q := "SELECT " + model.FilmsCols + ", " + model.MetaCols +
		" FROM films f LEFT JOIN film_metadata m ON m.film_id = f.id"
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY f.watch_year DESC, f.start_date DESC NULLS LAST, f.id DESC"

	rows, err := d.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.FilmRow{}
	for rows.Next() {
		var r model.FilmRow
		if err := rows.Scan(r.ScanPtrs()...); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetFilm 单条详情（GET /api/films/:id）。
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

// Filters 筛选项（GET /api/filters）。
func (d *DB) Filters() (*Filters, error) {
	out := &Filters{
		WatchYears:   []int64{},
		ReleaseYears: []int64{},
		Categories:   []string{},
		Platforms:    []string{},
	}

	// watchYears
	rows, err := d.db.Query("SELECT DISTINCT watch_year AS v FROM films WHERE watch_year IS NOT NULL ORDER BY v DESC")
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

	// platforms
	rows, err = d.db.Query("SELECT platforms_raw FROM films WHERE platforms_raw IS NOT NULL")
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

// Stats 概览统计（GET /api/stats）。
func (d *DB) Stats() (*Stats, error) {
	out := &Stats{
		ByCategory:  []CatStat{},
		ByWatchYear: []YearStat{},
	}
	if err := d.db.QueryRow("SELECT COUNT(*) FROM films").Scan(&out.Total); err != nil {
		return nil, err
	}
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM films f
		LEFT JOIN film_metadata m ON m.film_id = f.id WHERE m.tmdb_id IS NOT NULL`).Scan(&out.WithMetadata); err != nil {
		return nil, err
	}
	out.WithoutMetadata = out.Total - out.WithMetadata
	if err := d.db.QueryRow("SELECT COUNT(*) FROM films WHERE imdb_id IS NULL OR TRIM(imdb_id) = ''").Scan(&out.WithoutImdb); err != nil {
		return nil, err
	}
	if err := d.db.QueryRow("SELECT COUNT(*) FROM films WHERE douban_id IS NULL OR TRIM(douban_id) = ''").Scan(&out.WithoutDouban); err != nil {
		return nil, err
	}

	rows, err := d.db.Query("SELECT category AS k, COUNT(*) AS c FROM films GROUP BY category ORDER BY c DESC")
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

	rows, err = d.db.Query("SELECT watch_year AS k, COUNT(*) AS c FROM films WHERE watch_year IS NOT NULL GROUP BY watch_year ORDER BY k")
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

// FilmExists 判断影片是否存在。
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

// ListFilmRefsForRatings 评分刷新用的影片引用列表。
func (d *DB) ListFilmRefsForRatings(f Filter) ([]FilmRef, error) {
	where, args := d.buildWhere(f)
	q := "SELECT f.id, f.name, m.tmdb_id AS tmdb, m.media_type AS mt FROM films f LEFT JOIN film_metadata m ON m.film_id = f.id"
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

// InsertFilm 新增观影记录，返回新 id（POST /api/films）。
func (d *DB) InsertFilm(fields map[string]interface{}) (int64, error) {
	cols := []string{}
	args := []interface{}{}
	for _, k := range filmFieldWhiteList {
		v, ok := fields[k]
		if !ok {
			continue
		}
		var val interface{}
		if filmIntFields[k] {
			val = toInt64(v)
		} else {
			val = toText(v)
		}
		cols = append(cols, k)
		args = append(args, val)
	}
	if len(cols) == 0 {
		return 0, fmt.Errorf("无有效字段")
	}
	placeholders := make([]string, len(cols))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	q := "INSERT INTO films (" + strings.Join(cols, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"
	res, err := d.db.Exec(q, args...)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateFilm 编辑观看记录，返回是否有字段更新（PUT /api/films/:id）。
func (d *DB) UpdateFilm(id int64, fields map[string]interface{}) (bool, error) {
	sets := []string{}
	args := []interface{}{}
	for _, k := range filmFieldWhiteList {
		v, ok := fields[k]
		if !ok {
			continue
		}
		var val interface{}
		if filmIntFields[k] {
			val = toInt64(v)
		} else {
			val = toText(v)
		}
		sets = append(sets, fmt.Sprintf("%s = ?", k))
		args = append(args, val)
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

// DeleteFilm 删除整条观影记录（含元数据行），返回原本地图片文件名供删图。
func (d *DB) DeleteFilm(filmID int64) (ImageLocals, error) {
	locals, err := d.GetExistingLocals(filmID)
	if err != nil {
		return ImageLocals{}, err
	}
	if _, err := d.db.Exec("DELETE FROM film_metadata WHERE film_id = ?", filmID); err != nil {
		return ImageLocals{}, err
	}
	if _, err := d.db.Exec("DELETE FROM films WHERE id = ?", filmID); err != nil {
		return ImageLocals{}, err
	}
	return locals, nil
}

// UpdateRatings 刷新评分（POST /api/ratings/refresh 内单条更新）。
func (d *DB) UpdateRatings(filmID int64, voteAverage *float64, voteCount *int64, updatedAt string) error {
	_, err := d.db.Exec("UPDATE film_metadata SET vote_average = ?, vote_count = ?, updated_at = ? WHERE film_id = ?",
		valFloat64(voteAverage), valInt64(voteCount), updatedAt, filmID)
	return err
}
