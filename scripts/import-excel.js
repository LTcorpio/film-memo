/**
 * 读取「影视观看记录.xlsx」并导入 SQLite。
 * 表头（含换行）映射为英文字段；IMDb 规范化；日期转 ISO；制片国家/平台保留原始多值字段。
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

  // 仅首次导入：已存在的 id 跳过，避免覆盖用户在 UI 里修改过的观看记录与已刮削元数据。
  // 如需强制重置，删除 data/films.db 后再跑本脚本。
  const insert = db.prepare(`
    INSERT OR IGNORE INTO films
      (id, watch_year, category, name, imdb_id, production_countries_raw,
       release_year, start_date, end_date, total_episodes, platforms_raw, location, notes)
    VALUES
      (@id, @watch_year, @category, @name, @imdb_id, @production_countries_raw,
       @release_year, @start_date, @end_date, @total_episodes, @platforms_raw, @location, @notes)
  `);

  const tx = db.transaction(() => {
    let inserted = 0;
    let skipped = 0;
    for (let r = 2; r <= ws.rowCount; r++) {
      const row = ws.getRow(r);
      if (!row.cellCount) continue;
      const rec = { id: null };
      // 先取 id（序列可能是公式 ROW()-1）
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
      const info = insert.run(rec);
      if (info.changes > 0) inserted++;
      else skipped++;
    }
    return { inserted, skipped };
  });

  const { inserted, skipped } = tx();
  console.log(`✅ 导入完成：新增 ${inserted} 条，跳过 ${skipped} 条已存在记录。`);

  // 摘要
  const summary = db.prepare(`
    SELECT
      COUNT(*) AS total,
      SUM(CASE WHEN imdb_id IS NULL THEN 1 ELSE 0 END) AS no_imdb,
      SUM(CASE WHEN platforms_raw LIKE '%,%' THEN 1 ELSE 0 END) AS multi_platform
    FROM films
  `).get();
  console.log('📊 统计:', summary);
}

main().catch((err) => {
  console.error('❌ 导入失败:', err);
  process.exit(1);
});
