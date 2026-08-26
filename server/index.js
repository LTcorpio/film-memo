/**
 * Express 后端：观影记录查询/筛选 + TMDB 元数据搜索与填充。
 * 数据模型 v2：films 存影视级数据（同一影视一份），viewings 存观看记录（可多条）。
 * 支持编辑影视信息/观看记录/元数据、图片本地化存储与替换。
 */
import express from 'express';
import cors from 'cors';
import { fileURLToPath } from 'node:url';
import { dirname, join, extname } from 'node:path';
import { existsSync, mkdirSync, writeFileSync, unlinkSync } from 'node:fs';
import dotenv from 'dotenv';
const __dirname = dirname(fileURLToPath(import.meta.url));
dotenv.config({ path: join(__dirname, '..', '.env') });

import { db } from './db.js';
import { tmdbConfigured, getDetails, normalizeDetails, searchByName, imageUrl, getSeasons, getSeasonDetails } from './tmdb.js';

const app = express();
app.use(cors());
app.use(express.json({ limit: '12mb' }));

const PORT = process.env.PORT || 4000;

// 本地图片目录：优先从环境变量读取，否则默认项目根 data/images/
const IMAGES_DIR = process.env.IMAGES_DIR || join(__dirname, '..', 'data', 'images');
mkdirSync(IMAGES_DIR, { recursive: true });
// 静态托管本地图片：/images/{file} → data/images/{file}
app.use('/images', express.static(IMAGES_DIR, { maxAge: '7d', immutable: true }));

const META_COLS = `m.film_id AS m_film_id, m.imdb_id AS m_imdb_id, m.tmdb_id AS m_tmdb_id, m.media_type AS m_media_type,
  m.title AS m_title, m.original_title AS m_original_title, m.overview AS m_overview,
  m.poster_path AS m_poster_path, m.backdrop_path AS m_backdrop_path,
  m.poster_local AS m_poster_local, m.backdrop_local AS m_backdrop_local,
  m.genres AS m_genres, m.production_countries AS m_countries, m.runtime AS m_runtime,
  m.vote_average AS m_vote_average, m.vote_count AS m_vote_count,
  m.directors AS m_directors, m.cast AS m_cast, m.release_date AS m_release_date,
  m.status AS m_status, m.tagline AS m_tagline, m.updated_at AS m_updated_at,
  m.original_language AS m_original_language, m.spoken_languages AS m_spoken_languages,
  m.origin_country AS m_origin_country, m.production_companies AS m_production_companies,
  m.writers AS m_writers, m.cinematographers AS m_cinematographers,
  m.composers AS m_composers, m.producers AS m_producers, m.keywords AS m_keywords,
  m.number_of_seasons AS m_number_of_seasons, m.number_of_episodes AS m_number_of_episodes,
  m.budget AS m_budget, m.revenue AS m_revenue,
  m.content_rating AS m_content_rating, m.homepage AS m_homepage`;

// films 表影视级字段 / viewings 表观看级字段
const FILM_FIELDS = ['category', 'name', 'imdb_id', 'douban_id', 'production_countries_raw', 'release_year', 'total_episodes'];
const VIEWING_FIELDS = ['watch_year', 'start_date', 'end_date', 'platforms_raw', 'location', 'notes'];

function localImageUrl(file) {
  if (!file) return null;
  return `/images/${file}`;
}

/** 解析观看平台原始字段（按 "," 分割） */
function parsePlatforms(raw) {
  return String(raw || '')
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
}

/** 制片国家：优先元数据，否则用 Excel 字段按 "/" 拆分 */
function parseCountries(row) {
  let countries = [];
  if (row.m_countries) {
    try { countries = JSON.parse(row.m_countries).map((c) => c.name).filter(Boolean); } catch {}
  }
  if (!countries.length && row.production_countries_raw) {
    countries = row.production_countries_raw.split('/').map((s) => s.trim()).filter(Boolean);
  }
  return countries;
}

