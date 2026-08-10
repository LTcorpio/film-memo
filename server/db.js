import Database from 'better-sqlite3';
import { mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import dotenv from 'dotenv';

const __dirname = dirname(fileURLToPath(import.meta.url));
dotenv.config({ path: join(__dirname, '..', '.env') });
const DB_PATH = process.env.DB_PATH || join(__dirname, '..', 'data', 'films.db');

// 确保数据目录存在
mkdirSync(dirname(DB_PATH), { recursive: true });

export const db = new Database(DB_PATH);
db.pragma('journal_mode = WAL');

const SCHEMA = `
CREATE TABLE IF NOT EXISTS films (
  id                  INTEGER PRIMARY KEY,        -- 序
  watch_year          INTEGER,                    -- 观看年份
  category            TEXT,                       -- 类别
  name                TEXT NOT NULL,              -- 名称
  imdb_id             TEXT,                       -- IMDb 号（已规范化，缺失为 NULL）
  douban_id           TEXT,                       -- 豆瓣条目 ID（用户手动填写）
  production_countries_raw TEXT,                  -- 制片国家原始字段（按 "/" 分割）
  release_year        INTEGER,                    -- 上映年份
  start_date          TEXT,                       -- 开始观看日期 (ISO YYYY-MM-DD)
  end_date            TEXT,                       -- 结束观看日期
  total_episodes      INTEGER,                    -- 总集/期数
  platforms_raw       TEXT,                       -- 观看平台原始字段（按 "," 分割）
  location            TEXT,                       -- 观看地点
  notes               TEXT                        -- 备注
);

CREATE TABLE IF NOT EXISTS film_metadata (
  film_id             INTEGER PRIMARY KEY,        -- 关联 films.id
  imdb_id             TEXT,                       -- TMDB 返回的 IMDb 号（可能为空）
  tmdb_id             INTEGER,
  media_type          TEXT,                       -- movie / tv
  title               TEXT,
  original_title      TEXT,
  overview            TEXT,
  poster_path         TEXT,
  backdrop_path       TEXT,
  poster_local        TEXT,                       -- 本地图片文件名（用户上传/刮削下载）
  backdrop_local      TEXT,                       -- 本地图片文件名
  genres              TEXT,                       -- JSON 数组
  production_countries TEXT,                      -- JSON 数组（优先使用）
  runtime             INTEGER,
  vote_average        REAL,
  vote_count          INTEGER,
  directors           TEXT,                       -- JSON 数组（导演）
  cast                TEXT,                       -- JSON 数组（主要演员）
  release_date        TEXT,                        -- 上映/首播日期 (YYYY-MM-DD)
  status              TEXT,                       -- 状态（如 Released / Ended / Returning Series）
  tagline             TEXT,                       -- 宣传语
  -- 扩展元数据
  original_language   TEXT,                       -- 原始语言代码（如 en/zh/ja）
  spoken_languages    TEXT,                       -- JSON 数组（对白语言）
  origin_country      TEXT,                       -- JSON 数组（出品国家代码）
  production_companies TEXT,                      -- JSON 数组（制片公司）
  writers             TEXT,                       -- JSON 数组（编剧）
  cinematographers    TEXT,                       -- JSON 数组（摄影指导）
  composers           TEXT,                       -- JSON 数组（配乐）
  producers           TEXT,                       -- JSON 数组（制片人）
  keywords            TEXT,                       -- JSON 数组（关键词）
  number_of_seasons   INTEGER,                    -- 季数（TV）
  number_of_episodes  INTEGER,                    -- 集数（TV）
  budget              INTEGER,                    -- 预算（电影）
  revenue             INTEGER,                    -- 票房（电影）
  content_rating      TEXT,                       -- 内容分级（如 PG-13 / TV-MA）
  homepage            TEXT,                       -- 官方主页
  updated_at          TEXT
);
CREATE INDEX IF NOT EXISTS idx_meta_imdb ON film_metadata(imdb_id);

CREATE INDEX IF NOT EXISTS idx_films_watch_year ON films(watch_year);
CREATE INDEX IF NOT EXISTS idx_films_release_year ON films(release_year);
CREATE INDEX IF NOT EXISTS idx_films_category ON films(category);
`;

db.exec(SCHEMA);

// 兼容旧库：补齐 film_metadata 新增列
const cols = db.prepare("PRAGMA table_info(film_metadata)").all();
const names = new Set(cols.map((c) => c.name));
for (const col of [
  'poster_local', 'backdrop_local', 'directors', 'cast', 'release_date', 'status', 'tagline',
  'original_language', 'spoken_languages', 'origin_country', 'production_companies',
  'writers', 'cinematographers', 'composers', 'producers', 'keywords',
  'number_of_seasons', 'number_of_episodes', 'budget', 'revenue',
  'content_rating', 'homepage',
]) {
  if (!names.has(col)) {
    const type = (col === 'number_of_seasons' || col === 'number_of_episodes' ||
      col === 'budget' || col === 'revenue') ? 'INTEGER' : 'TEXT';
    db.exec(`ALTER TABLE film_metadata ADD COLUMN ${col} ${type};`);
  }
}

// 兼容旧库：补齐 films.douban_id 列
const filmCols = db.prepare("PRAGMA table_info(films)").all();
const filmColNames = new Set(filmCols.map((c) => c.name));
if (!filmColNames.has('douban_id')) {
  db.exec('ALTER TABLE films ADD COLUMN douban_id TEXT;');
}
// 若旧库 film_metadata 仍存在 douban_id 列：先把数据回填到 films，再删除该列（幂等）
const metaHasDouban = db.prepare("SELECT 1 FROM pragma_table_info('film_metadata') WHERE name = ?").get('douban_id');
if (metaHasDouban) {
  // 仅回填 films.douban_id 为 NULL 的行
  db.exec(`
    UPDATE films SET douban_id = (
      SELECT m.douban_id FROM film_metadata m WHERE m.film_id = films.id
    ) WHERE douban_id IS NULL;
  `);
  // 物理删除 film_metadata.douban_id 列（SQLite 3.35.0+）
  db.exec('ALTER TABLE film_metadata DROP COLUMN douban_id;');
}

export default db;
