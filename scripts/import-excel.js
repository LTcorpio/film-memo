/**
 * 读取「影视观看记录.xlsx」并导入 SQLite（数据模型 v2：films + viewings）。
 * 表头（含换行）映射为英文字段；IMDb 规范化；日期转 ISO；制片国家/平台保留原始多值字段。
 * 同名影视仅建一条 films 记录，每行 Excel 生成一条 viewings 观看记录；
 * 已存在的同内容观看记录跳过，避免覆盖用户在 UI 里修改过的数据。
 */
import ExcelJS from 'exceljs';
import { fileURLToPath } from 'node:url';
import { dirname, join, resolve } from 'node:path';
import { homedir } from 'node:os';
import dotenv from 'dotenv';

const __dirname = dirname(fileURLToPath(import.meta.url));
dotenv.config({ path: join(__dirname, '..', '.env') });

import { db } from '../server/db.js';

// 中文表头（去空白/换行后）→ 英文字段
const HEADER_MAP = {
  '序': 'id',
  '观看年份': 'watch_year',
  '类别': 'category',
  '名称': 'name',
  'IMDb': 'imdb_id',
  '制片国家': 'production_countries_raw',
  '上映年份': 'release_year',
  '开始观看日期': 'start_date',
  '结束观看日期': 'end_date',
  '总集/期数': 'total_episodes',
  '观看平台': 'platforms_raw',
  '观看地点': 'location',
  '备注': 'notes',
};

function cleanHeader(value) {
  return String(value ?? '').replace(/[\s\n\r]+/g, '').trim();
}

// exceljs 单元格可能是公式 / 富文本 / 超链接对象，统一解包为原始值
function unwrap(value) {
  if (value && typeof value === 'object') {
    if (value instanceof Date) return value;
    if ('result' in value) return value.result; // 公式取结果
    if (Array.isArray(value.richText)) return value.richText.map((t) => t.text).join('');
    if (typeof value.text === 'string') return value.text; // 超链接
  }
  return value;
}

function toDateISO(value) {
  if (!value) return null;
  if (value instanceof Date) {
    if (Number.isNaN(value.getTime())) return null;
    return value.toISOString().slice(0, 10);
  }
  // 字符串形式日期
  const s = String(value).trim();
  if (!s) return null;
  const d = new Date(s);
  return Number.isNaN(d.getTime()) ? s : d.toISOString().slice(0, 10);
}

function toInt(value) {
  if (value === null || value === undefined || value === '') return null;
  const n = Number(value);
  return Number.isFinite(n) ? Math.trunc(n) : null;
}

function normalizeImdb(value) {
  if (!value) return null;
  const s = String(value).trim();
  if (!s || s === '-' || s.toLowerCase() === 'na' || s.toLowerCase() === 'n/a') return null;
  return s;
}

function splitCountries(value) {
  if (!value) return null;
  const s = String(value).trim();
  if (!s) return null;
  // 按 "/" 分割；元数据缺失时前端/后端再用此字段
  return s;
}

function splitPlatforms(value) {
  if (!value) return null;
  const s = String(value).trim();
  return s || null;
}