/** 把元数据行整理为前端友好的对象（多值字段拆数组） */
function shapeMetadata(row) {
  if (!row.m_film_id) return null;
  const parseJson = (raw, fallback = []) => {
    if (!raw) return fallback;
    try { return JSON.parse(raw); } catch { return fallback; }
  };
  return {
    tmdbId: row.m_tmdb_id,
    mediaType: row.m_media_type,
    title: row.m_title,
    originalTitle: row.m_original_title,
    overview: row.m_overview,
    posterPath: row.m_poster_path,
    backdropPath: row.m_backdrop_path,
    posterLocal: row.m_poster_local,
    backdropLocal: row.m_backdrop_local,
    posterUrl: localImageUrl(row.m_poster_local) || imageUrl(row.m_poster_path, 'w500'),
    backdropUrl: localImageUrl(row.m_backdrop_local) || imageUrl(row.m_backdrop_path, 'w1280'),
    genres: parseJson(row.m_genres),
    runtime: row.m_runtime,
    voteAverage: row.m_vote_average,
    voteCount: row.m_vote_count,
    directors: parseJson(row.m_directors),
    cast: parseJson(row.m_cast),
    releaseDate: row.m_release_date,
    status: row.m_status,
    tagline: row.m_tagline,
    updatedAt: row.m_updated_at,
    // 扩展元数据
    originalLanguage: row.m_original_language,
    spokenLanguages: parseJson(row.m_spoken_languages),
    originCountry: parseJson(row.m_origin_country),
    productionCompanies: parseJson(row.m_production_companies),
    writers: parseJson(row.m_writers),
    cinematographers: parseJson(row.m_cinematographers),
    composers: parseJson(row.m_composers),
    producers: parseJson(row.m_producers),
    keywords: parseJson(row.m_keywords),
    numberOfSeasons: row.m_number_of_seasons,
    numberOfEpisodes: row.m_number_of_episodes,
    budget: row.m_budget,
    revenue: row.m_revenue,
    contentRating: row.m_content_rating,
    homepage: row.m_homepage,
  };
}

/** 观看记录行 → 前端对象 */
function shapeViewing(v) {
  return {
    id: v.id,
    watchYear: v.watch_year,
    startDate: v.start_date,
    endDate: v.end_date,
    platforms: parsePlatforms(v.platforms_raw),
    location: v.location,
    notes: v.notes,
  };
}

/** 列表条目：一条观看记录 + 所属影视信息（字段名与旧版保持一致，卡片/表格视图无需改动） */
function shapeEntry(row) {
  return {
    id: row.id, // 观看记录 id
    filmId: row.film_id,
    watchYear: row.watch_year,
    startDate: row.start_date,
    endDate: row.end_date,
    platforms: parsePlatforms(row.platforms_raw),
    location: row.location,
    notes: row.notes,
    category: row.category,
    name: row.name,
    imdbId: row.imdb_id,
    doubanId: row.douban_id,
    releaseYear: row.release_year,
    totalEpisodes: row.total_episodes,
    productionCountries: parseCountries(row),
    productionCountriesRaw: row.production_countries_raw,
    hasMetadata: Boolean(row.m_tmdb_id),
    metadata: shapeMetadata(row),
  };
}

/** 影视详情：影视级信息 + 全部观看记录 */
function shapeFilm(row, viewingRows) {
  return {
    id: row.id,
    name: row.name,
    category: row.category,
    imdbId: row.imdb_id,
    doubanId: row.douban_id,
    releaseYear: row.release_year,
    totalEpisodes: row.total_episodes,
    productionCountries: parseCountries(row),
    productionCountriesRaw: row.production_countries_raw,
    hasMetadata: Boolean(row.m_tmdb_id),
    metadata: shapeMetadata(row),
    viewings: viewingRows.map(shapeViewing),
  };
}

const FILM_BY_ID_SQL = `SELECT f.id, f.category, f.name, f.imdb_id, f.douban_id,
  f.production_countries_raw, f.release_year, f.total_episodes, ${META_COLS}
  FROM films f LEFT JOIN film_metadata m ON m.film_id = f.id WHERE f.id = ?`;

