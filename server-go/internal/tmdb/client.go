// Package tmdb 封装 TMDB API 客户端：鉴权、find、search、details、seasons。
// 对应 JS 版 tmdb.js 的全部导出函数。
package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// ErrNotFound 表示 TMDB 返回 404（对应 JS 版 tmdbFetch 在 404 时返回 null）。
var ErrNotFound = errors.New("TMDB resource not found (404)")

var preferTVRe = regexp.MustCompile(`剧|综艺|动漫|纪录`)

// preferTV 判断类别是否优先按电视剧匹配（对应 JS /剧|综艺|动漫|纪录/.test(category)）。
func preferTV(category string) bool { return preferTVRe.MatchString(category) }

// Client 是 TMDB API 客户端。
type Client struct {
	accessToken string // v4 Bearer Token
	apiKey      string // v3 API Key
	base        string
	http        *http.Client
}

// NewClient 创建客户端。鉴权优先使用 accessToken，其次 apiKey。
func NewClient(accessToken, apiKey string) *Client {
	return &Client{
		accessToken: accessToken,
		apiKey:      apiKey,
		base:        "https://api.themoviedb.org/3",
		http:        &http.Client{Timeout: 30 * time.Second},
	}
}

// Configured 是否配置了任一 TMDB 凭证（对应 tmdbConfigured）。
func (c *Client) Configured() bool {
	return c.accessToken != "" || c.apiKey != ""
}

// fetch 发起 TMDB 请求。404 返回 (nil, ErrNotFound)；其余非 2xx 返回错误；
// 成功返回响应体字节（对应 tmdbFetch）。
func (c *Client) fetch(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	u := c.base + path
	q := url.Values{}
	q.Set("language", "zh-CN")
	// authQuery：仅当无 accessToken 且有 apiKey 时附加 api_key
	if c.accessToken == "" && c.apiKey != "" {
		q.Set("api_key", c.apiKey)
	}
	for k, v := range params {
		q.Set(k, v)
	}
	full := u + "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("accept", "application/json")
	// authHeaders：有 accessToken 时附加 Bearer
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return nil, fmt.Errorf("TMDB %d %s: %s", resp.StatusCode, path, string(body))
	}
	return io.ReadAll(resp.Body)
}

