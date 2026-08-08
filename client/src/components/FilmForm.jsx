import { useState } from 'react';

export const CATEGORY_OPTS = ['电影', '电视剧', '网剧', '综艺', '动漫', '纪录片', '短剧'];

/** 观看记录编辑表单（可在新增/编辑复用） */
export default function FilmForm({ value, onChange }) {
  const set = (k, v) => onChange({ ...value, [k]: v });
  return (
    <div className="edit-form">
      <div className="form-row">
        <label>名称
          <input value={value.name || ''} onChange={(e) => set('name', e.target.value)} />
        </label>
        <label>类别
          <select value={value.category || ''} onChange={(e) => set('category', e.target.value)}>
            <option value="">—</option>
            {CATEGORY_OPTS.map((c) => <option key={c} value={c}>{c}</option>)}
          </select>
        </label>
      </div>
      <div className="form-row">
        <label>观看年份
          <input type="number" value={value.watchYear ?? ''} onChange={(e) => set('watchYear', e.target.value ? Number(e.target.value) : null)} />
        </label>
        <label>上映年份
          <input type="number" value={value.releaseYear ?? ''} onChange={(e) => set('releaseYear', e.target.value ? Number(e.target.value) : null)} />
        </label>
        <label>总集/期数
          <input type="number" value={value.totalEpisodes ?? ''} onChange={(e) => set('totalEpisodes', e.target.value ? Number(e.target.value) : null)} />
        </label>
      </div>
      <div className="form-row">
        <label>开始观看日期
          <input type="date" value={value.startDate || ''} onChange={(e) => set('startDate', e.target.value || null)} />
        </label>
        <label>结束观看日期
          <input type="date" value={value.endDate || ''} onChange={(e) => set('endDate', e.target.value || null)} />
        </label>
      </div>
      <label>观看平台（逗号分隔）
        <input value={value.platformsRaw || ''} placeholder="爱奇艺,腾讯视频" onChange={(e) => set('platformsRaw', e.target.value)} />
      </label>
      <label>制片国家（按 / 分割；元数据存在时优先用元数据）
        <input value={value.productionCountriesRaw || ''} onChange={(e) => set('productionCountriesRaw', e.target.value)} />
      </label>
      <label>观看地点
        <input value={value.location || ''} onChange={(e) => set('location', e.target.value)} />
      </label>
      <div className="form-row">
        <label>IMDb 号
          <input value={value.imdbId || ''} onChange={(e) => set('imdbId', e.target.value || null)} />
        </label>
        <label>豆瓣 ID
          <input value={value.doubanId || ''} placeholder="如 1292052" onChange={(e) => set('doubanId', e.target.value)} />
        </label>
      </div>
      <label>备注
        <textarea rows={2} value={value.notes || ''} onChange={(e) => set('notes', e.target.value)} />
      </label>
    </div>
  );
}

/** 默认空表单（用于新增） */
export function emptyFilmForm() {
  return {
    name: '',
    category: '',
    watchYear: null,
    releaseYear: null,
    totalEpisodes: null,
    startDate: null,
    endDate: null,
    platformsRaw: '',
    productionCountriesRaw: '',
    location: '',
    imdbId: '',
    doubanId: '',
    notes: '',
  };
}

/** 前端表单 → 后端 films 字段名（snake_case） */
export function filmFormToPatch(f) {
  return {
    name: f.name,
    category: f.category || null,
    watch_year: f.watchYear ?? null,
    release_year: f.releaseYear ?? null,
    total_episodes: f.totalEpisodes ?? null,
    start_date: f.startDate || null,
    end_date: f.endDate || null,
    platforms_raw: f.platformsRaw || null,
    production_countries_raw: f.productionCountriesRaw || null,
    location: f.location || null,
    imdb_id: f.imdbId || null,
    douban_id: f.doubanId?.trim() || null,
    notes: f.notes || null,
  };
}

/** 影片数据 → 表单数据 */
export function filmToForm(film) {
  return {
    name: film.name,
    category: film.category,
    watchYear: film.watchYear,
    releaseYear: film.releaseYear,
    totalEpisodes: film.totalEpisodes,
    startDate: film.startDate,
    endDate: film.endDate,
    platformsRaw: (film.platforms || []).join(','),
    productionCountriesRaw: film.productionCountriesRaw || '',
    location: film.location,
    imdbId: film.imdbId,
    doubanId: film.doubanId,
    notes: film.notes,
  };
}