// ---- 列表/筛选（每条观看记录一个条目，展示与旧版一致） ----
app.get('/api/films', (req, res) => {
  const { watchYear, releaseYear, platform, category, q, missing } = req.query;
  const where = [];
  const params = [];
  if (watchYear) { where.push('v.watch_year = ?'); params.push(Number(watchYear)); }
  if (releaseYear) { where.push('f.release_year = ?'); params.push(Number(releaseYear)); }
  // 特殊筛选：无元数据（未刮削 TMDB）
  if (category === '__no_meta__') {
    where.push('m.tmdb_id IS NULL');
  } else if (category) {
    where.push('f.category = ?'); params.push(category);
  }
  // 缺失值筛选：imdb / douban
  if (missing === 'imdb') {
    where.push("(f.imdb_id IS NULL OR TRIM(f.imdb_id) = '')");
  } else if (missing === 'douban') {
    where.push("(f.douban_id IS NULL OR TRIM(f.douban_id) = '')");
  }
  if (platform) {
    where.push("(',' || v.platforms_raw || ',') LIKE ? COLLATE NOCASE");
    params.push(`%,${platform},%`);
  }
  if (q) { where.push('f.name LIKE ? COLLATE NOCASE'); params.push(`%${q}%`); }

  const sql = `SELECT v.id, v.watch_year, v.start_date, v.end_date, v.platforms_raw, v.location, v.notes,
      f.id AS film_id, f.category, f.name, f.imdb_id, f.douban_id,
      f.production_countries_raw, f.release_year, f.total_episodes, ${META_COLS}
    FROM viewings v
    JOIN films f ON f.id = v.film_id
    LEFT JOIN film_metadata m ON m.film_id = f.id
    ${where.length ? 'WHERE ' + where.join(' AND ') : ''}
    ORDER BY v.watch_year DESC, v.start_date DESC NULLS LAST, v.id DESC`;
  const rows = db.prepare(sql).all(...params);
  res.json(rows.map(shapeEntry));
});

// ---- 影视详情（含全部观看记录） ----
app.get('/api/films/:id', (req, res) => {
  const row = db.prepare(FILM_BY_ID_SQL).get(req.params.id);
  if (!row) return res.status(404).json({ error: 'not found' });
  const viewings = db.prepare(
    'SELECT id, watch_year, start_date, end_date, platforms_raw, location, notes FROM viewings WHERE film_id = ? ORDER BY watch_year ASC NULLS LAST, start_date ASC NULLS LAST, id ASC'
  ).all(row.id);
  res.json(shapeFilm(row, viewings));
});

// ---- 筛选项（下拉数据） ----
app.get('/api/filters', (_req, res) => {
  const watchYears = db.prepare('SELECT DISTINCT watch_year AS v FROM viewings WHERE watch_year IS NOT NULL ORDER BY v DESC').all().map((r) => r.v);
  const releaseYears = db.prepare('SELECT DISTINCT release_year AS v FROM films WHERE release_year IS NOT NULL ORDER BY v DESC').all().map((r) => r.v);
  const categories = db.prepare('SELECT DISTINCT category AS v FROM films WHERE category IS NOT NULL ORDER BY v').all().map((r) => r.v);
  const platformRows = db.prepare("SELECT platforms_raw FROM viewings WHERE platforms_raw IS NOT NULL").all();
  const platformSet = new Set();
  for (const r of platformRows) {
    for (const p of String(r.platforms_raw).split(',')) {
      const s = p.trim();
      if (s) platformSet.add(s);
    }
  }
  const platforms = [...platformSet].sort((a, b) => a.localeCompare(b, 'zh-Hans'));
  res.json({ watchYears, releaseYears, categories, platforms });
});

// ---- 概览统计（以观看记录为计数单位，与列表筛选口径一致） ----
app.get('/api/stats', (_req, res) => {
  const total = db.prepare('SELECT COUNT(*) AS c FROM viewings').get().c;
  const byCategory = db.prepare(`SELECT f.category AS k, COUNT(*) AS c FROM viewings v
    JOIN films f ON f.id = v.film_id GROUP BY f.category ORDER BY c DESC`).all();
  const byWatchYear = db.prepare('SELECT watch_year AS k, COUNT(*) AS c FROM viewings WHERE watch_year IS NOT NULL GROUP BY watch_year ORDER BY k').all();
  // 已刮削 TMDB 元数据 = 有 tmdb_id；无元数据 = tmdb_id 为空（含无元数据行）
  const withMeta = db.prepare(`SELECT COUNT(*) AS c FROM viewings v
    JOIN films f ON f.id = v.film_id
    LEFT JOIN film_metadata m ON m.film_id = f.id WHERE m.tmdb_id IS NOT NULL`).get().c;
  const withoutMeta = total - withMeta;
  const withoutImdb = db.prepare(`SELECT COUNT(*) AS c FROM viewings v
    JOIN films f ON f.id = v.film_id WHERE f.imdb_id IS NULL OR TRIM(f.imdb_id) = ''`).get().c;
  const withoutDouban = db.prepare(`SELECT COUNT(*) AS c FROM viewings v
    JOIN films f ON f.id = v.film_id WHERE f.douban_id IS NULL OR TRIM(f.douban_id) = ''`).get().c;
  res.json({ total, withMetadata: withMeta, withoutMetadata: withoutMeta, withoutImdb, withoutDouban, byCategory, byWatchYear });
});

