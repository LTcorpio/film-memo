// Package scripts 实现命令行脚本：Excel 导入与 TMDB 批量刮削。
// 对应 JS 版 scripts/import-excel.js 与 scripts/scrape-metadata.js。
package scripts

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"film-memo/internal/db"
)

// headerMap 中文表头（去空白/换行后）→ 英文字段（对应 import-excel.js HEADER_MAP）。
var headerMap = map[string]string{
	"序":          "id",
	"观看年份":      "watch_year",
	"类别":         "category",
	"名称":         "name",
	"IMDb":       "imdb_id",
	"制片国家":      "production_countries_raw",
	"上映年份":      "release_year",
	"开始观看日期":    "start_date",
	"结束观看日期":    "end_date",
	"总集/期数":     "total_episodes",
	"观看平台":      "platforms_raw",
	"观看地点":      "location",
	"备注":         "notes",
}

// cleanHeader 去除所有空白字符（对应 cleanHeader 的 /[\s\n\r]+/g）。
func cleanHeader(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' && r != '\u00a0' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// cell 安全取行内某列（越界返回空串）。
func cell(row []string, idx int) string {
	if idx >= 0 && idx < len(row) {
		return row[idx]
	}
	return ""
}

// toInt 解析为 int64 或 nil（对应 toInt）。
func toInt(s string) interface{} {
	t := strings.TrimSpace(s)
	if t == "" {
		return nil
	}
	n, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return nil
	}
	return int64(n)
}

// toDateISO 转为 YYYY-MM-DD 或 nil（对应 toDateISO）。
func toDateISO(s string) interface{} {
	t := strings.TrimSpace(s)
	if t == "" {
		return nil
	}
	layouts := []string{
		"2006-01-02", "2006/1/2", "2006/01/02", "2006-1-2",
		"1/2/2006", "01/02/2006", "2006年1月2日",
		"2006.1.2", "2006.01.02", "2006-01-02 15:04:05",
	}
	for _, l := range layouts {
		if tt, err := time.Parse(l, t); err == nil {
			return tt.Format("2006-01-02")
		}
	}
	// 兜底：Excel 日期序列号
	if f, err := strconv.ParseFloat(t, 64); err == nil && f > 59 && f < 80000 {
		if tt, err := excelize.ExcelDateToTime(f, false); err == nil {
			return tt.Format("2006-01-02")
		}
	}
	return t // 解析失败保留原串（与 JS 兜底一致）
}

// normalizeImdb 规范化 IMDb 号，无效值返回 nil（对应 normalizeImdb）。
func normalizeImdb(s string) interface{} {
	t := strings.TrimSpace(s)
	if t == "" || t == "-" || strings.EqualFold(t, "na") || strings.EqualFold(t, "n/a") {
		return nil
	}
	return t
}

// splitMulti 多值字段原样保留 trim 后字符串，空返回 nil（对应 splitCountries/splitPlatforms）。
func splitMulti(s string) interface{} {
	t := strings.TrimSpace(s)
	if t == "" {
		return nil
	}
	return t
}

