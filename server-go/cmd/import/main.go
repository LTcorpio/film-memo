// Command import 读取 Excel 并导入 SQLite（对应 npm run import / scripts/import-excel.js）。
package main

import (
	"fmt"
	"log"
	"os"

	"film-memo/internal/config"
	"film-memo/internal/db"
	"film-memo/internal/scripts"
)

func main() {
	cfg := config.Load()
	d, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer d.Close()

	if err := scripts.ImportExcel(d, cfg.ExcelPath); err != nil {
		fmt.Fprintf(os.Stderr, "❌ 导入失败: %v\n", err)
		os.Exit(1)
	}
}