// ---- 按名称搜索 TMDB 候选（手动填充） ----
app.get('/api/meta/search', async (req, res) => {
  const q = String(req.query.q || '').trim();
  if (!q) return res.json({ configured: tmdbConfigured(), results: [] });
  if (!tmdbConfigured()) {
    return res.status(400).json({ error: 'TMDB 未配置，请在 .env 设置 TMDB_ACCESS_TOKEN 或 TMDB_API_KEY' });
  }
  try {
    const results = (await searchByName(q, req.query.category || '')).map((r) => ({
      ...r,
      posterUrl: imageUrl(r.poster_path, 'w185'),
    }));
    res.json({ configured: true, results });
  } catch (e) {
    res.status(502).json({ error: e.message });
  }
});

// ---- 获取某 TMDB TV 剧集的季列表 ----
app.get('/api/meta/seasons', async (req, res) => {
  const tmdbId = Number(req.query.tmdbId);
  if (!tmdbId) return res.status(400).json({ error: '需要 tmdbId' });
  if (!tmdbConfigured()) {
    return res.status(400).json({ error: 'TMDB 未配置' });
  }
  try {
    const seasons = (await getSeasons(tmdbId)).map((s) => ({
      ...s,
      posterUrl: imageUrl(s.poster_path, 'w185'),
      year: s.air_date ? s.air_date.slice(0, 4) : null,
    }));
    res.json({ configured: true, seasons });
  } catch (e) {
    res.status(502).json({ error: e.message });
  }
});

/** 下载 TMDB 图片到本地，返回文件名（失败返回 null） */
async function downloadTmdbImage(path, filename) {
  if (!path) return null;
  // 原图为高质量
  const url = `https://image.tmdb.org/t/p/original${path}`;
  try {
    const r = await fetch(url);
    if (!r.ok) throw new Error(`img ${r.status}`);
    const buf = Buffer.from(await r.arrayBuffer());
    // 文件名含时间戳，避免重刮后浏览器仍使用旧缓存（immutable 静态资源）
    const file = `${filename}-${Date.now()}${extname(path) || '.jpg'}`;
    writeFileSync(join(IMAGES_DIR, file), buf);
    return file;
  } catch (e) {
    console.warn('图片下载失败:', url, e.message);
    return null;
  }
}

