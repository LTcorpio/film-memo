package tmdb

import (
	"encoding/json"
	"time"

	"film-memo/internal/model"
)

// rawCountry 用于序列化 production_countries（键名 iso/name，与 JS 一致）。
type rawCountry struct {
	ISO  string `json:"iso"`
	Name string `json:"name"`
}

// rawSpokenLang 用于序列化 spoken_languages。
type rawSpokenLang struct {
	ISO         string `json:"iso"`
	Name        string `json:"name"`
	EnglishName string `json:"english_name"`
}

// rawCompany 用于序列化 production_companies。
type rawCompany struct {
	ID            *int64 `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

// jsonMarshal 序列化为字符串，失败返回 "null"。
func jsonMarshal(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}

// byJobs 从 crew 中按职位提取去重后的姓名列表（对应 normalizeDetails 里的 byJobs）。
func byJobs(crew []CrewMember, jobs ...string) []string {
	set := map[string]bool{}
	out := []string{}
	for _, c := range crew {
		if c.Name == "" {
			continue
		}
		matched := false
		for _, j := range jobs {
			if c.Job == j {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if !set[c.Name] {
			set[c.Name] = true
			out = append(out, c.Name)
		}
	}
	return out
}

// NormalizeDetails 把 TMDB 详情整理为可入库的元数据对象（对应 normalizeDetails）。
// details 为 nil 时返回 nil。
func NormalizeDetails(d *Details, mediaType string) *model.Metadata {
	if d == nil {
		return nil
	}

	// genres
	genres := make([]string, 0, len(d.Genres))
	for _, g := range d.Genres {
		if g.Name != "" {
			genres = append(genres, g.Name)
		}
	}

	// countries
	countries := make([]rawCountry, 0, len(d.ProductionCountries))
	for _, c := range d.ProductionCountries {
		countries = append(countries, rawCountry{ISO: c.ISO31661, Name: c.Name})
	}

	// runtime：tv 取 episode_run_time[0]；movie 取 details.runtime；0/缺失→nil
	var runtime *int64
	if mediaType == "tv" {
		if len(d.EpisodeRunTime) > 0 && d.EpisodeRunTime[0] != 0 {
			v := d.EpisodeRunTime[0]
			runtime = &v
		}
	} else {
		if d.Runtime != nil && *d.Runtime != 0 {
			runtime = d.Runtime
		}
	}

	crew := d.Credits.Crew

	// 导演：tv 用 created_by；movie 用 crew.job=Director
	var directors []string
	if mediaType == "tv" {
		directors = make([]string, 0, len(d.CreatedBy))
		for _, p := range d.CreatedBy {
			if p.Name != "" {
				directors = append(directors, p.Name)
			}
		}
	} else {
		directors = byJobs(crew, "Director")
	}
	writers := byJobs(crew, "Writer", "Screenplay", "Story", "Novel")
	cinematographers := byJobs(crew, "Director of Photography", "Cinematography")
	composers := byJobs(crew, "Original Music Composer", "Music", "Composer")
	producers := byJobs(crew, "Producer", "Executive Producer", "Co-Producer", "Associate Producer")

	// 主要演员（前 12）
	cast := make([]string, 0, 12)
	for i, c := range d.Credits.Cast {
		if i >= 12 {
			break
		}
		if c.Name != "" {
			cast = append(cast, c.Name)
		}
	}

	// 制片公司（过滤无 name 的项）
	companies := make([]rawCompany, 0, len(d.ProductionCompanies))
	for _, c := range d.ProductionCompanies {
		if c.Name == "" {
			continue
		}
		var id *int64
		if c.ID != nil {
			id = c.ID
		}
		companies = append(companies, rawCompany{
			ID:            id,
			Name:          c.Name,
			LogoPath:      c.LogoPath,
			OriginCountry: c.OriginCountry,
		})
	}

	// 关键词
	keywords := make([]string, 0, len(d.Keywords.Results))
	for _, k := range d.Keywords.Results {
		if k.Name != "" {
			keywords = append(keywords, k.Name)
		}
	}

	// 对白语言
	spoken := make([]rawSpokenLang, 0, len(d.SpokenLanguages))
	for _, l := range d.SpokenLanguages {
		spoken = append(spoken, rawSpokenLang{
			ISO:         l.ISO6391,
			Name:        l.Name,
			EnglishName: l.EnglishName,
		})
	}

	// 出品国家代码
	originCountry := d.OriginCountry
	if originCountry == nil {
		originCountry = []string{}
	}

	// 内容分级（优先 US）
	var contentRating string
	if mediaType == "tv" {
		for _, r := range d.ContentRatings.Results {
			if r.ISO31661 == "US" {
				contentRating = r.Rating
				break
			}
		}
		if contentRating == "" && len(d.ContentRatings.Results) > 0 {
			contentRating = d.ContentRatings.Results[0].Rating
		}
	} else {
		var usCert string
		usFound := false
		for _, r := range d.ReleaseDates.Results {
			if r.ISO31661 == "US" {
				usFound = true
				if len(r.ReleaseDates) > 0 {
					usCert = r.ReleaseDates[0].Certification
				}
				break
			}
		}
		if usCert != "" {
			contentRating = usCert
		} else if usFound {
			// US 存在但无 certification，回退到首条
			if len(d.ReleaseDates.Results) > 0 && len(d.ReleaseDates.Results[0].ReleaseDates) > 0 {
				contentRating = d.ReleaseDates.Results[0].ReleaseDates[0].Certification
			}
		} else {
			if len(d.ReleaseDates.Results) > 0 && len(d.ReleaseDates.Results[0].ReleaseDates) > 0 {
				contentRating = d.ReleaseDates.Results[0].ReleaseDates[0].Certification
			}
		}
	}

	// 上映/首播日期
	var releaseDate string
	if mediaType == "tv" {
		releaseDate = d.FirstAirDate
	} else {
		releaseDate = d.ReleaseDate
	}

	// 标题
	var title, originalTitle string
	if mediaType == "tv" {
		title = d.Name
		originalTitle = d.OriginalName
	} else {
		title = d.Title
		originalTitle = d.OriginalTitle
	}

	// budget/revenue（movie: details.budget||0；tv: nil）
	var budget, revenue *int64
	if mediaType == "movie" {
		var b int64
		if d.Budget != nil {
			b = *d.Budget
		}
		budget = &b
		var rv int64
		if d.Revenue != nil {
			rv = *d.Revenue
		}
		revenue = &rv
	}

	// number_of_seasons/episodes（tv）
	var nos, noe *int64
	if mediaType == "tv" {
		nos = d.NumberOfSeasons
		noe = d.NumberOfEpisodes
	}

	return &model.Metadata{
		ImdbID:              d.ExternalIDs.ImdbID,
		TmdbID:              d.ID,
		MediaType:           mediaType,
		Title:               title,
		OriginalTitle:       originalTitle,
		Overview:            d.Overview,
		PosterPath:          d.PosterPath,
		BackdropPath:        d.BackdropPath,
		Genres:              jsonMarshal(genres),
		ProductionCountries: jsonMarshal(countries),
		Runtime:             runtime,
		VoteAverage:         d.VoteAverage,
		VoteCount:           d.VoteCount,
		Directors:           jsonMarshal(directors),
		Cast:                jsonMarshal(cast),
		ReleaseDate:         releaseDate,
		Status:              d.Status,
		Tagline:             d.Tagline,
		OriginalLanguage:    d.OriginalLanguage,
		SpokenLanguages:     jsonMarshal(spoken),
		OriginCountry:       jsonMarshal(originCountry),
		ProductionCompanies: jsonMarshal(companies),
		Writers:             jsonMarshal(writers),
		Cinematographers:    jsonMarshal(cinematographers),
		Composers:           jsonMarshal(composers),
		Producers:           jsonMarshal(producers),
		Keywords:            jsonMarshal(keywords),
		NumberOfSeasons:     nos,
		NumberOfEpisodes:    noe,
		Budget:              budget,
		Revenue:             revenue,
		ContentRating:       contentRating,
		Homepage:            d.Homepage,
		UpdatedAt:           time.Now().UTC().Format("2006-01-02T15:04:05.000Z07:00"),
	}
}
