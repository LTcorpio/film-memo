/**
 * TMDB API 客户端：通过 IMDb 号查找 / 按名称搜索 / 拉取详情。
 * 鉴权：优先使用 v4 Read Access Token (Bearer)，其次 v3 API Key。
 */
import 'dotenv/config';

const TMDB_BASE = 'https://api.themoviedb.org/3';
const IMG_BASE = 'https://image.tmdb.org/t/p';

export function tmdbConfigured() {
  return Boolean(process.env.TMDB_ACCESS_TOKEN || process.env.TMDB_API_KEY);
}

function authHeaders() {
  if (process.env.TMDB_ACCESS_TOKEN) {
    return { Authorization: `Bearer ${process.env.TMDB_ACCESS_TOKEN}` };
  }
  return {};
}

function authQuery() {
  if (!process.env.TMDB_ACCESS_TOKEN && process.env.TMDB_API_KEY) {
    return { api_key: process.env.TMDB_API_KEY };
  }
  return {};
}

async function tmdbFetch(path, params = {}) {
  const url = new URL(`${TMDB_BASE}${path}`);
  Object.entries({ language: 'zh-CN', ...authQuery(), ...params }).forEach(
    ([k, v]) => v != null && url.searchParams.set(k, v)
  );
  const res = await fetch(url, { headers: { accept: 'application/json', ...authHeaders() } });
  if (res.status === 404) return null;
  if (!res.ok) {
    const text = await res.text().catch(() => '');
    throw new Error(`TMDB ${res.status} ${url.pathname}: ${text.slice(0, 200)}`);
  }
  return res.json();
}

/** 图片完整 URL */
export function imageUrl(path, size = 'w500') {
  if (!path) return null;
  return `${IMG_BASE}/${size}${path}`;
}

/**
 * 按 IMDb 号查找（/find）。根据 category 决定优先 movie 还是 tv。
 * 返回 { tmdb_id, media_type } 或 null。
 */
export async function findByImdb(imdbId, category = '') {
  const data = await tmdbFetch(`/find/${encodeURIComponent(imdbId)}`, {
    external_source: 'imdb_id',
  });
  if (!data) return null;
  const movies = data.movie_results || [];
  const tvs = data.tv_results || [];
  const preferTv = /剧|综艺|动漫|纪录/.test(category);
  const pick = preferTv
    ? (tvs[0] || movies[0])
    : (movies[0] || tvs[0]);
  if (!pick) return null;
  return { tmdb_id: pick.id, media_type: pick.id && movies.includes(pick) ? 'movie' : 'tv' };
}

/** 拉取详情（含 external_ids / credits / release_dates / content_ratings / keywords 等） */
export async function getDetails(tmdbId, mediaType) {
  const path = mediaType === 'tv' ? `/tv/${tmdbId}` : `/movie/${tmdbId}`;
  return tmdbFetch(path, {
    append_to_response: 'external_ids,credits,release_dates,content_ratings,keywords,watch/providers',
  });
}