async function main() {
  const excelPath = process.env.EXCEL_PATH
    ? resolve(process.env.EXCEL_PATH)
    : join(homedir(), 'Desktop', '影视观看记录.xlsx');

  console.log(`📖 读取 Excel: ${excelPath}`);
  const workbook = new ExcelJS.Workbook();
  await workbook.xlsx.readFile(excelPath);
  const ws = workbook.worksheets[0];

  // 第一行表头
  const headerRow = ws.getRow(1);
  const colMap = {}; // 列索引 → 英文字段
  headerRow.eachCell((cell, col) => {
    const zh = cleanHeader(cell.value);
    if (HEADER_MAP[zh]) colMap[col] = HEADER_MAP[zh];
  });

  const expectedFields = Object.values(HEADER_MAP);
  const missing = expectedFields.filter((f) => !Object.values(colMap).includes(f));
  if (missing.length) {
    console.warn(`⚠️  表头缺少字段: ${missing.join(', ')}`);
  }

  // 影视级字段（与 server/db.js v2 模型一致）
  const FILM_FIELDS = ['category', 'name', 'imdb_id', 'production_countries_raw', 'release_year', 'total_episodes'];

  const findFilmByName = db.prepare('SELECT * FROM films WHERE TRIM(name) = TRIM(?) COLLATE NOCASE');
  const insertFilm = db.prepare(() => {
    const cols = ['name', ...FILM_FIELDS.filter((f) => f !== 'name')];
    return `INSERT INTO films (${cols.join(', ')}) VALUES (${cols.map((c) => `@${c}`).join(', ')})`;
  });
  const backfillFilm = db.prepare(() => {
    // 仅回填 NULL 字段
    const sets = FILM_FIELDS.filter((f) => f !== 'name').map((f) => `${f} = COALESCE(${f}, @${f})`);
    return `UPDATE films SET ${sets.join(', ')} WHERE id = @id`;
  });
  const viewingExists = db.prepare(`
    SELECT 1 FROM viewings WHERE film_id = @film_id
      AND watch_year IS @watch_year AND start_date IS @start_date AND end_date IS @end_date
      AND platforms_raw IS @platforms_raw AND location IS @location AND notes IS @notes
  `);
  const insertViewing = db.prepare(`
    INSERT INTO viewings (film_id, watch_year, start_date, end_date, platforms_raw, location, notes)
    VALUES (@film_id, @watch_year, @start_date, @end_date, @platforms_raw, @location, @notes)
  `);

  const tx = db.transaction(() => {
    let inserted = 0;
    let skipped = 0;
    for (let r = 2; r <= ws.rowCount; r++) {
      const row = ws.getRow(r);
      if (!row.cellCount) continue;
      const rec = { id: null };
      // 先取 id（序列可能是公式 ROW()-1），仅用于识别有效行
      const idVal = toInt(unwrap(row.getCell(1)?.value));
      if (idVal === null) continue; // 跳过空行/合计行
      rec.id = idVal;

      for (const [col, field] of Object.entries(colMap)) {
        if (field === 'id') continue;
        const raw = unwrap(row.getCell(Number(col))?.value);
        switch (field) {
          case 'watch_year': rec[field] = toInt(raw); break;
          case 'release_year': rec[field] = toInt(raw); break;
          case 'total_episodes': rec[field] = toInt(raw); break;
          case 'imdb_id': rec[field] = normalizeImdb(raw); break;
          case 'start_date': rec[field] = toDateISO(raw); break;
          case 'end_date': rec[field] = toDateISO(raw); break;
          case 'production_countries_raw': rec[field] = splitCountries(raw); break;
          case 'platforms_raw': rec[field] = splitPlatforms(raw); break;
          default:
            rec[field] = raw == null ? null : String(raw).trim() || null;
        }
      }

      const name = rec.name;
      if (!name) continue;

      // 同名影视已存在时复用（IMDb 冲突视为不同影视，新建一条）
      const existing = findFilmByName.get(name);
      const imdbConflict = existing && rec.imdb_id && existing.imdb_id && rec.imdb_id !== existing.imdb_id;

      let filmId;
      if (existing && !imdbConflict) {
        filmId = existing.id;
        backfillFilm.run({ id: filmId, ...rec });
      } else {
        const filmRec = { name };
        for (const k of FILM_FIELDS) {
          if (k !== 'name') filmRec[k] = rec[k] ?? null;
        }
        filmId = insertFilm.run(filmRec).lastInsertRowid;
      }

      // 同内容观看记录已存在则跳过（幂等，避免重复导入）
      const viewingRec = {
        film_id: filmId,
        watch_year: rec.watch_year ?? null,
        start_date: rec.start_date ?? null,
        end_date: rec.end_date ?? null,
        platforms_raw: rec.platforms_raw ?? null,
        location: rec.location ?? null,
        notes: rec.notes ?? null,
      };
      if (viewingExists.get(viewingRec)) {
        skipped++;
        continue;
      }
      insertViewing.run(viewingRec);
      inserted++;
    }
    return { inserted, skipped };
  });

  const { inserted, skipped } = tx();
  console.log(`✅ 导入完成：新增 ${inserted} 条观看记录，跳过 ${skipped} 条已存在记录。`);

  // 摘要
  const summary = db.prepare(`
    SELECT
      (SELECT COUNT(*) FROM films) AS films,
      (SELECT COUNT(*) FROM viewings) AS viewings,
      (SELECT COUNT(*) FROM films WHERE imdb_id IS NULL) AS no_imdb,
      (SELECT COUNT(*) FROM viewings WHERE platforms_raw LIKE '%,%') AS multi_platform
  `).get();
  console.log('📊 统计:', summary);
}

main().catch((err) => {
  console.error('❌ 导入失败:', err);
  process.exit(1);
});
