const BASE = '/api';

async function http(url, opts) {
  const res = await fetch(url, opts);
  if (!res.ok) {
    const body = await res.json().catch(() => ({}));
    throw new Error(body.error || `HTTP ${res.status}`);
  }
  return res.json();
}

export const fetchFilms = (params) => {
  const qs = new URLSearchParams();
  Object.entries(params || {}).forEach(([k, v]) => {
    if (v !== '' && v != null) qs.set(k, v);
  });
  return http(`${BASE}/films?${qs}`);
};

export const fetchFilm = (id) => http(`${BASE}/films/${id}`);
export const fetchFilters = () => http(`${BASE}/filters`);
export const fetchStats = () => http(`${BASE}/stats`);

/** 新增观影记录 */
export const createFilm = (data) =>
  http(`${BASE}/films`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(data),
  });

export const searchMeta = (q, category) =>
  http(`${BASE}/meta/search?q=${encodeURIComponent(q)}&category=${encodeURIComponent(category || '')}`);

/** 选择 TMDB 候选写入元数据（同时下载图片到本地） */
export const saveMeta = (filmId, tmdbId, mediaType) =>
  http(`${BASE}/films/${filmId}/metadata`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ tmdbId, mediaType }),
  });

/** 删除元数据（同时删除本地图片） */
export const deleteMeta = (filmId) =>
  http(`${BASE}/films/${filmId}/metadata`, { method: 'DELETE' });

/** 编辑观看记录（films 表字段，传需要更新的字段即可） */
export const updateFilm = (filmId, patch) =>
  http(`${BASE}/films/${filmId}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  });

/** 编辑元数据（title/overview/genres/runtime/vote_average 等） */
export const updateMeta = (filmId, patch) =>
  http(`${BASE}/films/${filmId}/metadata`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  });

/** 上传本地图片替换海报/背景图（type: 'poster' | 'backdrop'）
 *  后端用 express.raw({type:'image/*'}) 接收原始字节，故直接发送 file Blob。 */
export const uploadImage = (filmId, type, file) =>
  http(`${BASE}/films/${filmId}/image?type=${type}`, {
    method: 'POST',
    headers: { 'Content-Type': file.type || 'image/jpeg' },
    body: file,
  });

/** 从 TMDB 远程下载图片到本地（type: 'poster' | 'backdrop'） */
export const scrapeImage = (filmId, type) =>
  http(`${BASE}/films/${filmId}/scrape-image?type=${type}`, { method: 'POST' });

/** 删除本地图片，回退到远程 TMDB 图 */
export const deleteImage = (filmId, type) =>
  http(`${BASE}/films/${filmId}/image?type=${type}`, { method: 'DELETE' });