// ---- 选择某 TMDB 候选，拉详情并写入元数据（同时下载图片到本地） ----
app.post('/api/films/:id/metadata', async (req, res) => {
  const film = db.prepare('SELECT id, name, category, imdb_id FROM films WHERE id = ?').get(req.params.id);
  if (!film) return res.status(404).json({ error: 'film not found' });
  if (!tmdbConfigured()) {
    return res.status(400).json({ error: 'TMDB 未配置' });
  }
  const { tmdbId, mediaType, season } = req.body || {};
  if (!tmdbId || !mediaType) {
    return res.status(400).json({ error: '需要 tmdbId 与 mediaType' });
  }
  try {
    const details = await getDetails(tmdbId, mediaType);

    // TV 剧集：若指定了具体季，先拉取该季详情（含 credits），
    // 用季级数据覆盖 details 对应字段，使 normalizeDetails 产出季级演职员表 / 海报 / 首播日期 / 简介 / 集数
    if (mediaType === 'tv' && season != null && season !== '') {
      const seasonDetails = await getSeasonDetails(tmdbId, season);
      if (seasonDetails) {
        if (seasonDetails.credits) details.credits = seasonDetails.credits;
        if (seasonDetails.poster_path) details.poster_path = seasonDetails.poster_path;
        if (seasonDetails.air_date) details.first_air_date = seasonDetails.air_date;
        if (seasonDetails.overview && seasonDetails.overview.trim()) details.overview = seasonDetails.overview;
        const eps = Array.isArray(seasonDetails.episodes) ? seasonDetails.episodes.length : 0;
        if (eps > 0) details.number_of_episodes = eps;
      }
    }

    const meta = normalizeDetails(details, mediaType);
    if (!meta) return res.status(404).json({ error: 'TMDB 无详情' });

    // 下载海报/背景图到本地（保留原 poster_path/backdrop_path 以便回退远程）
    // 若 TMDB 无对应图或下载失败，保留用户已上传的本地图
    const existing = db.prepare('SELECT poster_local, backdrop_local FROM film_metadata WHERE film_id = ?').get(film.id);
    const newPosterLocal = await downloadTmdbImage(meta.poster_path, `${film.id}-poster`);
    const newBackdropLocal = await downloadTmdbImage(meta.backdrop_path, `${film.id}-backdrop`);
    if (newPosterLocal && existing?.poster_local && existing.poster_local !== newPosterLocal) {
      try { unlinkSync(join(IMAGES_DIR, existing.poster_local)); } catch {}
    }
    if (newBackdropLocal && existing?.backdrop_local && existing.backdrop_local !== newBackdropLocal) {
      try { unlinkSync(join(IMAGES_DIR, existing.backdrop_local)); } catch {}
    }
    const posterLocal = newPosterLocal || existing?.poster_local || null;
    const backdropLocal = newBackdropLocal || existing?.backdrop_local || null;
    db.prepare(`
      INSERT OR REPLACE INTO film_metadata
        (film_id, imdb_id, tmdb_id, media_type, title, original_title, overview,
         poster_path, backdrop_path, poster_local, backdrop_local,
         genres, production_countries, runtime, vote_average, vote_count,
         directors, cast, release_date, status, tagline,
         original_language, spoken_languages, origin_country, production_companies,
         writers, cinematographers, composers, producers, keywords,
         number_of_seasons, number_of_episodes, budget, revenue,
         content_rating, homepage, updated_at)
      VALUES (@film_id, @imdb_id, @tmdb_id, @media_type, @title, @original_title, @overview,
         @poster_path, @backdrop_path, @poster_local, @backdrop_local,
         @genres, @production_countries, @runtime, @vote_average, @vote_count,
         @directors, @cast, @release_date, @status, @tagline,
         @original_language, @spoken_languages, @origin_country, @production_companies,
         @writers, @cinematographers, @composers, @producers, @keywords,
         @number_of_seasons, @number_of_episodes, @budget, @revenue,
         @content_rating, @homepage, @updated_at)
    `).run({
      film_id: film.id, ...meta,
      poster_local: posterLocal,
      backdrop_local: backdropLocal,
    });
    if (!film.imdb_id && meta.imdb_id) {
      db.prepare('UPDATE films SET imdb_id = ? WHERE id = ?').run(meta.imdb_id, film.id);
    }
    res.json({ ok: true, tmdbId: meta.tmdb_id, imdbId: meta.imdb_id, posterLocal, backdropLocal });
  } catch (e) {
    res.status(502).json({ error: e.message });
  }
});

// ---- 编辑元数据（手动修改刮削后的字段） ----
app.put('/api/films/:id/metadata', (req, res) => {
  const film = db.prepare('SELECT id FROM films WHERE id = ?').get(req.params.id);
  if (!film) return res.status(404).json({ error: 'film not found' });
  const allowed = ['title', 'original_title', 'overview', 'runtime', 'vote_average', 'vote_count',
    'genres', 'production_countries', 'media_type', 'directors', 'cast', 'release_date', 'status', 'tagline',
    'original_language', 'spoken_languages', 'origin_country', 'production_companies',
    'writers', 'cinematographers', 'composers', 'producers', 'keywords',
    'number_of_seasons', 'number_of_episodes', 'budget', 'revenue',
    'content_rating', 'homepage'];
  const jsonFields = new Set(['genres', 'production_countries', 'directors', 'cast',
    'spoken_languages', 'origin_country', 'production_companies',
    'writers', 'cinematographers', 'composers', 'producers', 'keywords']);
  const sets = [];
  const params = {};
  for (const k of allowed) {
    if (k in (req.body || {})) {
      let v = req.body[k];
      if (jsonFields.has(k)) {
        v = Array.isArray(v) ? JSON.stringify(v) : v;
      }
      sets.push(`${k} = @${k}`);
      params[k] = v ?? null;
    }
  }
  if (!sets.length) return res.json({ ok: true, updated: false });
  sets.push(`updated_at = @updated_at`);
  params.updated_at = new Date().toISOString();
  params.film_id = film.id;
  // 若该影片尚无元数据，先插一条空记录再更新
  const exists = db.prepare('SELECT film_id FROM film_metadata WHERE film_id = ?').get(film.id);
  if (!exists) {
    db.prepare('INSERT INTO film_metadata (film_id) VALUES (?)').run(film.id);
  }
  db.prepare(`UPDATE film_metadata SET ${sets.join(', ')} WHERE film_id = @film_id`).run(params);
  res.json({ ok: true, updated: true });
});

