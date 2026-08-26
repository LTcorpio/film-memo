// Package db 封装 SQLite 连接、建表、迁移与全部数据访问。
// 对应 JS 版 db.js（连接/建表/迁移）与 index.js 中的全部 DB 查询。
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动
)

// DB 包装 *sql.DB，承载所有数据访问方法。
type DB struct {
	db        *sql.DB
	imagesDir string // 迁移去重时删除被合并影片的本地图片
}

// schema 对应 db.js 的 SCHEMA（CREATE TABLE + INDEX）。
// 数据模型 v2：
//
//	films      影视级数据，同一影视仅存一份（名称/类别/上映年份/总集数/IMDb/豆瓣/制片国家）
//	viewings   观看记录，同一影视可有多条（观看年份/起止日期/平台/地点/备注）
//	film_metadata TMDB 元数据，挂在 films.id 上，同一影视共享一份
const schema = `
CREATE TABLE IF NOT EXISTS films (
  id                  INTEGER PRIMARY KEY,        -- 序
  category            TEXT,                       -- 类别
  name                TEXT NOT NULL,              -- 名称
  imdb_id             TEXT,                       -- IMDb 号（已规范化，缺失为 NULL）
  douban_id           TEXT,                       -- 豆瓣条目 ID（用户手动填写，同一影视判定的最高优先级键）
  production_countries_raw TEXT,                  -- 制片国家原始字段（按 "/" 分割）
  release_year        INTEGER,                    -- 上映年份
  total_episodes      INTEGER                     -- 总集/期数
);

CREATE TABLE IF NOT EXISTS viewings (
  id                  INTEGER PRIMARY KEY,        -- 观看记录 id
  film_id             INTEGER NOT NULL,           -- 关联 films.id
  watch_year          INTEGER,                    -- 观看年份
  start_date          TEXT,                       -- 开始观看日期 (ISO YYYY-MM-DD)
  end_date            TEXT,                       -- 结束观看日期
  platforms_raw       TEXT,                       -- 观看平台原始字段（按 "," 分割）
  location            TEXT,                       -- 观看地点
  notes               TEXT,                       -- 备注
  FOREIGN KEY (film_id) REFERENCES films(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS film_metadata (
  film_id             INTEGER PRIMARY KEY,
  imdb_id             TEXT,
  tmdb_id             INTEGER,
  media_type          TEXT,
  title               TEXT,
  original_title      TEXT,
  overview            TEXT,
  poster_path         TEXT,
  backdrop_path       TEXT,
  poster_local        TEXT,
  backdrop_local      TEXT,
  genres              TEXT,
  production_countries TEXT,
  runtime             INTEGER,
  vote_average        REAL,
  vote_count          INTEGER,
  directors           TEXT,
  cast                TEXT,
  release_date        TEXT,
  status              TEXT,
  tagline             TEXT,
  original_language   TEXT,
  spoken_languages    TEXT,
  origin_country      TEXT,
  production_companies TEXT,
  writers             TEXT,
  cinematographers    TEXT,
  composers           TEXT,
  producers           TEXT,
  keywords            TEXT,
  number_of_seasons   INTEGER,
  number_of_episodes  INTEGER,
  budget              INTEGER,
  revenue             INTEGER,
  content_rating      TEXT,
  homepage            TEXT,
  updated_at          TEXT
);
CREATE INDEX IF NOT EXISTS idx_meta_imdb ON film_metadata(imdb_id);
CREATE INDEX IF NOT EXISTS idx_viewings_film ON viewings(film_id);
CREATE INDEX IF NOT EXISTS idx_viewings_watch_year ON viewings(watch_year);
CREATE INDEX IF NOT EXISTS idx_films_release_year ON films(release_year);
CREATE INDEX IF NOT EXISTS idx_films_category ON films(category);
`

