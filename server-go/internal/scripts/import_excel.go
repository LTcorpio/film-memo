// Package scripts 实现命令行脚本：Excel 导入与 TMDB 批量刮削。
// 对应 JS 版 scripts/import-excel.js 与 scripts/scrape-metadata.js。
package scripts

import (
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

// ImportExcel 读取 Excel 并导入 SQLite（对应 import-excel.js main）。
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

	const insertSQL = `INSERT OR IGNORE INTO films
		(id, watch_year, category, name, imdb_id, production_countries_raw,
		 release_year, start_date, end_date, total_episodes, platforms_raw, location, notes)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

	tx, err := d.DB().Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(insertSQL)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	inserted, skipped := 0, 0
	for r := 1; r < len(rows); r++ {
		row := rows[r]
		if len(row) == 0 {
			continue
		}
		id := toInt(cell(row, 0))
		if id == nil {
			continue // 跳过空行/合计行
		}
		rec := map[string]interface{}{"id": id}
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
		res, err := stmt.Exec(
			rec["id"], rec["watch_year"], rec["category"], rec["name"], rec["imdb_id"],
			rec["production_countries_raw"], rec["release_year"], rec["start_date"], rec["end_date"],
			rec["total_episodes"], rec["platforms_raw"], rec["location"], rec["notes"],
		)
		if err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			return fmt.Errorf("第 %d 行: %w", r+1, err)
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			inserted++
		} else {
			skipped++
		}
	}
	_ = stmt.Close()
	if err := tx.Commit(); err != nil {
		return err
	}

	log.Printf("✅ 导入完成：新增 %d 条，跳过 %d 条已存在记录。", inserted, skipped)

	// 摘要统计
	var total, noImdb, multiPlatform int64
	var noImdbN, multiN int64
	_ = d.DB().QueryRow(`SELECT COUNT(*),
		SUM(CASE WHEN imdb_id IS NULL THEN 1 ELSE 0 END),
		SUM(CASE WHEN platforms_raw LIKE '%,%' THEN 1 ELSE 0 END) FROM films`).Scan(&total, &noImdbN, &multiN)
	noImdb = noImdbN
	multiPlatform = multiN
	log.Printf("📊 统计: { total: %d, no_imdb: %d, multi_platform: %d }", total, noImdb, multiPlatform)
	return nil
}
