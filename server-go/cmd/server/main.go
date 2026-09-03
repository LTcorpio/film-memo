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

	// ListenAndServe 的错误不能在 goroutine 里 log.Fatalf 直接退出：
	// os.Exit 会跳过 defer/收尾逻辑，导致 WAL 未 checkpoint、数据库未关闭。
	serveErr := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serveErr <- err
		}
	}()

	// 优雅关闭：SIGTERM（docker stop）或 SIGINT（Ctrl-C）
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serveErr:
		// 启动失败：先落盘关闭数据库，再退出
		_ = d.Close()
		log.Fatalf("服务启动失败: %v", err)
	case sig := <-stop:
		log.Printf("收到信号 %v，正在停止服务...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = httpSrv.Shutdown(ctx)
		cancel()
	}

	// 显式收尾：WAL checkpoint 落盘 + 关闭连接 + 清理 -wal/-shm 临时文件
	if err := d.Close(); err != nil {
		log.Printf("数据库关闭异常: %v", err)
	}
	log.Printf("数据库已落盘，服务已停止")
}