// metadataMigrateCols 是 film_metadata 需要兼容旧库补齐的列（顺序与 JS 一致）。
var metadataMigrateCols = []struct {
	name string
	typ  string
}{
	{"poster_local", "TEXT"}, {"backdrop_local", "TEXT"}, {"directors", "TEXT"},
	{"cast", "TEXT"}, {"release_date", "TEXT"}, {"status", "TEXT"}, {"tagline", "TEXT"},
	{"original_language", "TEXT"}, {"spoken_languages", "TEXT"}, {"origin_country", "TEXT"},
	{"production_companies", "TEXT"}, {"writers", "TEXT"}, {"cinematographers", "TEXT"},
	{"composers", "TEXT"}, {"producers", "TEXT"}, {"keywords", "TEXT"},
	{"number_of_seasons", "INTEGER"}, {"number_of_episodes", "INTEGER"},
	{"budget", "INTEGER"}, {"revenue", "INTEGER"},
	{"content_rating", "TEXT"}, {"homepage", "TEXT"},
}

// watchCols 是旧 films 表内嵌的观看字段（v1 模型），存在则触发向 v2 迁移。
var watchCols = []string{"watch_year", "start_date", "end_date", "platforms_raw", "location", "notes"}

// Open 打开数据库并完成建表与旧库迁移。单连接模式以避免写锁冲突（与 better-sqlite3 同步语义一致）。
// imagesDir 用于迁移去重时删除被合并影片的本地图片文件。
func Open(path, imagesDir string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// 单连接：避免 WAL 下的写锁竞争（个人应用，无需并发）
	conn.SetMaxOpenConns(1)
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, err
	}
	if _, err := conn.Exec("PRAGMA busy_timeout=5000"); err != nil {
		conn.Close()
		return nil, err
	}
	d := &DB{db: conn, imagesDir: imagesDir}
	if err := d.migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return d, nil
}

// Close 关闭连接。
func (d *DB) Close() error { return d.db.Close() }

// DB 暴露底层 *sql.DB，供命令行脚本（Excel 导入事务等）使用。
func (d *DB) DB() *sql.DB { return d.db }

// migrate 建表并兼容旧库结构（对应 db.js 的迁移逻辑）。
func (d *DB) migrate() error {
	if _, err := d.db.Exec(schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}

	// 兼容旧库：补齐 film_metadata 新增列
	metaCols, err := d.columns("film_metadata")
	if err != nil {
		return err
	}
	for _, c := range metadataMigrateCols {
		if !metaCols[c.name] {
			if _, err := d.db.Exec(fmt.Sprintf("ALTER TABLE film_metadata ADD COLUMN %s %s;", c.name, c.typ)); err != nil {
				return fmt.Errorf("add column %s: %w", c.name, err)
			}
		}
	}

	// 兼容旧库：补齐 films.douban_id
	filmCols, err := d.columns("films")
	if err != nil {
		return err
	}
	if !filmCols["douban_id"] {
		if _, err := d.db.Exec("ALTER TABLE films ADD COLUMN douban_id TEXT;"); err != nil {
			return fmt.Errorf("add films.douban_id: %w", err)
		}
	}

	// 清洗数值列中的脏数据（须在 v1→v2 迁移前执行，保证旧观看字段可被扫描）。
	// 旧 JS 后端可能把空字符串写入 INTEGER/REAL 列（SQLite 动态类型不报错），
	// Go 驱动严格类型会导致 Scan 失败；空白文本置 NULL，纯数字文本转为数值。
	// 仅清洗实际存在的列（films 的旧观看字段在迁移后不存在）。
	if err := d.cleanNumericCols(); err != nil {
		return err
	}

	// 若旧库 film_metadata 仍存在 douban_id 列：回填 films.douban_id 后删除该列
	metaCols, err = d.columns("film_metadata")
	if err != nil {
		return err
	}
	if metaCols["douban_id"] {
		if _, err := d.db.Exec(`
			UPDATE films SET douban_id = (
				SELECT m.douban_id FROM film_metadata m WHERE m.film_id = films.id
			) WHERE douban_id IS NULL;`); err != nil {
			return fmt.Errorf("backfill douban_id: %w", err)
		}
		if _, err := d.db.Exec("ALTER TABLE film_metadata DROP COLUMN douban_id;"); err != nil {
			return fmt.Errorf("drop film_metadata.douban_id: %w", err)
		}
	}

	// 旧模型（观看字段内嵌 films）→ 新模型（films + viewings）迁移
	if err := d.migrateToViewings(); err != nil {
		return err
	}

	// 迁移后清洗 viewings.watch_year（库可能已被 JS v2 后端迁移过而未清洗）
	return d.cleanNumericCol("viewings", "watch_year", "INTEGER")
}

