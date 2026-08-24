// Package db 封装 SQLite 连接、建表、迁移与全部数据访问。
// 对应 JS 版 db.js（连接/建表/迁移）与 index.js 中的全部 DB 查询。
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动
)

// DB 包装 *sql.DB，承载所有数据访问方法。
type DB struct {
	db *sql.DB
}

// schema 对应 db.js 的 SCHEMA（CREATE TABLE + INDEX）。
const schema = `
CREATE TABLE IF NOT EXISTS films (
  id                  INTEGER PRIMARY KEY,
  watch_year          INTEGER,
  category            TEXT,
  name                TEXT NOT NULL,
  imdb_id             TEXT,
  douban_id           TEXT,
  production_countries_raw TEXT,
  release_year        INTEGER,
  start_date          TEXT,
  end_date            TEXT,
  total_episodes      INTEGER,
  platforms_raw       TEXT,
  location            TEXT,
  notes               TEXT
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
CREATE INDEX IF NOT EXISTS idx_films_watch_year ON films(watch_year);
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

// Open 打开数据库并完成建表与旧库迁移。单连接模式以避免写锁冲突（与 better-sqlite3 同步语义一致）。
func Open(path string) (*DB, error) {
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
	d := &DB{db: conn}
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
func (d *DB) buildWhere(f Filter) ([]string, []interface{}) {
	var where []string
	var args []interface{}
	if f.WatchYear != nil {
		where = append(where, "f.watch_year = ?")
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
		where = append(where, "(',' || f.platforms_raw || ',') LIKE ? COLLATE NOCASE")
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