/** 把 TMDB 详情整理为可入库的元数据对象 */
export function normalizeDetails(details, mediaType) {
  if (!details) return null;
  const ext = details.external_ids || {};
  const genres = (details.genres || []).map((g) => g.name).filter(Boolean);
  const countries = (details.production_countries || []).map((c) => ({
    iso: c.iso_3166_1,
    name: c.name,
  }));
  const runtime = mediaType === 'tv'
    ? (Array.isArray(details.episode_run_time) ? (details.episode_run_time[0] || null) : details.runtime)
    : details.runtime;

  const crew = details.credits?.crew || [];

  // 从 crew 中按职位提取各主创
  const byJobs = (jobs) => {
    const set = new Set();
    for (const c of crew) {
      if (jobs.includes(c.job)) set.add(c.name);
    }
    return [...set].filter(Boolean);
  };
  // 导演：movie 用 crew.job=Director，tv 用 created_by
  const directors = mediaType === 'tv'
    ? (details.created_by || []).map((p) => p.name).filter(Boolean)
    : byJobs(['Director']);
  // 编剧：Writer / Screenplay / Story
  const writers = byJobs(['Writer', 'Screenplay', 'Story', 'Novel']);
  // 摄影指导
  const cinematographers = byJobs(['Director of Photography', 'Cinematography']);
  // 配乐
  const composers = byJobs(['Original Music Composer', 'Music', 'Composer']);
  // 制片人
  const producers = byJobs(['Producer', 'Executive Producer', 'Co-Producer', 'Associate Producer']);
  // 主要演员（前 12）
  const cast = (details.credits?.cast || [])
    .slice(0, 12)
    .map((c) => c.name)
    .filter(Boolean);

  // 制片公司
  const companies = (details.production_companies || []).map((c) => ({
    id: c.id || null,
    name: c.name,
    logo_path: c.logo_path || null,
    origin_country: c.origin_country || null,
  })).filter((c) => c.name);

  // 关键词
  const keywords = ((details.keywords?.results) || [])
    .map((k) => k.name)
    .filter(Boolean);

  // 对白语言
  const spokenLanguages = (details.spoken_languages || []).map((l) => ({
    iso: l.iso_639_1 || null,
    name: l.name || null,
    english_name: l.english_name || null,
  }));

  // 出品国家代码
  const originCountry = details.origin_country || (mediaType === 'tv'
    ? (details.origin_country || [])
    : (details.origin_country || []));

  // 内容分级（优先 US）
  let contentRating = null;
  if (mediaType === 'tv') {
    const crs = details.content_ratings?.results || [];
    const us = crs.find((r) => r.iso_3166_1 === 'US');
    contentRating = us?.rating || crs[0]?.rating || null;
  } else {
    const rds = details.release_dates?.results || [];
    const us = rds.find((r) => r.iso_3166_1 === 'US');
    const cert = us?.release_dates?.[0]?.certification
      || rds[0]?.release_dates?.[0]?.certification;
    contentRating = cert || null;
  }

  // 上映/首播日期
  const releaseDate = mediaType === 'tv'
    ? (details.first_air_date || null)
    : (details.release_date || null);

  return {
    imdb_id: ext.imdb_id || null,
    tmdb_id: details.id,
    media_type: mediaType,
    title: mediaType === 'tv' ? details.name : details.title,
    original_title: mediaType === 'tv' ? details.original_name : details.original_title,
    overview: details.overview || null,
    poster_path: details.poster_path || null,
    backdrop_path: details.backdrop_path || null,
    genres: JSON.stringify(genres),
    production_countries: JSON.stringify(countries),
    runtime: runtime ? Number(runtime) : null,
    vote_average: details.vote_average ?? null,
    vote_count: details.vote_count ?? null,
    directors: JSON.stringify(directors),
    cast: JSON.stringify(cast),
    release_date: releaseDate || null,
    status: details.status || null,
    tagline: details.tagline || null,
    // 扩展元数据
    original_language: details.original_language || null,
    spoken_languages: JSON.stringify(spokenLanguages),
    origin_country: JSON.stringify(originCountry),
    production_companies: JSON.stringify(companies),
    writers: JSON.stringify(writers),
    cinematographers: JSON.stringify(cinematographers),
    composers: JSON.stringify(composers),
    producers: JSON.stringify(producers),
    keywords: JSON.stringify(keywords),
    number_of_seasons: mediaType === 'tv' ? (details.number_of_seasons ?? null) : null,
    number_of_episodes: mediaType === 'tv' ? (details.number_of_episodes ?? null) : null,
    budget: mediaType === 'movie' ? (details.budget || 0) : null,
    revenue: mediaType === 'movie' ? (details.revenue || 0) : null,
    content_rating: contentRating,
    homepage: details.homepage || null,
    updated_at: new Date().toISOString(),
  };
}

/**
 * 按名称搜索（movie + tv 合并候选）。
 * 返回 [{ tmdb_id, media_type, title, original_title, release_year, poster_path, overview }]
 */
export async function searchByName(query, category = '') {
  const preferTv = /剧|综艺|动漫|纪录/.test(category);
  const [movieRes, tvRes] = await Promise.allSettled([
    tmdbFetch('/search/movie', { query, page: 1 }),
    tmdbFetch('/search/tv', { query, page: 1 }),
  ]);
  const movies = movieRes.status === 'fulfilled' ? (movieRes.value?.results || []) : [];
  const tvs = tvRes.status === 'fulfilled' ? (tvRes.value?.results || []) : [];
  const fmt = (m, type) => ({
    tmdb_id: m.id,
    media_type: type,
    title: type === 'tv' ? m.name : m.title,
    original_title: type === 'tv' ? m.original_name : m.original_title,
    release_year: (((type === 'tv' ? m.first_air_date : m.release_date) || '').slice(0, 4)) || null,
    poster_path: m.poster_path || null,
    overview: m.overview || null,
  });
  const list = [
    ...(preferTv ? tvs : movies).map((m) => fmt(m, preferTv ? 'tv' : 'movie')),
    ...(preferTv ? movies : tvs).map((m) => fmt(m, preferTv ? 'movie' : 'tv')),
  ];
  // 去重（按 media_type+tmdb_id）
  const seen = new Set();
  return list.filter((x) => {
    const k = `${x.media_type}:${x.tmdb_id}`;
    if (seen.has(k)) return false;
    seen.add(k);
    return true;
  });
}

export { tmdbFetch };
