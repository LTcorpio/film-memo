// Command server 启动 film-memo HTTP 后端服务。
// 对应 JS 版 `node server/index.js`。
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"film-memo/internal/api"
	"film-memo/internal/config"
	"film-memo/internal/db"
	"film-memo/internal/image"
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

	srv := api.New(cfg, d, tc, imgs)
	httpSrv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("film-memo 后端: http://localhost:%s", cfg.Port)
	log.Printf("  本地图片目录: %s", cfg.ImagesDir)
	if tc.Configured() {
		log.Printf("  TMDB 凭证: 已配置")
	} else {
		log.Printf("  TMDB 凭证: 未配置（手动填充不可用）")
	}

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	// 优雅关闭
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
	log.Printf("服务已停止")
}