// ---- 删除某影视的元数据（同时删除本地图片） ----
app.delete('/api/films/:id/metadata', (req, res) => {
  const row = db.prepare('SELECT poster_local, backdrop_local FROM film_metadata WHERE film_id = ?').get(req.params.id);
  if (row) {
    for (const f of [row.poster_local, row.backdrop_local]) {
      if (f) { try { unlinkSync(join(IMAGES_DIR, f)); } catch {} }
    }
  }
  db.prepare('DELETE FROM film_metadata WHERE film_id = ?').run(req.params.id);
  res.json({ ok: true });
});

/** 删除整部影视（观看记录 + 元数据 + 本地图片） */
function deleteFilmCascade(filmId) {
  const row = db.prepare('SELECT poster_local, backdrop_local FROM film_metadata WHERE film_id = ?').get(filmId);
  if (row) {
    for (const f of [row.poster_local, row.backdrop_local]) {
      if (f) { try { unlinkSync(join(IMAGES_DIR, f)); } catch {} }
    }
  }
  db.prepare('DELETE FROM viewings WHERE film_id = ?').run(filmId);
  db.prepare('DELETE FROM film_metadata WHERE film_id = ?').run(filmId);
  db.prepare('DELETE FROM films WHERE id = ?').run(filmId);
}

// ---- 删除整部影视及其全部观看记录 ----
app.delete('/api/films/:id', (req, res) => {
  const film = db.prepare('SELECT id FROM films WHERE id = ?').get(req.params.id);
  if (!film) return res.status(404).json({ error: 'film not found' });
  deleteFilmCascade(film.id);
  res.json({ ok: true });
});

// ---- 删除单条观看记录（若为该影视最后一条，则连同影视与元数据一并删除） ----
app.delete('/api/viewings/:id', (req, res) => {
  const v = db.prepare('SELECT id, film_id FROM viewings WHERE id = ?').get(req.params.id);
  if (!v) return res.status(404).json({ error: 'viewing not found' });
  db.prepare('DELETE FROM viewings WHERE id = ?').run(v.id);
  const remaining = db.prepare('SELECT COUNT(*) AS c FROM viewings WHERE film_id = ?').get(v.film_id).c;
  if (remaining === 0) {
    deleteFilmCascade(v.film_id);
    return res.json({ ok: true, filmDeleted: true });
  }
  res.json({ ok: true, filmDeleted: false });
});

// ---- 新增观影记录：同名影视已存在时追加一条观看记录，否则新建影视 ----
app.post('/api/films', (req, res) => {
  const b = req.body || {};
  const name = String(b.name || '').trim();
  if (!name) {
    return res.status(400).json({ error: '名称不能为空' });
  }
  const result = db.transaction(() => {
    // 按名称查找已有影视（忽略大小写与首尾空白）；IMDb 冲突时视为不同影视
    const existing = db.prepare('SELECT * FROM films WHERE TRIM(name) = TRIM(?) COLLATE NOCASE').get(name);
    const imdbConflict = existing && b.imdb_id && existing.imdb_id && b.imdb_id !== existing.imdb_id;

    let filmId;
    if (existing && !imdbConflict) {
      filmId = existing.id;
      // 回填影视级缺失字段
      const sets = [];
      const params = {};
      for (const k of FILM_FIELDS) {
        if (existing[k] == null && b[k] != null) {
          sets.push(`${k} = @${k}`);
          params[k] = b[k];
        }
      }
      if (sets.length) {
        params.id = filmId;
        db.prepare(`UPDATE films SET ${sets.join(', ')} WHERE id = @id`).run(params);
      }
    } else {
      const params = { name };
      for (const k of FILM_FIELDS) {
        if (k !== 'name' && k in b) params[k] = b[k] ?? null;
      }
      const cols = Object.keys(params);
      const placeholders = cols.map((c) => `@${c}`).join(', ');
      filmId = db.prepare(`INSERT INTO films (${cols.join(', ')}) VALUES (${placeholders})`).run(params).lastInsertRowid;
    }

    // 插入观看记录
    const vp = { film_id: filmId };
    for (const k of VIEWING_FIELDS) {
      if (k in b) vp[k] = b[k] ?? null;
    }
    db.prepare(`INSERT INTO viewings (${Object.keys(vp).join(', ')}) VALUES (${Object.keys(vp).map((c) => `@${c}`).join(', ')})`).run(vp);
    return filmId;
  })();

  const row = db.prepare(FILM_BY_ID_SQL).get(result);
  const viewings = db.prepare(
    'SELECT id, watch_year, start_date, end_date, platforms_raw, location, notes FROM viewings WHERE film_id = ? ORDER BY watch_year ASC NULLS LAST, start_date ASC NULLS LAST, id ASC'
  ).all(result);
  res.json(shapeFilm(row, viewings));
});

