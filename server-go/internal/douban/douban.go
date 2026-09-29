// Package douban 提供豆瓣评分的薄客户端。
//
// 豆瓣没有公开的官方 API，社区亦无长期可用的公共数据服务，故此处直接调用
// 豆瓣自身的移动端接口（rexxar）：
//
//	GET https://m.douban.com/rexxar/api/v2/{movie|tv}/{id}
//	    需带移动端 UA 与 Referer: https://m.douban.com/{movie|tv}/subject/{id}/
//	    响应 rating.value / rating.count
//
// 兜底接口（rexxar 均不可用时）：
//
//	GET https://movie.douban.com/j/subject_abstract?subject_id={id}
//	    需带 Referer: https://movie.douban.com/
//	    响应 subject.rate（字符串，未评分为空串）
//
// 请求串行执行并限速，避免触发反爬；错误重试若干次。
package douban

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	rexxarBase  = "https://m.douban.com/rexxar/api/v2"
	abstractAPI = "https://movie.douban.com/j/subject_abstract"
	mobileUA    = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1"
	// 两次请求之间的最小间隔（限速）
	minInterval = 1200 * time.Millisecond
	maxAttempts = 3
)

// ErrNotFound 表示该豆瓣条目不存在（404）。
var ErrNotFound = errors.New("豆瓣条目不存在")

// Rating 是豆瓣评分与评价人数。
type Rating struct {
	Value float64 // 0 表示暂无评分
	Count int64   // 评价人数；兜底接口不返回时为 0
}

// Client 是豆瓣接口客户端。
type Client struct {
	http *http.Client
	mu   sync.Mutex // 串行化全部请求（含限速），避免并发触发反爬
	last time.Time
}

// NewClient 创建豆瓣客户端。
func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 15 * time.Second}}
}

// GetRating 获取条目评分。mediaType 取 "movie"/"tv"，仅决定首选接口路径；
// 首选返回 404 时自动尝试另一类型（同一条目可能被归到另一类）；
// rexxar 整体不可用时回退 PC 端抽象接口。
func (c *Client) GetRating(ctx context.Context, id, mediaType string) (*Rating, error) {
	primary, secondary := "movie", "tv"
	if mediaType == "tv" {
		primary, secondary = "tv", "movie"
	}
	var lastErr error
	r, err := c.rexxar(ctx, id, primary)
	if err == nil {
		return r, nil
	}
	if errors.Is(err, ErrNotFound) {
		// 仅 404（归类不同）才换类型重试；网络类错误换类型必然同样失败
		if r, err = c.rexxar(ctx, id, secondary); err == nil {
			return r, nil
		}
		if !errors.Is(err, ErrNotFound) {
			lastErr = err
		}
	} else {
		lastErr = err
	}
	// 兜底：PC 端条目摘要接口（不同主机，rexxar 端点不可用时仍可能生效）
	if r, err = c.abstract(ctx, id); err == nil {
		return r, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, err
}

// rexxar 调用移动端接口，返回评分（未评分时 Value 为 0）。
func (c *Client) rexxar(ctx context.Context, id, mediaType string) (*Rating, error) {
	endpoint := fmt.Sprintf("%s/%s/%s", rexxarBase, mediaType, url.PathEscape(id))
	referer := fmt.Sprintf("https://m.douban.com/%s/subject/%s/", mediaType, url.PathEscape(id))

	var raw []byte
	var err error
	c.mu.Lock()
	if err = c.acquire(ctx); err == nil {
		raw, err = c.retryGet(ctx, endpoint, referer)
	}
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	var resp struct {
		Rating struct {
			Value float64 `json:"value"`
			Count int64   `json:"count"`
		} `json:"rating"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("解析豆瓣响应失败: %w", err)
	}
	return &Rating{Value: resp.Rating.Value, Count: resp.Rating.Count}, nil
}

// abstract 调用 PC 端条目摘要接口（无评价人数）。
func (c *Client) abstract(ctx context.Context, id string) (*Rating, error) {
	endpoint := abstractAPI + "?subject_id=" + url.QueryEscape(id)
	var raw []byte
	var err error
	c.mu.Lock()
	if err = c.acquire(ctx); err == nil {
		raw, err = c.retryGet(ctx, endpoint, "https://movie.douban.com/")
	}
	c.mu.Unlock()
	if err != nil {
		return nil, err
	}

	var resp struct {
		Subject struct {
			Rate string `json:"rate"`
		} `json:"subject"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("解析豆瓣响应失败: %w", err)
	}
	value := 0.0
	if s := strings.TrimSpace(resp.Subject.Rate); s != "" {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			value = f
		}
	}
	return &Rating{Value: value}, nil
}

// acquire 限速：距上次请求不足 minInterval 时等待。调用方须持有 mu。
func (c *Client) acquire(ctx context.Context) error {
	if wait := minInterval - time.Since(c.last); wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.last = time.Now()
	return nil
}

// retryGet 带重试的 GET；404 立即返回 ErrNotFound（不重试）。
// 调用方须持有 mu（内部含重试等待，串行执行可避免整站并发压力）。
func (c *Client) retryGet(ctx context.Context, endpoint, referer string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * 1500 * time.Millisecond):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		body, status, err := c.doGet(ctx, endpoint, referer)
		if err != nil {
			// 连接层失败（离线、DNS 失败、拒绝连接）重试无意义，立即返回
			if isDialError(err) {
				return nil, err
			}
			lastErr = err
			continue
		}
		switch {
		case status == http.StatusNotFound:
			return nil, ErrNotFound
		case status >= 200 && status < 300:
			return body, nil
		case status == http.StatusForbidden || status == http.StatusTooManyRequests || status >= 500:
			lastErr = fmt.Errorf("豆瓣接口返回 HTTP %d", status)
		default:
			return nil, fmt.Errorf("豆瓣接口返回 HTTP %d", status)
		}
	}
	return nil, lastErr
}

// isDialError 判断是否为连接层失败（离线 / DNS 失败 / 拒绝连接）。
func isDialError(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr) && opErr.Op == "dial"
}

// doGet 发起单次请求，返回响应体与状态码（非 2xx 也可带体返回，由调用方判断）。
func (c *Client) doGet(ctx context.Context, endpoint, referer string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", mobileUA)
	req.Header.Set("Referer", referer)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}
