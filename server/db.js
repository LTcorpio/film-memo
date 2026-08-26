import Database from 'better-sqlite3';
import { mkdirSync, unlinkSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import dotenv from 'dotenv';

const __dirname = dirname(fileURLToPath(import.meta.url));
dotenv.config({ path: join(__dirname, '..', '.env') });
const DB_PATH = process.env.DB_PATH || join(__dirname, '..', 'data', 'films.db');
const IMAGES_DIR = process.env.IMAGES_DIR || join(__dirname, '..', 'data', 'images');

// 确保数据目录存在
mkdirSync(dirname(DB_PATH), { recursive: true });

export const db = new Database(DB_PATH);
db.pragma('journal_mode = WAL');
db.pragma('foreign_keys = ON');

// 数据模型（v2）：
//   films      —— 影视级数据，同一影视仅存一份（名称/类别/上映年份/总集数/IMDb/豆瓣/制片国家）
//   viewings   —— 观看记录，同一影视可有多条（观看年份/起止日期/平台/地点/备注）
//   film_metadata —— TMDB 元数据，挂在 films.id 上，同一影视共享一份
const SCHEMA = `
CREATE TABLE IF NOT EXISTS films (
  id                  INTEGER PRIMARY KEY,        -- 序
  category            TEXT,                       -- 类别
  name                TEXT NOT NULL,              -- 名称
  imdb_id             TEXT,                       -- IMDb 号（已规范化，缺失为 NULL）
  douban_id           TEXT,                       -- 豆瓣条目 ID（用户手动填写）
  production_countries_raw TEXT,                  -- 制片国家原始字段（按 "/" 分割）
  release_year        INTEGER,                    -- 上映年份
  total_episodes      INTEGER                     -- 总集/期数
);

CREATE TABLE IF NOT EXISTS viewings (
  id                  INTEGER PRIMARY KEY,        -- 观看记录 id
  film_id             INTEGER NOT NULL,           -- 关联 films.id
  watch_year          INTEGER,                    -- 观看年份
  start_date          TEXT,                       -- 开始观看日期 (ISO YYYY-MM-DD)
  end_date            TEXT,                       -- 结束观看日期
  platforms_raw       TEXT,                       -- 观看平台原始字段（按 "," 分割）
  location            TEXT,                       -- 观看地点
  notes               TEXT,                       -- 备注
  FOREIGN KEY (film_id) REFERENCES films(id) ON DELETE CASCADE
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

CREATE INDEX IF NOT EXISTS idx_viewings_film ON viewings(film_id);
CREATE INDEX IF NOT EXISTS idx_viewings_watch_year ON viewings(watch_year);
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

// ---- 旧模型 → 新模型迁移（幂等） ----
// 旧 films 表内嵌观看字段（watch_year/start_date/end_date/platforms_raw/location/notes），
// 检测到旧列时执行：同一影视去重合并（按 tmdb_id / imdb_id / 名称匹配），
// 观看字段拆分为 viewings 记录，影视级字段与元数据只保留一份。
migrateToViewings();

function migrateToViewings() {
  const currentCols = db.prepare("PRAGMA table_info(films)").all().map((c) => c.name);
  const WATCH_COLS = ['watch_year', 'start_date', 'end_date', 'platforms_raw', 'location', 'notes'];
  if (!WATCH_COLS.every((c) => currentCols.includes(c))) return; // 新库或已迁移

  const migrate = db.transaction(() => {
    // 读取旧数据（含元数据标识，便于选出规范行）
    const rows = db.prepare(`
      SELECT f.*, m.tmdb_id AS m_tmdb_id, m.imdb_id AS m_imdb_id
      FROM films f LEFT JOIN film_metadata m ON m.film_id = f.id
    `).all();
    // 已刮削元数据的行优先作为规范行，其次按 id 升序
    rows.sort((a, b) => (b.m_tmdb_id ? 1 : 0) - (a.m_tmdb_id ? 1 : 0) || a.id - b.id);

    // ---- 去重分组：tmdb_id > imdb_id > 名称（同名但 tmdb/imdb 冲突视为不同影视） ----
    const byTmdb = new Map();
    const byImdb = new Map();
    const byName = new Map();
    const groups = [];

    for (const row of rows) {
      const tmdbKey = row.m_tmdb_id ? `t:${row.m_tmdb_id}` : null;
      const imdbVal = String(row.imdb_id || row.m_imdb_id || '').trim();
      const imdbKey = imdbVal ? `i:${imdbVal.toLowerCase()}` : null;
      const nameKey = `n:${String(row.name || '').trim().toLowerCase()}`;

      let g = (tmdbKey && byTmdb.get(tmdbKey)) || (imdbKey && byImdb.get(imdbKey)) || null;
      if (!g) {
        const cand = byName.get(nameKey);
        const conflict = cand && (
          (tmdbKey && cand.tmdbKey && tmdbKey !== cand.tmdbKey) ||
          (imdbKey && cand.imdbKey && imdbKey !== cand.imdbKey)
        );
        if (!conflict) g = cand;
      }
      if (!g) {
        g = { canonical: row, members: [], tmdbKey, imdbKey };
        groups.push(g);
      }
      g.members.push(row);
      if (tmdbKey && !g.tmdbKey) g.tmdbKey = tmdbKey;
      if (imdbKey && !g.imdbKey) g.imdbKey = imdbKey;
      if (tmdbKey) byTmdb.set(tmdbKey, g);
      if (imdbKey) byImdb.set(imdbKey, g);
      byName.set(nameKey, g);
    }

    const insViewing = db.prepare(`
      INSERT INTO viewings (film_id, watch_year, start_date, end_date, platforms_raw, location, notes)
      VALUES (@film_id, @watch_year, @start_date, @end_date, @platforms_raw, @location, @notes)
    `);
    const updFilm = db.prepare(`
      UPDATE films SET category = @category, imdb_id = @imdb_id, douban_id = @douban_id,
        production_countries_raw = @production_countries_raw, release_year = @release_year,
        total_episodes = @total_episodes
      WHERE id = @id
    `);
    const delFilm = db.prepare('DELETE FROM films WHERE id = ?');
    const delMeta = db.prepare('DELETE FROM film_metadata WHERE film_id = ?');
    const reattachMeta = db.prepare('UPDATE film_metadata SET film_id = ? WHERE film_id = ?');
    const hasMeta = db.prepare('SELECT 1 FROM film_metadata WHERE film_id = ?');

    let mergedFilms = 0;
    for (const g of groups) {
      const c = g.canonical;

      // 影视级字段：规范行缺失时用成员值回填
      const merged = {
        category: c.category,
        imdb_id: c.imdb_id,
        douban_id: c.douban_id,
        production_countries_raw: c.production_countries_raw,
        release_year: c.release_year,
        total_episodes: c.total_episodes,
      };
      for (const m of g.members) {
        for (const k of Object.keys(merged)) {
          if (merged[k] == null && m[k] != null) merged[k] = m[k];
        }
      }

      // 元数据：规范行无而成员有 → 迁移给规范行；都有 → 删除成员的（连同本地图片）
      let canonicalHasMeta = Boolean(hasMeta.get(c.id));
      for (const m of g.members) {
        if (m.id === c.id) continue;
        if (!hasMeta.get(m.id)) continue;
        if (!canonicalHasMeta) {
          reattachMeta.run(c.id, m.id);
          canonicalHasMeta = true;
        } else {
          const meta = db.prepare('SELECT poster_local, backdrop_local FROM film_metadata WHERE film_id = ?').get(m.id);
          for (const f of [meta?.poster_local, meta?.backdrop_local]) {
            if (f) { try { unlinkSync(join(IMAGES_DIR, f)); } catch {} }
          }
          delMeta.run(m.id);
        }
      }

      // 每条旧记录生成一条观看记录（指向规范行）
      for (const m of g.members) {
        insViewing.run({
          film_id: c.id,
          watch_year: m.watch_year,
          start_date: m.start_date,
          end_date: m.end_date,
          platforms_raw: m.platforms_raw,
          location: m.location,
          notes: m.notes,
        });
      }

      updFilm.run({ id: c.id, ...merged });
      for (const m of g.members) {
        if (m.id !== c.id) { delFilm.run(m.id); mergedFilms++; }
      }
    }

    // 旧列上有索引时先删索引，再物理删除已迁移的观看字段列
    db.exec('DROP INDEX IF EXISTS idx_films_watch_year;');
    for (const col of WATCH_COLS) {
      db.exec(`ALTER TABLE films DROP COLUMN ${col};`);
    }
    return { films: groups.length, merged: mergedFilms };
  });

  const r = migrate();
  console.log(`[db] 数据模型迁移完成：${r.films} 部影视（合并去重 ${r.merged} 条重复记录）`);
}

export default db;