// cleanNumericCol 清洗单个数值列中的脏文本（空串→NULL，纯数字文本→数值）。
func (d *DB) cleanNumericCol(table, col, typ string) error {
	if _, err := d.db.Exec(fmt.Sprintf(
		"UPDATE %s SET %s = NULL WHERE typeof(%s) = 'text' AND trim(%s) = ''",
		table, col, col, col)); err != nil {
		return fmt.Errorf("clean empty text in %s.%s: %w", table, col, err)
	}
	if _, err := d.db.Exec(fmt.Sprintf(
		"UPDATE %s SET %s = CAST(%s AS %s) WHERE typeof(%s) = 'text' AND trim(%s) GLOB '[-+0-9.eE]*'",
		table, col, col, typ, col, col)); err != nil {
		return fmt.Errorf("cast numeric text in %s.%s: %w", table, col, err)
	}
	return nil
}

// cleanNumericCols 清洗全部数值列（列不存在时跳过）。
func (d *DB) cleanNumericCols() error {
	cols := []struct{ table, col, typ string }{
		{"films", "watch_year", "INTEGER"},
		{"films", "release_year", "INTEGER"},
		{"films", "total_episodes", "INTEGER"},
		{"film_metadata", "tmdb_id", "INTEGER"},
		{"film_metadata", "runtime", "INTEGER"},
		{"film_metadata", "vote_average", "REAL"},
		{"film_metadata", "vote_count", "INTEGER"},
		{"film_metadata", "number_of_seasons", "INTEGER"},
		{"film_metadata", "number_of_episodes", "INTEGER"},
		{"film_metadata", "budget", "INTEGER"},
		{"film_metadata", "revenue", "INTEGER"},
	}
	tableCols := map[string]map[string]bool{}
	for _, c := range cols {
		tc, ok := tableCols[c.table]
		if !ok {
			var err error
			tc, err = d.columns(c.table)
			if err != nil {
				return err
			}
			tableCols[c.table] = tc
		}
		if !tc[c.col] {
			continue
		}
		if err := d.cleanNumericCol(c.table, c.col, c.typ); err != nil {
			return err
		}
	}
	return nil
}

// oldFilmRow 迁移时读取的旧 films 行（含元数据标识列）。
type oldFilmRow struct {
	id                     int64
	watchYear              *int64
	startDate              *string
	endDate                *string
	platformsRaw           *string
	location               *string
	notes                  *string
	category               *string
	name                   *string
	imdbID                 *string
	doubanID               *string
	productionCountriesRaw *string
	releaseYear            *int64
	totalEpisodes          *int64
	mTmdbID                *int64
	mImdbID                *string
}

