// Package imdb 提供 IMDb 官方评分数据集的下载、缓存与查询。
//
// 数据源为官方公开数据集 https://datasets.imdbws.com/title.ratings.tsv.gz
// （每日更新、无需 API Key、无限流），字段：tconst / averageRating / numVotes。
// 本地缓存压缩包并按 Last-Modified 做条件请求（304 即复用），日常零流量。
//
// 查询采用「流式扫描 + 白名单收集」：只保留本次需要的 tconst，
// 内存占用与待查数量成正比，避免为 170 万条数据常驻上百 MB 索引。
package imdb

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	datasetURL   = "https://datasets.imdbws.com/title.ratings.tsv.gz"
	fileName     = "title.ratings.tsv.gz"
	metaName     = "title.ratings.meta.json"
	fetchTimeout = 5 * time.Minute
)

// Rating 是单个 IMDb 条目的评分与投票数。
type Rating struct {
	Average float64
	Votes   int64
}

// Meta 描述本地缓存的来源与状态。
type Meta struct {
	LastModified string `json:"lastModified"` // 远端 Last-Modified 响应头
	FetchedAt    string `json:"fetchedAt"`    // 本地下载完成时间（ISO8601，UTC）
	Size         int64  `json:"size"`         // 压缩包字节数
	File         string `json:"file"`         // 本地缓存文件路径
	Offline      bool   `json:"offline"`      // 本次未联网校验，直接使用本地缓存
	Available    bool   `json:"available"`    // 本地是否存在可用缓存
}

// Store 管理 IMDb 数据集的本地缓存。
type Store struct {
	dir  string
	http *http.Client
	mu   sync.Mutex // 串行化下载与扫描，避免并发重复拉取
}

// NewStore 创建数据集缓存，dir 为缓存目录（不存在时自动创建）。
func NewStore(dir string) *Store {
	return &Store{dir: dir, http: &http.Client{Timeout: fetchTimeout}}
}

func (s *Store) filePath() string { return filepath.Join(s.dir, fileName) }
func (s *Store) metaPath() string { return filepath.Join(s.dir, metaName) }

// Ensure 确保本地数据集存在且为最新：带 If-Modified-Since 的条件请求，
// 304 时直接复用本地文件；网络异常但本地已有缓存时降级为离线使用。
func (s *Store) Ensure(ctx context.Context) (*Meta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}
	cached, _ := s.readMeta()
	hasFile := false
	if info, err := os.Stat(s.filePath()); err == nil && info.Size() > 0 {
		hasFile = true
	}
	// fallback：网络不可用时使用本地缓存（若无缓存则返回 nil）
	fallback := func(offline bool) *Meta {
		if !hasFile {
			return nil
		}
		m := Meta{}
		if cached != nil {
			m = *cached
		}
		m.File = s.filePath()
		m.Offline = offline
		m.Available = true
		_ = s.writeMeta(&m) // 落盘本次是否离线，供 /api/ratings/status 展示
		return &m
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, datasetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "film-memo/1.0")
	if hasFile && cached != nil && cached.LastModified != "" {
		req.Header.Set("If-Modified-Since", cached.LastModified)
	}

	resp, err := s.http.Do(req)
	if err != nil {
		if m := fallback(true); m != nil {
			return m, nil
		}
		return nil, fmt.Errorf("下载 IMDb 数据集失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		if m := fallback(false); m != nil {
			return m, nil
		}
	}
	if resp.StatusCode != http.StatusOK {
		if m := fallback(true); m != nil {
			return m, nil
		}
		return nil, fmt.Errorf("下载 IMDb 数据集失败: HTTP %d", resp.StatusCode)
	}

	// 流式写入临时文件，成功后原子替换，避免中断留下损坏缓存
	tmp, err := os.CreateTemp(s.dir, "ratings-*.tmp")
	if err != nil {
		return nil, err
	}
	tmpName := tmp.Name()
	size, copyErr := io.Copy(tmp, resp.Body)
	closeErr := tmp.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmpName)
		if copyErr == nil {
			copyErr = closeErr
		}
		return nil, fmt.Errorf("写入 IMDb 数据集失败: %w", copyErr)
	}
	if err := os.Rename(tmpName, s.filePath()); err != nil {
		_ = os.Remove(tmpName)
		return nil, err
	}

	m := &Meta{
		LastModified: resp.Header.Get("Last-Modified"),
		FetchedAt:    time.Now().UTC().Format(time.RFC3339),
		Size:         size,
		File:         s.filePath(),
		Available:    true,
	}
	if err := s.writeMeta(m); err != nil {
		return nil, err
	}
	return m, nil
}

// LookupAll 流式扫描本地数据集，返回请求集合中命中的评分（未命中的不出现）。
// 调用前需保证 Ensure 已成功（本地存在缓存文件）。
func (s *Store) LookupAll(ctx context.Context, ids []string) (map[string]Rating, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			want[id] = true
		}
	}
	out := make(map[string]Rating, len(want))
	if len(want) == 0 {
		return out, nil
	}

	f, err := os.Open(s.filePath())
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	header := true
	for sc.Scan() {
		if header { // 跳过表头 tconst\taverageRating\tnumVotes
			header = false
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line := sc.Text()
		i := strings.IndexByte(line, '\t')
		if i <= 0 {
			continue
		}
		tconst := line[:i]
		if !want[tconst] {
			continue
		}
		rest := line[i+1:]
		j := strings.IndexByte(rest, '\t')
		if j <= 0 {
			continue
		}
		avg, err := strconv.ParseFloat(rest[:j], 64)
		if err != nil {
			continue
		}
		votes, err := strconv.ParseInt(strings.TrimSpace(rest[j+1:]), 10, 64)
		if err != nil {
			votes = 0
		}
		out[tconst] = Rating{Average: avg, Votes: votes}
		if len(out) == len(want) {
			break // 全部命中，提前结束扫描
		}
	}
	return out, sc.Err()
}

// Cache 返回本地缓存信息（无缓存时 Available 为 false）。
func (s *Store) Cache() Meta {
	if m, err := s.readMeta(); err == nil && m != nil {
		if info, err := os.Stat(s.filePath()); err == nil && info.Size() > 0 {
			m.File = s.filePath()
			m.Available = true
			return *m
		}
	}
	return Meta{File: s.filePath()}
}

func (s *Store) readMeta() (*Meta, error) {
	b, err := os.ReadFile(s.metaPath())
	if err != nil {
		return nil, err
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) writeMeta(m *Meta) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.metaPath(), b, 0o644)
}
