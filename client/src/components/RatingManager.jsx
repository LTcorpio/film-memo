import { useState } from 'react';
import { updateFilm, refreshRatings } from '../api.js';
import Icon from './Icon.jsx';

/** 单行：剧集名称 + IMDb 输入 + 豆瓣 ID 输入 + 评分数据源徽标 + 保存按钮 */
function RatingRow({ film, onSaved }) {
  const [imdbId, setImdbId] = useState(film.imdbId || '');
  const [doubanId, setDoubanId] = useState(film.doubanId || '');
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [err, setErr] = useState(null);

  // 归一化：trim 后空值视为 ''，与 save 的持久化逻辑（trim || null）一致
  const norm = (v) => (v && v.trim()) || '';
  const persistedImdbId = norm(film.imdbId);
  const persistedDoubanId = norm(film.doubanId);
  const dirty = norm(imdbId) !== persistedImdbId || norm(doubanId) !== persistedDoubanId;
  const source = persistedDoubanId ? 'douban' : 'tmdb';

  const save = async () => {
    setSaving(true);
    setErr(null);
    setSaved(false);
    try {
      const payload = {
        imdb_id: imdbId.trim() || null,
        douban_id: doubanId.trim() || null,
      };
      await updateFilm(film.filmId, payload);
      setSaved(true);
      onSaved(film.filmId, payload);
    } catch (e) {
      setErr(e.message);
    } finally {
      setSaving(false);
    }
  };

  // 编辑任一字段：清除已保存状态，按钮恢复为「保存」并按 dirty 启用
  const editImdb = (v) => { setImdbId(v); setSaved(false); };
  const editDouban = (v) => { setDoubanId(v); setSaved(false); };

  const onKey = (e) => {
    if (e.key === 'Enter' && dirty && !saving) save();
  };

  return (
    <div className="rating-row">
      <div className="rating-row-info">
        <div className="rating-row-title">{film.name}</div>
        <div className="rating-row-meta">
          <span className="rating-row-cat">{film.category || '—'}</span>
          <span className={`rating-source-badge ${source}`}>
            {source === 'douban' ? '豆瓣' : 'TMDB'}
          </span>
        </div>
      </div>
      <div className="rating-row-inputs">
        <label className="rating-field">
          <span className="rating-field-label">IMDb</span>
          <input
            type="text"
            value={imdbId}
            onChange={(e) => editImdb(e.target.value)}
            onKeyDown={onKey}
            placeholder=""
            disabled={saving}
          />
        </label>
        <label className="rating-field">
          <span className="rating-field-label">豆瓣</span>
          <input
            type="text"
            value={doubanId}
            onChange={(e) => editDouban(e.target.value)}
            onKeyDown={onKey}
            placeholder=""
            disabled={saving}
          />
        </label>
      </div>
      <button
        type="button"
        className="btn-primary small"
        disabled={saved || !dirty || saving}
        onClick={save}
      >
        {saving ? '保存中…' : saved ? <Icon name="check" size={14} /> : '保存'}
      </button>
      {err && <div className="rating-row-err">{err}</div>}
    </div>
  );
}

export default function RatingManager({ films, filters, onClose, onChanged }) {
  const [refreshing, setRefreshing] = useState(false);
  const [refreshErr, setRefreshErr] = useState(null);
  const [refreshResult, setRefreshResult] = useState(null);
  // 保存后的覆盖层：filmId -> { imdbId, doubanId }（用于即时刷新徽标，无需重拉列表）
  const [overrides, setOverrides] = useState({});

  const handleRowSaved = (filmId, payload) => {
    setOverrides((o) => ({ ...o, [filmId]: payload }));
  };

  const doRefresh = async () => {
    setRefreshing(true);
    setRefreshErr(null);
    setRefreshResult(null);
    try {
      const summary = await refreshRatings(filters);
      setRefreshResult(summary);
      onChanged();
    } catch (e) {
      setRefreshErr(e.message);
    } finally {
      setRefreshing(false);
    }
  };

  // 将覆盖层应用到 films，使徽标即时反映刚保存的豆瓣 ID；
  // 同一影视可能有多条观看记录，按 filmId 去重只保留一行
  const seen = new Set();
  const rows = films
    .filter((f) => {
      if (seen.has(f.filmId)) return false;
      seen.add(f.filmId);
      return true;
    })
    .map((f) => {
      if (!(f.filmId in overrides)) return f;
      const { imdbId, doubanId } = overrides[f.filmId];
      return { ...f, imdbId: imdbId || null, doubanId: doubanId || null };
    });

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal rating-manager" onClick={(e) => e.stopPropagation()}>
        <button className="modal-close" onClick={onClose} title="关闭"><Icon name="close" size={16} /></button>
        <h3><Icon name="star" size={16} /> 评分管理</h3>
        <div className="rating-manager-sub">
          共 {films.length} 条记录 · 统一维护 IMDb 号与豆瓣 ID（豆瓣评分数据源暂未开发，现阶段仅可填写并保存豆瓣 ID）
        </div>

        <div className="rating-toolbar">
          <button
            type="button"
            className="btn-primary"
            disabled={refreshing || films.length === 0}
            onClick={doRefresh}
            title="按当前筛选结果批量重抓 TMDB 评分"
          >
            <Icon name="refresh" size={14} /> {refreshing ? '更新中…' : '一键更新评分数据'}
          </button>
          {refreshResult && (
            <span className="rating-result">
              已更新 {refreshResult.updated} · 跳过 {refreshResult.skipped} · 失败 {refreshResult.failed}
              （共 {refreshResult.total}）
            </span>
          )}
          {refreshErr && <span className="rating-result err"><Icon name="alert" size={12} /> {refreshErr}</span>}
        </div>

        <div className="rating-list">
          {rows.length === 0 ? (
            <div className="results-empty">无匹配记录，请先调整筛选条件。</div>
          ) : (
            rows.map((f) => (
              <RatingRow key={f.filmId} film={f} onSaved={handleRowSaved} />
            ))
          )}
        </div>
      </div>
    </div>
  );
}