// ImportExcel 读取 Excel 并导入 SQLite（数据模型 v2：films + viewings，对应 import-excel.js）。
// 匹配已有影视优先按 IMDb（Excel 无豆瓣列，豆瓣键不参与导入匹配；名称不同也视为同一影视），
// 其次按名称（IMDb 冲突时新建一条）；命中则回填 NULL 字段；每行 Excel 生成一条 viewings
// 观看记录；已存在的同内容观看记录跳过，保证幂等、不覆盖 UI 修改过的数据。
func ImportExcel(d *db.DB, excelPath string) error {
	log.Printf("📖 读取 Excel: %s", excelPath)
	f, err := excelize.OpenFile(excelPath)
	if err != nil {
		return err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return fmt.Errorf("Excel 无工作表")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return fmt.Errorf("读取工作表失败: %w", err)
	}
	if len(rows) == 0 {
		return fmt.Errorf("Excel 无数据")
	}

	// 第一行表头
	header := rows[0]
	colMap := map[int]string{}
	for i, h := range header {
		zh := cleanHeader(h)
		if field, ok := headerMap[zh]; ok {
			colMap[i] = field
		}
	}

	// 检查缺失字段
	seen := map[string]bool{}
	for _, field := range colMap {
		seen[field] = true
	}
	var missing []string
	for _, field := range headerMap {
		if !seen[field] {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		log.Printf("⚠️  表头缺少字段: %s", strings.Join(missing, ", "))
	}

	tx, err := d.DB().Begin()
	if err != nil {
		return err
	}
	rollback := func() { _ = tx.Rollback() }

	findFilm, err := tx.Prepare(`SELECT id, imdb_id FROM films WHERE TRIM(name) = TRIM(?) COLLATE NOCASE`)
	if err != nil {
		rollback()
		return err
	}
	backfillFilm, err := tx.Prepare(`UPDATE films SET
		category = COALESCE(category, ?), imdb_id = COALESCE(imdb_id, ?),
		production_countries_raw = COALESCE(production_countries_raw, ?),
		release_year = COALESCE(release_year, ?), total_episodes = COALESCE(total_episodes, ?)
		WHERE id = ?`)
	if err != nil {
		_ = findFilm.Close()
		rollback()
		return err
	}
	insertFilm, err := tx.Prepare(`INSERT INTO films
		(name, category, imdb_id, production_countries_raw, release_year, total_episodes)
		VALUES (?,?,?,?,?,?)`)
	if err != nil {
		_ = findFilm.Close()
		_ = backfillFilm.Close()
		rollback()
		return err
	}
	viewingExists, err := tx.Prepare(`SELECT 1 FROM viewings WHERE film_id = ?
		AND watch_year IS ? AND start_date IS ? AND end_date IS ?
		AND platforms_raw IS ? AND location IS ? AND notes IS ?`)
	if err != nil {
		_ = findFilm.Close()
		_ = backfillFilm.Close()
		_ = insertFilm.Close()
		rollback()
		return err
	}
	insertViewing, err := tx.Prepare(`INSERT INTO viewings
		(film_id, watch_year, start_date, end_date, platforms_raw, location, notes)
		VALUES (?,?,?,?,?,?,?)`)
	if err != nil {
		_ = findFilm.Close()
		_ = backfillFilm.Close()
		_ = insertFilm.Close()
		_ = viewingExists.Close()
		rollback()
		return err
	}
	findFilmByImdb, err := tx.Prepare(`SELECT id FROM films WHERE TRIM(imdb_id) = TRIM(?) COLLATE NOCASE`)
	if err != nil {
		_ = findFilm.Close()
		_ = backfillFilm.Close()
		_ = insertFilm.Close()
		_ = viewingExists.Close()
		_ = insertViewing.Close()
		rollback()
		return err
	}
	defer func() {
		_ = findFilm.Close()
		_ = backfillFilm.Close()
		_ = insertFilm.Close()
		_ = viewingExists.Close()
		_ = insertViewing.Close()
		_ = findFilmByImdb.Close()
	}()

	inserted, skipped := 0, 0
	for r := 1; r < len(rows); r++ {
		row := rows[r]
		if len(row) == 0 {
			continue
		}
		if toInt(cell(row, 0)) == nil {
			continue // 跳过空行/合计行（序号列无效）
		}
		rec := map[string]interface{}{}
		for col, field := range colMap {
			if field == "id" {
				continue
			}
			raw := cell(row, col)
			switch field {
			case "watch_year", "release_year", "total_episodes":
				rec[field] = toInt(raw)
			case "imdb_id":
				rec[field] = normalizeImdb(raw)
			case "start_date", "end_date":
				rec[field] = toDateISO(raw)
			case "production_countries_raw", "platforms_raw":
				rec[field] = splitMulti(raw)
			default:
				t := strings.TrimSpace(raw)
				if t == "" {
					rec[field] = nil
				} else {
					rec[field] = t
				}
			}
		}

		name, _ := rec["name"].(string)
		if strings.TrimSpace(name) == "" {
			continue
		}

		// 匹配已有影视：IMDb > 名称。
		// 行内带 IMDb 时按 IMDb 匹配（名称不同也视为同一影视）；
		// 未命中再按名称匹配，IMDb 冲突（双方均非空且不同）视为不同影视新建一条。
		newImdb, _ := rec["imdb_id"].(string)
		var filmID int64
		matched := false
		if newImdb != "" {
			var idByImdb sql.NullInt64
			if err := findFilmByImdb.QueryRow(newImdb).Scan(&idByImdb); err != nil && !errors.Is(err, sql.ErrNoRows) {
				rollback()
				return fmt.Errorf("第 %d 行: %w", r+1, err)
			}
			if idByImdb.Valid {
				filmID = idByImdb.Int64
				matched = true
				if _, err := backfillFilm.Exec(
					rec["category"], rec["imdb_id"], rec["production_countries_raw"],
					rec["release_year"], rec["total_episodes"], filmID,
				); err != nil {
					rollback()
					return fmt.Errorf("第 %d 行: %w", r+1, err)
				}
			}
		}
		if !matched {
			var existingID sql.NullInt64
			var existingImdb sql.NullString
			if err := findFilm.QueryRow(name).Scan(&existingID, &existingImdb); err != nil && !errors.Is(err, sql.ErrNoRows) {
				rollback()
				return fmt.Errorf("第 %d 行: %w", r+1, err)
			}
			imdbConflict := existingID.Valid && newImdb != "" && existingImdb.Valid && existingImdb.String != "" && newImdb != existingImdb.String

			if existingID.Valid && !imdbConflict {
				filmID = existingID.Int64
				matched = true
				if _, err := backfillFilm.Exec(
					rec["category"], rec["imdb_id"], rec["production_countries_raw"],
					rec["release_year"], rec["total_episodes"], filmID,
				); err != nil {
					rollback()
					return fmt.Errorf("第 %d 行: %w", r+1, err)
				}
			}
		}
		if !matched {
			res, err := insertFilm.Exec(
				name, rec["category"], rec["imdb_id"], rec["production_countries_raw"],
				rec["release_year"], rec["total_episodes"],
			)
			if err != nil {
				rollback()
				return fmt.Errorf("第 %d 行: %w", r+1, err)
			}
			if filmID, err = res.LastInsertId(); err != nil {
				rollback()
				return fmt.Errorf("第 %d 行: %w", r+1, err)
			}
		}

		// 同内容观看记录已存在则跳过（幂等，避免重复导入）
		var one int
		err := viewingExists.QueryRow(
			filmID, rec["watch_year"], rec["start_date"], rec["end_date"],
			rec["platforms_raw"], rec["location"], rec["notes"],
		).Scan(&one)
		if err == nil {
			skipped++
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			rollback()
			return fmt.Errorf("第 %d 行: %w", r+1, err)
		}
		if _, err := insertViewing.Exec(
			filmID, rec["watch_year"], rec["start_date"], rec["end_date"],
			rec["platforms_raw"], rec["location"], rec["notes"],
		); err != nil {
			rollback()
			return fmt.Errorf("第 %d 行: %w", r+1, err)
		}
		inserted++
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("✅ 导入完成：新增 %d 条观看记录，跳过 %d 条已存在记录。", inserted, skipped)

	// 摘要统计
	var films, viewings, noImdb, multiPlatform int64
	_ = d.DB().QueryRow(`SELECT
		(SELECT COUNT(*) FROM films),
		(SELECT COUNT(*) FROM viewings),
		(SELECT COUNT(*) FROM films WHERE imdb_id IS NULL),
		(SELECT COUNT(*) FROM viewings WHERE platforms_raw LIKE '%,%')`).Scan(&films, &viewings, &noImdb, &multiPlatform)
	log.Printf("📊 统计: { films: %d, viewings: %d, no_imdb: %d, multi_platform: %d }", films, viewings, noImdb, multiPlatform)
	return nil
}
