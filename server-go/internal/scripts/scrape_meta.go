package scripts

import (
	"context"
	"fmt"
	"log"
	"time"

	"film-memo/internal/db"
	"film-memo/internal/image"
	"film-memo/internal/model"
	"film-memo/internal/tmdb"
)

// ScrapeMetadata 按 IMDb 号批量刮削 TMDB 元数据（对应 scrape-metadata.js main）。
// force=true 强制重新刮削；onlyID 限定单条；缓存复用以避免重复请求 TMDB。
func ScrapeMetadata(ctx context.Context, d *db.DB, tc *tmdb.Client, imgs *image.Store, force bool, onlyID *int64) error {
	if !tc.Configured() {
		return fmt.Errorf("未配置 TMDB 凭证。请在 .env 中设置 TMDB_ACCESS_TOKEN 或 TMDB_API_KEY")
	}

	films, err := d.ListFilmsForScrape(force, onlyID)
	if err != nil {
		return err
	}

	suffix := ""
	if force {
		suffix = " (强制覆盖)"
	}
	log.Printf("待刮削: %d 部%s", len(films), suffix)

	ok, fail := 0, 0
	// imdb_id → *model.Metadata（nil 表示未匹配/失败，复用跳过）
	cache := map[string]*model.Metadata{}
	for _, film := range films {
		if scrapeOne(ctx, d, tc, imgs, film, force, cache) {
			ok++
		} else {
			fail++
		}
		time.Sleep(250 * time.Millisecond) // TMDB 速率限制 ~40 req/10s
	}
	log.Printf("\n完成: 成功 %d，失败 %d", ok, fail)
	return nil
}

// scrapeOne 处理单部影片，返回是否成功。
func scrapeOne(ctx context.Context, d *db.DB, tc *tmdb.Client, imgs *image.Store, film db.FilmBasic, force bool, cache map[string]*model.Metadata) bool {
	// 缓存复用（仅 !force）
	if !force {
		if meta, found := cache[film.ImdbID]; found {
			if meta != nil {
				posterLocal := imgs.DownloadTmdb(ctx, meta.PosterPath, fmt.Sprintf("%d-poster", film.ID), false)
				backdropLocal := imgs.DownloadTmdb(ctx, meta.BackdropPath, fmt.Sprintf("%d-backdrop", film.ID), false)
				if err := d.UpsertMetadata(film.ID, meta, posterLocal, backdropLocal); err != nil {
					log.Printf("  ❌ id=%d «%s»: %v", film.ID, film.Name, err)
					return false
				}
				log.Printf("  ♻️  id=%d «%s» (复用缓存)", film.ID, film.Name)
				return true
			}
			return false // 缓存为未匹配
		}
	}

	found, err := tc.FindByImdb(ctx, film.ImdbID, film.Category)
	if err != nil {
		log.Printf("  ❌ id=%d «%s»: %v", film.ID, film.Name, err)
		if !force {
			cache[film.ImdbID] = nil
		}
		return false
	}
	if found == nil {
		log.Printf("  ⚠️  未匹配: id=%d «%s» (%s)", film.ID, film.Name, film.ImdbID)
		if !force {
			cache[film.ImdbID] = nil
		}
		return false
	}
	details, err := tc.GetDetails(ctx, found.TmdbID, found.MediaType)
	if err != nil {
		log.Printf("  ❌ id=%d «%s»: %v", film.ID, film.Name, err)
		if !force {
			cache[film.ImdbID] = nil
		}
		return false
	}
	meta := tmdb.NormalizeDetails(details, found.MediaType)
	if meta == nil {
		log.Printf("  ⚠️  详情为空: id=%d «%s»", film.ID, film.Name)
		if !force {
			cache[film.ImdbID] = nil
		}
		return false
	}

	posterLocal := imgs.DownloadTmdb(ctx, meta.PosterPath, fmt.Sprintf("%d-poster", film.ID), false)
	backdropLocal := imgs.DownloadTmdb(ctx, meta.BackdropPath, fmt.Sprintf("%d-backdrop", film.ID), false)
	if err := d.UpsertMetadata(film.ID, meta, posterLocal, backdropLocal); err != nil {
		log.Printf("  ❌ id=%d «%s»: %v", film.ID, film.Name, err)
		if !force {
			cache[film.ImdbID] = nil
		}
		return false
	}
	log.Printf("  ✅ id=%d «%s» → %s/%d «%s»", film.ID, film.Name, meta.MediaType, meta.TmdbID, meta.Title)
	if !force {
		cache[film.ImdbID] = meta
	}
	return true
}