// migrateToViewings 旧模型 → 新模型迁移（幂等，对应 db.js migrateToViewings）：
// 检测到旧 films 表含观看字段时，按 豆瓣 > IMDb > TMDB > 名称（忽略大小写）去重合并，
// 每条旧记录生成一条观看记录，影视级字段与元数据只保留一份（已刮削元数据的行优先作为规范行）。
func (d *DB) migrateToViewings() error {
	filmCols, err := d.columns("films")
	if err != nil {
		return err
	}
	for _, c := range watchCols {
		if !filmCols[c] {
			return nil // 新库或已完成迁移
		}
	}

	rows, err := d.db.Query(`SELECT f.id, f.watch_year, f.start_date, f.end_date, f.platforms_raw, f.location, f.notes,
		f.category, f.name, f.imdb_id, f.douban_id, f.production_countries_raw, f.release_year, f.total_episodes,
		m.tmdb_id AS m_tmdb_id, m.imdb_id AS m_imdb_id
		FROM films f LEFT JOIN film_metadata m ON m.film_id = f.id`)
	if err != nil {
		return fmt.Errorf("read old films: %w", err)
	}
	var old []oldFilmRow
	for rows.Next() {
		var r oldFilmRow
		if err := rows.Scan(&r.id, &r.watchYear, &r.startDate, &r.endDate, &r.platformsRaw, &r.location, &r.notes,
			&r.category, &r.name, &r.imdbID, &r.doubanID, &r.productionCountriesRaw, &r.releaseYear, &r.totalEpisodes,
			&r.mTmdbID, &r.mImdbID); err != nil {
			rows.Close()
			return fmt.Errorf("scan old films: %w", err)
		}
		old = append(old, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	// 已刮削元数据的行优先作为规范行，其次按 id 升序（保证迁移结果稳定）
	sort.SliceStable(old, func(i, j int) bool {
		ti, tj := old[i].mTmdbID != nil, old[j].mTmdbID != nil
		if ti != tj {
			return ti
		}
		return old[i].id < old[j].id
	})

	// 去重分组：豆瓣 > IMDb > TMDB > 名称。
	// 豆瓣号为最高优先级键：相同即同一影视，双方均非空且不同则绝不合并
	// （多季综艺各季豆瓣条目不同，据此保持独立）；
	// 无豆瓣时依次按 IMDb / TMDB 匹配，更高优先级键冲突时不得合并；
	// 三键均未命中时按名称兜底，任一 ID 冲突视为不同影视。
	type group struct {
		canonical *oldFilmRow
		members   []*oldFilmRow
		doubanKey string
		imdbKey   string
		tmdbKey   string
	}
	byDouban := map[string]*group{}
	byImdb := map[string]*group{}
	byTmdb := map[string]*group{}
	byName := map[string]*group{}
	var groups []*group

	// imdb 候选：films.imdb_id 优先，缺失时用元数据 imdb_id（对应 JS `row.imdb_id || row.m_imdb_id || ''`）
	imdbOf := func(r *oldFilmRow) string {
		if r.imdbID != nil && strings.TrimSpace(*r.imdbID) != "" {
			return strings.TrimSpace(*r.imdbID)
		}
		if r.mImdbID != nil {
			return strings.TrimSpace(*r.mImdbID)
		}
		return ""
	}
	doubanOf := func(r *oldFilmRow) string {
		if r.doubanID == nil {
			return ""
		}
		return strings.TrimSpace(*r.doubanID)
	}
	nameOf := func(r *oldFilmRow) string {
		if r.name == nil {
			return ""
		}
		return strings.ToLower(strings.TrimSpace(*r.name))
	}

	for i := range old {
		row := &old[i]
		doubanKey := ""
		if dv := doubanOf(row); dv != "" {
			doubanKey = "d:" + dv
		}
		tmdbKey := ""
		if row.mTmdbID != nil {
			tmdbKey = fmt.Sprintf("t:%d", *row.mTmdbID)
		}
		imdbVal := imdbOf(row)
		imdbKey := ""
		if imdbVal != "" {
			imdbKey = "i:" + strings.ToLower(imdbVal)
		}
		nameKey := "n:" + nameOf(row)

		var g *group
		if doubanKey != "" {
			// 豆瓣为唯一键：命中即合并（低优先级键差异不阻断）
			g = byDouban[doubanKey]
		}
		if g == nil && imdbKey != "" {
			if c := byImdb[imdbKey]; c != nil &&
				!(doubanKey != "" && c.doubanKey != "" && doubanKey != c.doubanKey) {
				g = c
			}
		}
		if g == nil && tmdbKey != "" {
			if c := byTmdb[tmdbKey]; c != nil &&
				!(doubanKey != "" && c.doubanKey != "" && doubanKey != c.doubanKey) &&
				!(imdbKey != "" && c.imdbKey != "" && imdbKey != c.imdbKey) {
				g = c
			}
		}
		if g == nil {
			if cand := byName[nameKey]; cand != nil {
				conflict := (doubanKey != "" && cand.doubanKey != "" && doubanKey != cand.doubanKey) ||
					(imdbKey != "" && cand.imdbKey != "" && imdbKey != cand.imdbKey) ||
					(tmdbKey != "" && cand.tmdbKey != "" && tmdbKey != cand.tmdbKey)
				if !conflict {
					g = cand
				}
			}
		}
		if g == nil {
			g = &group{canonical: row}
			groups = append(groups, g)
		}
		g.members = append(g.members, row)
		if doubanKey != "" && g.doubanKey == "" {
			g.doubanKey = doubanKey
		}
		if tmdbKey != "" && g.tmdbKey == "" {
			g.tmdbKey = tmdbKey
		}
		if imdbKey != "" && g.imdbKey == "" {
			g.imdbKey = imdbKey
		}
		if doubanKey != "" {
			byDouban[doubanKey] = g
		}
		if tmdbKey != "" {
			byTmdb[tmdbKey] = g
		}
		if imdbKey != "" {
			byImdb[imdbKey] = g
		}
		byName[nameKey] = g
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	insViewing, err := tx.Prepare(`INSERT INTO viewings (film_id, watch_year, start_date, end_date, platforms_raw, location, notes) VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	updFilm, err := tx.Prepare(`UPDATE films SET category = ?, imdb_id = ?, douban_id = ?, production_countries_raw = ?, release_year = ?, total_episodes = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	delFilm, err := tx.Prepare(`DELETE FROM films WHERE id = ?`)
	if err != nil {
		return err
	}
	delMeta, err := tx.Prepare(`DELETE FROM film_metadata WHERE film_id = ?`)
	if err != nil {
		return err
	}
	reattach, err := tx.Prepare(`UPDATE film_metadata SET film_id = ? WHERE film_id = ?`)
	if err != nil {
		return err
	}
	hasMeta, err := tx.Prepare(`SELECT 1 FROM film_metadata WHERE film_id = ?`)
	if err != nil {
		return err
	}
	metaLocals, err := tx.Prepare(`SELECT poster_local, backdrop_local FROM film_metadata WHERE film_id = ?`)
	if err != nil {
		return err
	}
	defer func() {
		_ = insViewing.Close()
		_ = updFilm.Close()
		_ = delFilm.Close()
		_ = delMeta.Close()
		_ = reattach.Close()
		_ = hasMeta.Close()
		_ = metaLocals.Close()
	}()

	hasMetaRow := func(id int64) bool {
		var one int
		return hasMeta.QueryRow(id).Scan(&one) == nil
	}

	mergedFilms := 0
	for _, g := range groups {
		c := g.canonical

		// 影视级字段：规范行的值，缺失时用成员值回填（首个非空）
		category, imdbID, doubanID, countriesRaw := c.category, c.imdbID, c.doubanID, c.productionCountriesRaw
		releaseYear, totalEpisodes := c.releaseYear, c.totalEpisodes
		for _, m := range g.members {
			if category == nil {
				category = m.category
			}
			if imdbID == nil {
				imdbID = m.imdbID
			}
			if doubanID == nil {
				doubanID = m.doubanID
			}
			if countriesRaw == nil {
				countriesRaw = m.productionCountriesRaw
			}
			if releaseYear == nil {
				releaseYear = m.releaseYear
			}
			if totalEpisodes == nil {
				totalEpisodes = m.totalEpisodes
			}
		}

		// 元数据：规范行无而成员有 → 迁移给规范行；都有 → 删除成员的（连同本地图片）
		canonicalHasMeta := hasMetaRow(c.id)
		for _, m := range g.members {
			if m.id == c.id || !hasMetaRow(m.id) {
				continue
			}
			if !canonicalHasMeta {
				if _, err = reattach.Exec(c.id, m.id); err != nil {
					return fmt.Errorf("reattach metadata %d→%d: %w", m.id, c.id, err)
				}
				canonicalHasMeta = true
			} else {
				var poster, backdrop sql.NullString
				if err2 := metaLocals.QueryRow(m.id).Scan(&poster, &backdrop); err2 == nil {
					for _, f := range []sql.NullString{poster, backdrop} {
						if f.Valid && f.String != "" {
							_ = os.Remove(filepath.Join(d.imagesDir, f.String))
						}
					}
				}
				if _, err = delMeta.Exec(m.id); err != nil {
					return fmt.Errorf("delete metadata %d: %w", m.id, err)
				}
			}
		}

		// 每条旧记录生成一条观看记录（指向规范行）
		for _, m := range g.members {
			if _, err = insViewing.Exec(c.id, m.watchYear, m.startDate, m.endDate, m.platformsRaw, m.location, m.notes); err != nil {
				return fmt.Errorf("insert viewing for film %d: %w", c.id, err)
			}
		}

		if _, err = updFilm.Exec(category, imdbID, doubanID, countriesRaw, releaseYear, totalEpisodes, c.id); err != nil {
			return fmt.Errorf("update canonical film %d: %w", c.id, err)
		}
		for _, m := range g.members {
			if m.id != c.id {
				if _, err = delFilm.Exec(m.id); err != nil {
					return fmt.Errorf("delete merged film %d: %w", m.id, err)
				}
				mergedFilms++
			}
		}
	}

	// 旧列上有索引时先删索引，再物理删除已迁移的观看字段列
	if _, err = tx.Exec("DROP INDEX IF EXISTS idx_films_watch_year;"); err != nil {
		return fmt.Errorf("drop old index: %w", err)
	}
	for _, col := range watchCols {
		if _, err = tx.Exec("ALTER TABLE films DROP COLUMN " + col + ";"); err != nil {
			return fmt.Errorf("drop films.%s: %w", col, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	log.Printf("[db] 数据模型迁移完成：%d 部影视（合并去重 %d 条重复记录）", len(groups), mergedFilms)
	return nil
}

// columns 返回表的所有列名集合（对应 PRAGMA table_info）。
func (d *DB) columns(table string) (map[string]bool, error) {
	rows, err := d.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

// buildWhere 构造筛选 WHERE 子句与参数（GET /api/films 与 ratings/refresh 共用）。
// v2 模型下观看字段在 viewings 表（别名 v），影视字段在 films 表（别名 f）。
func (d *DB) buildWhere(f Filter) ([]string, []interface{}) {
	var where []string
	var args []interface{}
	if f.WatchYear != nil {
		where = append(where, "v.watch_year = ?")
		args = append(args, *f.WatchYear)
	}
	if f.ReleaseYear != nil {
		where = append(where, "f.release_year = ?")
		args = append(args, *f.ReleaseYear)
	}
	if f.Category == "__no_meta__" {
		where = append(where, "m.tmdb_id IS NULL")
	} else if f.Category != "" {
		where = append(where, "f.category = ?")
		args = append(args, f.Category)
	}
	if f.Missing == "imdb" {
		where = append(where, "(f.imdb_id IS NULL OR TRIM(f.imdb_id) = '')")
	} else if f.Missing == "douban" {
		where = append(where, "(f.douban_id IS NULL OR TRIM(f.douban_id) = '')")
	}
	if f.Platform != "" {
		where = append(where, "(',' || v.platforms_raw || ',') LIKE ? COLLATE NOCASE")
		args = append(args, "%,"+f.Platform+",%")
	}
	if f.Q != "" {
		where = append(where, "f.name LIKE ? COLLATE NOCASE")
		args = append(args, "%"+f.Q+"%")
	}
	return where, args
}

// --- 值转换辅助 ---

// nullIfEmpty 空字符串 → nil（对应 JS `x || null` 语义，用于 || null 字段）。
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// valStr *string → interface{}（nil 指针 → nil）。
func valStr(p *string) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

// valInt64 *int64 → interface{}。
func valInt64(p *int64) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

// valFloat64 *float64 → interface{}。
func valFloat64(p *float64) interface{} {
	if p == nil {
		return nil
	}
	return *p
}

// ErrNotFound 用于内部判断（未使用，保留以备扩展）。
var ErrNotFound = errors.New("not found")

// quoteIdent 简单转义标识符（白名单内字段名，仍做防御）。
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