// ---- 编辑影视信息（films 表） ----
app.put('/api/films/:id', (req, res) => {
  const film = db.prepare('SELECT id FROM films WHERE id = ?').get(req.params.id);
  if (!film) return res.status(404).json({ error: 'film not found' });
  const sets = [];
  const params = {};
  for (const k of FILM_FIELDS) {
    if (k in (req.body || {})) {
      sets.push(`${k} = @${k}`);
      params[k] = req.body[k];
    }
  }
  if (!sets.length) return res.json({ ok: true, updated: false });
  params.id = film.id;
  db.prepare(`UPDATE films SET ${sets.join(', ')} WHERE id = @id`).run(params);
  res.json({ ok: true, updated: true });
});

// ---- 编辑观看记录（viewings 表） ----
app.put('/api/viewings/:id', (req, res) => {
  const v = db.prepare('SELECT id FROM viewings WHERE id = ?').get(req.params.id);
  if (!v) return res.status(404).json({ error: 'viewing not found' });
  const sets = [];
  const params = {};
  for (const k of VIEWING_FIELDS) {
    if (k in (req.body || {})) {
      sets.push(`${k} = @${k}`);
      params[k] = req.body[k];
    }
  }
  if (!sets.length) return res.json({ ok: true, updated: false });
  params.id = v.id;
  db.prepare(`UPDATE viewings SET ${sets.join(', ')} WHERE id = @id`).run(params);
  res.json({ ok: true, updated: true });
});

// ---- 上传/替换本地图片（raw body：Content-Type: image/*） ----
app.post('/api/films/:id/image', express.raw({ type: 'image/*', limit: '12mb' }), (req, res) => {
  const film = db.prepare('SELECT id FROM films WHERE id = ?').get(req.params.id);
  if (!film) return res.status(404).json({ error: 'film not found' });
  const type = req.query.type === 'backdrop' ? 'backdrop' : 'poster';
  if (!req.body || !req.body.length) return res.status(400).json({ error: '空图片数据' });
  const ct = req.get('content-type') || 'image/jpeg';
  const ext = { 'image/png': '.png', 'image/webp': '.webp', 'image/gif': '.gif', 'image/jpeg': '.jpg', 'image/jpg': '.jpg' }[ct] || '.jpg';
  // 删除旧本地文件
  const old = db.prepare(`SELECT ${type}_local AS f FROM film_metadata WHERE film_id = ?`).get(film.id);
  if (old?.f) { try { unlinkSync(join(IMAGES_DIR, old.f)); } catch {} }
  const file = `${film.id}-${type}-${Date.now()}${ext}`;
  writeFileSync(join(IMAGES_DIR, file), req.body);
  const exists = db.prepare('SELECT film_id FROM film_metadata WHERE film_id = ?').get(film.id);
  if (!exists) db.prepare('INSERT INTO film_metadata (film_id) VALUES (?)').run(film.id);
  db.prepare(`UPDATE film_metadata SET ${type}_local = ?, updated_at = ? WHERE film_id = ?`)
    .run(file, new Date().toISOString(), film.id);
  res.json({ ok: true, file, url: localImageUrl(file) });
});

// ---- 从 TMDB 远程下载图片到本地（poster/backdrop） ----
app.post('/api/films/:id/scrape-image', async (req, res) => {
  const film = db.prepare('SELECT id FROM films WHERE id = ?').get(req.params.id);
  if (!film) return res.status(404).json({ error: 'film not found' });
  const type = req.query.type === 'backdrop' ? 'backdrop' : 'poster';
  const meta = db.prepare(`SELECT poster_path, backdrop_path FROM film_metadata WHERE film_id = ?`).get(film.id);
  const path = type === 'backdrop' ? meta?.backdrop_path : meta?.poster_path;
  if (!path) return res.status(400).json({ error: 'TMDB 无对应图片路径，请先刮削元数据' });
  // 删除旧本地文件
  const old = db.prepare(`SELECT ${type}_local AS f FROM film_metadata WHERE film_id = ?`).get(film.id);
  if (old?.f) { try { unlinkSync(join(IMAGES_DIR, old.f)); } catch {} }
  const file = await downloadTmdbImage(path, `${film.id}-${type}`);
  if (!file) return res.status(502).json({ error: '图片下载失败' });
  db.prepare(`UPDATE film_metadata SET ${type}_local = ?, updated_at = ? WHERE film_id = ?`)
    .run(file, new Date().toISOString(), film.id);
  res.json({ ok: true, file, url: localImageUrl(file) });
});