// FindByImdb 按 IMDb 号查找（/find）。根据 category 决定优先 movie 还是 tv。
// 返回 *Found 或 nil（未匹配 / 404）。
func (c *Client) FindByImdb(ctx context.Context, imdbID, category string) (*Found, error) {
	var resp struct {
		MovieResults []struct {
			ID int64 `json:"id"`
		} `json:"movie_results"`
		TVResults []struct {
			ID int64 `json:"id"`
		} `json:"tv_results"`
	}
	data, err := c.fetch(ctx, "/find/"+url.PathEscape(imdbID), map[string]string{
		"external_source": "imdb_id",
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	pick := func(id int64, mt string) *Found { return &Found{TmdbID: id, MediaType: mt} }
	if preferTV(category) {
		if len(resp.TVResults) > 0 {
			return pick(resp.TVResults[0].ID, "tv"), nil
		}
		if len(resp.MovieResults) > 0 {
			return pick(resp.MovieResults[0].ID, "movie"), nil
		}
	} else {
		if len(resp.MovieResults) > 0 {
			return pick(resp.MovieResults[0].ID, "movie"), nil
		}
		if len(resp.TVResults) > 0 {
			return pick(resp.TVResults[0].ID, "tv"), nil
		}
	}
	return nil, nil
}

// GetDetails 拉取详情（含 external_ids/credits/release_dates/content_ratings/keywords/watch/providers）。
// 404 返回 (nil, nil)（对应 JS tmdbFetch 404→null）。
func (c *Client) GetDetails(ctx context.Context, tmdbID int64, mediaType string) (*Details, error) {
	path := "/movie/" + strconv.FormatInt(tmdbID, 10)
	if mediaType == "tv" {
		path = "/tv/" + strconv.FormatInt(tmdbID, 10)
	}
	data, err := c.fetch(ctx, path, map[string]string{
		"append_to_response": "external_ids,credits,release_dates,content_ratings,keywords,watch/providers",
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var d Details
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// GetSeasons 获取某 TV 剧集的季列表（轻量请求，不附带 credits）。
// 404 返回空切片（对应 JS data 为 null 时 return []）。
func (c *Client) GetSeasons(ctx context.Context, tmdbID int64) ([]Season, error) {
	data, err := c.fetch(ctx, "/tv/"+strconv.FormatInt(tmdbID, 10), nil)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return []Season{}, nil
		}
		return nil, err
	}
	var resp struct {
		Seasons []struct {
			SeasonNumber *int64 `json:"season_number"`
			Name         string `json:"name"`
			AirDate      string `json:"air_date"`
			EpisodeCount int64  `json:"episode_count"`
			PosterPath   string `json:"poster_path"`
			Overview     string `json:"overview"`
		} `json:"seasons"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	out := make([]Season, 0, len(resp.Seasons))
	for _, s := range resp.Seasons {
		name := s.Name
		if name == "" {
			if s.SeasonNumber != nil {
				if *s.SeasonNumber == 0 {
					name = "特别篇"
				} else {
					name = fmt.Sprintf("第 %d 季", *s.SeasonNumber)
				}
			}
		}
		out = append(out, Season{
			SeasonNumber: s.SeasonNumber,
			Name:         name,
			AirDate:      s.AirDate,
			EpisodeCount: s.EpisodeCount,
			PosterPath:   s.PosterPath,
			Overview:     s.Overview,
		})
	}
	return out, nil
}

// GetSeasonDetails 拉取某 TV 剧集指定季的详情（含 credits）。404 返回 (nil, nil)。
func (c *Client) GetSeasonDetails(ctx context.Context, tmdbID int64, seasonNumber int) (*SeasonDetails, error) {
	data, err := c.fetch(ctx, fmt.Sprintf("/tv/%d/season/%d", tmdbID, seasonNumber), map[string]string{
		"append_to_response": "credits,external_ids",
	})
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var sd SeasonDetails
	if err := json.Unmarshal(data, &sd); err != nil {
		return nil, err
	}
	return &sd, nil
}

// searchOne 发起单类型搜索，失败时返回 (nil, err)（由调用方忽略 error，模拟 allSettled）。
func (c *Client) searchOne(ctx context.Context, mediaType, endpoint, query string) ([]SearchResult, error) {
	data, err := c.fetch(ctx, endpoint, map[string]string{
		"query": query,
		"page":  "1",
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Results []struct {
			ID            int64  `json:"id"`
			Name          string `json:"name"`
			OriginalName  string `json:"original_name"`
			FirstAirDate  string `json:"first_air_date"`
			Title         string `json:"title"`
			OriginalTitle string `json:"original_title"`
			ReleaseDate   string `json:"release_date"`
			PosterPath    string `json:"poster_path"`
			Overview      string `json:"overview"`
		} `json:"results"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, len(resp.Results))
	for _, m := range resp.Results {
		var title, orig, dateStr string
		if mediaType == "tv" {
			title, orig, dateStr = m.Name, m.OriginalName, m.FirstAirDate
		} else {
			title, orig, dateStr = m.Title, m.OriginalTitle, m.ReleaseDate
		}
		var year *string
		if len(dateStr) >= 4 {
			y := dateStr[:4]
			year = &y
		}
		out = append(out, SearchResult{
			TmdbID:        m.ID,
			MediaType:     mediaType,
			Title:         title,
			OriginalTitle: orig,
			ReleaseYear:   year,
			PosterPath:    m.PosterPath,
			Overview:      m.Overview,
		})
	}
	return out, nil
}

// SearchByName 按名称搜索（movie + tv 合并候选），对应 searchByName。
// movie/tv 并发请求，任一失败则该部分为空（模拟 Promise.allSettled）。
// preferTv 时 tv 结果在前，否则 movie 在前；按 media_type:tmdb_id 去重。
func (c *Client) SearchByName(ctx context.Context, query, category string) ([]SearchResult, error) {
	var (
		movies, tvs []SearchResult
		wg          sync.WaitGroup
	)
	wg.Add(2)
	go func() { defer wg.Done(); movies, _ = c.searchOne(ctx, "movie", "/search/movie", query) }()
	go func() { defer wg.Done(); tvs, _ = c.searchOne(ctx, "tv", "/search/tv", query) }()
	wg.Wait()

	var first, second []SearchResult
	if preferTV(category) {
		first, second = tvs, movies
	} else {
		first, second = movies, tvs
	}

	list := make([]SearchResult, 0, len(first)+len(second))
	seen := map[string]bool{}
	add := func(items []SearchResult) {
		for _, x := range items {
			k := x.MediaType + ":" + strconv.FormatInt(x.TmdbID, 10)
			if seen[k] {
				continue
			}
			seen[k] = true
			list = append(list, x)
		}
	}
	add(first)
	add(second)
	return list, nil
}
