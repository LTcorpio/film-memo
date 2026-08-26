// Command scrape 按 IMDb 号批量刮削 TMDB 元数据（对应 npm run scrape / scripts/scrape-metadata.js）。
// 用法:
//
//	scrape            # 仅处理缺少元数据、且已有 imdb_id 的影片
//	scrape --force    # 强制重新刮削（覆盖已有元数据）
//	scrape --id 6     # 仅处理指定 film id
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"film-memo/internal/config"
	"film-memo/internal/db"
	"film-memo/internal/image"
	"film-memo/internal/scripts"
	"film-memo/internal/tmdb"
)

func main() {
	cfg := config.Load()
	d, err := db.Open(cfg.DBPath, cfg.ImagesDir)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer d.Close()

	tc := tmdb.NewClient(cfg.TmdbAccessToken, cfg.TmdbAPIKey)
	imgs, err := image.NewStore(cfg.ImagesDir)
	if err != nil {
		log.Fatalf("创建图片目录失败: %v", err)
	}

	// 解析命令行参数
	args := os.Args[1:]
	force := false
	var onlyID *int64
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--force":
			force = true
		case "--id":
			if i+1 < len(args) {
				if n, err := strconv.ParseInt(args[i+1], 10, 64); err == nil {
					onlyID = &n
				}
				i++
			}
		}
	}

	if err := scripts.ScrapeMetadata(context.Background(), d, tc, imgs, force, onlyID); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}
}