// ---- 删除本地图片（回退到远程 TMDB 图） ----
app.delete('/api/films/:id/image', (req, res) => {
  const film = db.prepare('SELECT id FROM films WHERE id = ?').get(req.params.id);
  if (!film) return res.status(404).json({ error: 'film not found' });
  const type = req.query.type === 'backdrop' ? 'backdrop' : 'poster';
  const row = db.prepare(`SELECT ${type}_local AS f FROM film_metadata WHERE film_id = ?`).get(film.id);
  if (row?.f) { try { unlinkSync(join(IMAGES_DIR, row.f)); } catch {} }
  db.prepare(`UPDATE film_metadata SET ${type}_local = NULL WHERE film_id = ?`).run(film.id);
  res.json({ ok: true });
});

// ---- 一键刷新评分数据（按当前筛选条件批量重抓 TMDB vote_average / vote_count） ----
// 豆瓣评分数据源尚未开发，现阶段仅刷新 TMDB 评分。
app.post('/api/ratings/refresh', async (req, res) => {
  if (!tmdbConfigured()) {
    return res.status(400).json({ error: 'TMDB 未配置' });
  }
  // 允许通过 query 或 body 传入筛选条件
  const f = { ...req.query, ...req.body };
  const where = [];
  const params = [];
  if (f.watchYear) { where.push('v.watch_year = ?'); params.push(Number(f.watchYear)); }
  if (f.releaseYear) { where.push('f.release_year = ?'); params.push(Number(f.releaseYear)); }
  if (f.category === '__no_meta__') {
    where.push('m.tmdb_id IS NULL');
  } else if (f.category) {
    where.push('f.category = ?'); params.push(f.category);
  }
  if (f.platform) {
    where.push("(',' || v.platforms_raw || ',') LIKE ? COLLATE NOCASE");
    params.push(`%,${f.platform},%`);
  }
  if (f.q) { where.push('f.name LIKE ? COLLATE NOCASE'); params.push(`%${f.q}%`); }

  // 同一影视可能有多条观看记录，DISTINCT 去重后按影视刷新
  const rows = db.prepare(`SELECT DISTINCT f.id, f.name, m.tmdb_id AS tmdb, m.media_type AS mt
    FROM films f
    JOIN viewings v ON v.film_id = f.id
    LEFT JOIN film_metadata m ON m.film_id = f.id
    ${where.length ? 'WHERE ' + where.join(' AND ') : ''}`).all(...params);

  const summary = { total: rows.length, updated: 0, skipped: 0, failed: 0, errors: [] };
  const upd = db.prepare('UPDATE film_metadata SET vote_average = ?, vote_count = ?, updated_at = ? WHERE film_id = ?');
  for (const r of rows) {
    if (!r.tmdb || !r.mt) { summary.skipped++; continue; }
    try {
      const details = await getDetails(r.tmdb, r.mt);
      if (!details) { summary.skipped++; continue; }
      const voteAverage = details.vote_average ?? null;
      const voteCount = details.vote_count ?? null;
      upd.run(voteAverage, voteCount, new Date().toISOString(), r.id);
      summary.updated++;
    } catch (e) {
      summary.failed++;
      if (summary.errors.length < 10) summary.errors.push({ id: r.id, name: r.name, error: e.message });
    }
  }
  res.json(summary);
});

// ---- 生产环境托管前端 ----
const clientDist = join(__dirname, '..', 'client', 'dist');
if (existsSync(clientDist)) {
  app.use(express.static(clientDist));
  app.get(/^\/(?!api|images).*/, (_req, res) => res.sendFile(join(clientDist, 'index.html')));
}

app.listen(PORT, () => {
  console.log(`film-memo 后端: http://localhost:${PORT}`);
  console.log(`  本地图片目录: ${IMAGES_DIR}`);
  console.log(`  TMDB 凭证: ${tmdbConfigured() ? '已配置' : '未配置（手动填充不可用）'}`);
});
