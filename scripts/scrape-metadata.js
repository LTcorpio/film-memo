/**
 * 按 IMDb 号批量刮削 TMDB 元数据，写入 film_metadata。
 * 用法:
 *   npm run scrape            # 只处理尚无元数据、且已有 imdb_id 的影片
 *   npm run scrape -- --force # 强制重新刮削（覆盖已有元数据）
 *   npm run scrape -- --id 6  # 仅处理指定 film id
 * 需在 .env 配置 TMDB_ACCESS_TOKEN 或 TMDB_API_KEY。
 */
import { db } from '../server/db.js';
import { tmdbConfigured, findByImdb, getDetails, normalizeDetails } from '../server/tmdb.js';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join, extname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const IMAGES_DIR = join(__dirname, '..', 'data', 'images');
mkdirSync(IMAGES_DIR, { recursive: true });

const args = process.argv.slice(2);
const force = args.includes('--force');
const idArgIdx = args.indexOf('--id');
const onlyId = idArgIdx >= 0 ? Number(args[idArgIdx + 1]) : null;

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function downloadImage(path, filename) {
  if (!path) return null;
  const url = `https://image.tmdb.org/t/p/original${path}`;
  try {
    const r = await fetch(url);
    if (!r.ok) throw new Error(`img ${r.status}`);
    const buf = Buffer.from(await r.arrayBuffer());
    const file = filename + (extname(path) || '.jpg');
    writeFileSync(join(IMAGES_DIR, file), buf);
    return file;
  } catch (e) {
    console.warn(`  ⚠️  图片下载失败 ${url}: ${e.message}`);
    return null;
  }
}

async function scrapeOne(film) {
  const found = await findByImdb(film.imdb_id, film.category);
  if (!found) {
    console.log(`  ⚠️  未匹配: id=${film.id} «${film.name}» (${film.imdb_id})`);
    return { ok: false, reason: 'not_found' };
  }
  const details = await getDetails(found.tmdb_id, found.media_type);
  const meta = normalizeDetails(details, found.media_type);
  if (!meta) {
    console.log(`  ⚠️  详情为空: id=${film.id} «${film.name}»`);
    return { ok: false, reason: 'no_details' };
  }
  // 下载海报/背景图到本地
  const posterLocal = await downloadImage(meta.poster_path, `${film.id}-poster`);
  const backdropLocal = await downloadImage(meta.backdrop_path, `${film.id}-backdrop`);
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
    VALUES
      (@film_id, @imdb_id, @tmdb_id, @media_type, @title, @original_title, @overview,
       @poster_path, @backdrop_path, @poster_local, @backdrop_local,
       @genres, @production_countries, @runtime, @vote_average, @vote_count,
       @directors, @cast, @release_date, @status, @tagline,
       @original_language, @spoken_languages, @origin_country, @production_companies,
       @writers, @cinematographers, @composers, @producers, @keywords,
       @number_of_seasons, @number_of_episodes, @budget, @revenue,
       @content_rating, @homepage, @updated_at)
  `).run({ film_id: film.id, ...meta, poster_local: posterLocal, backdrop_local: backdropLocal });
  console.log(`  ✅ id=${film.id} «${film.name}» → ${meta.media_type}/${meta.tmdb_id} «${meta.title}»`);
  return { ok: true };
}

async function main() {
  if (!tmdbConfigured()) {
    console.error('❌ 未配置 TMDB 凭证。请在 .env 中设置 TMDB_ACCESS_TOKEN 或 TMDB_API_KEY。');
    process.exit(1);
  }

  const where = ['imdb_id IS NOT NULL'];
  const params = [];
  if (!force) {
    where.push('id NOT IN (SELECT film_id FROM film_metadata)');
  }
  if (onlyId) {
    where.push('id = ?');
    params.push(onlyId);
  }
  const films = db.prepare(`SELECT id, name, imdb_id, category FROM films WHERE ${where.join(' AND ')} ORDER BY id`).all(...params);

  console.log(`待刮削: ${films.length} 部${force ? ' (强制覆盖)' : ''}`);
  let ok = 0, fail = 0;
  // imdb_id -> meta（不含本地图片），复用以避免重复请求 TMDB
  const cache = new Map();
  for (const film of films) {
    try {
      let result;
      if (!force && cache.has(film.imdb_id)) {
        const meta = cache.get(film.imdb_id);
        if (meta) {
          // 复用元数据，但仍需为本 film.id 下载独立图片
          const posterLocal = await downloadImage(meta.poster_path, `${film.id}-poster`);
          const backdropLocal = await downloadImage(meta.backdrop_path, `${film.id}-backdrop`);
          db.prepare(`INSERT OR REPLACE INTO film_metadata
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
             @content_rating, @homepage, @updated_at)`)
            .run({ film_id: film.id, ...meta, poster_local: posterLocal, backdrop_local: backdropLocal });
          console.log(`  ♻️  id=${film.id} «${film.name}» (复用缓存)`);
          result = { ok: true };
        } else {
          result = { ok: false, reason: 'cached_not_found' };
        }
      } else {
        result = await scrapeOne(film);
        // 缓存不含本地图片字段，便于其它 film.id 复用时重新下载
        if (result.ok) {
          const row = db.prepare(`SELECT imdb_id, tmdb_id, media_type, title, original_title, overview,
            poster_path, backdrop_path, genres, production_countries, runtime, vote_average, vote_count,
            directors, "cast" AS cast, release_date, status, tagline,
            original_language, spoken_languages, origin_country, production_companies,
            writers, cinematographers, composers, producers, keywords,
            number_of_seasons, number_of_episodes, budget, revenue,
            content_rating, homepage, updated_at
            FROM film_metadata WHERE film_id = ?`).get(film.id);
          cache.set(film.imdb_id, row);
        } else {
          cache.set(film.imdb_id, null);
        }
      }
      result.ok ? ok++ : fail++;
    } catch (e) {
      console.error(`  ❌ id=${film.id} «${film.name}»: ${e.message}`);
      fail++;
    }
    await sleep(250); // TMDB 速率限制 ~40 req/10s
  }
  console.log(`\n完成: 成功 ${ok}，失败 ${fail}`);
}

main().catch((e) => { console.error(e); process.exit(1); });
