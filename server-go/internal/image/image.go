// Package image 负责本地图片的下载、保存与删除。
// 对应 index.js 的 downloadTmdbImage 与各 image handler 中的文件操作。
package image

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Store 管理本地图片目录。
type Store struct {
	dir  string
	http *http.Client
}

// NewStore 创建图片存储并确保目录存在。
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{
		dir:  dir,
		http: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// Dir 返回图片目录路径。
func (s *Store) Dir() string { return s.dir }

// Path 返回某文件的完整路径。
func (s *Store) Path(file string) string {
	return filepath.Join(s.dir, file)
}

// Remove 删除本地图片，文件不存在时静默忽略（对应 unlinkSync 的 try/catch）。
func (s *Store) Remove(file string) {
	if file == "" {
		return
	}
	_ = os.Remove(s.Path(file))
}

// SaveUpload 保存上传的原始字节到指定文件名。
func (s *Store) SaveUpload(filename string, data []byte) error {
	return os.WriteFile(s.Path(filename), data, 0o644)
}

// DownloadTmdb 从 TMDB（original 尺寸）下载图片到本地，返回文件名；失败返回 nil（对应 downloadTmdbImage）。
// prefix 为文件名前缀（如 "{id}-poster"）；timestamp 为 true 时追加毫秒时间戳（服务端重刮场景，避免缓存），
// 为 false 时仅用 prefix+扩展名（批量脚本场景）。
func (s *Store) DownloadTmdb(ctx context.Context, path, prefix string, timestamp bool) *string {
	if path == "" {
		return nil
	}
	url := "https://image.tmdb.org/t/p/original" + path
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.Printf("图片下载失败: %s: %v", url, err)
		return nil
	}
	resp, err := s.http.Do(req)
	if err != nil {
		log.Printf("图片下载失败: %s: %v", url, err)
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("图片下载失败: %s: status %d", url, resp.StatusCode)
		return nil
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("图片下载失败: %s: %v", url, err)
		return nil
	}
	ext := filepath.Ext(path)
	if ext == "" {
		ext = ".jpg"
	}
	var file string
	if timestamp {
		file = fmt.Sprintf("%s-%d%s", prefix, time.Now().UnixMilli(), ext)
	} else {
		file = prefix + ext
	}
	if err := os.WriteFile(s.Path(file), data, 0o644); err != nil {
		log.Printf("图片写入失败: %s: %v", file, err)
		return nil
	}
	return &file
}
