import { useState } from 'react';
import { updateFilm, refreshRatings } from '../api.js';
import Icon from './Icon.jsx';

/** 单行：剧集名称 + IMDb + 豆瓣 ID 输入 + 评分数据源徽标 + 保存按钮 */
function RatingRow({ film, onSaved }) {
  const [doubanId, setDoubanId] = useState(film.doubanId || '');
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [err, setErr] = useState(null);

  const persistedDoubanId = film.doubanId || '';
  const dirty = (doubanId || '') !== (persistedDoubanId || '');
  const source = persistedDoubanId ? 'douban' : 'tmdb';

  const save = async () => {
    setSaving(true);
    setErr(null);
    setSaved(false);
    try {
      await updateFilm(film.id, { douban_id: doubanId.trim() || null });
      setSaved(true);
      setTimeout(() => setSaved(false), 1500);
      onSaved(film.id, doubanId.trim() || null);
    } catch (e) {
      setErr(e.message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="rating-row">
      <div className="rating-row-info">
        <div className="rating-row-title">
          {film.name}
          {film.imdbId && <span className="rating-row-imdb">IMDb: {film.imdbId}</span>}
        </div>
        <div className="rating-row-meta">
          <span className="rating-row-cat">{film.category || '—'}</span>
          <span className={`rating-source-badge ${source}`}>
            {source === 'douban' ? '豆瓣' : 'TMDB'}
          </span>
        </div>
      </div>
      <div className="rating-row-input">
        <input
          type="text"
          value={doubanId}
          onChange={(e) => setDoubanId(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && dirty && !saving && save()}
          placeholder="豆瓣条目 ID"
          disabled={saving}
        />
      </div>
      <button
        type="button"
        className="btn-primary small"
        disabled={!dirty || saving}
        onClick={save}
      >
        {saving ? '保存中…' : saved ? '已保存' : '保存'}
      </button>
      {err && <div className="rating-row-err">{err}</div>}
    </div>
  );
}

export default function RatingManager({ films, filters, onClose, onChanged }) {
  const [refreshing, setRefreshing] = useState(false);
  const [refreshErr, setRefreshErr] = useState(null);
  const [refreshResult, setRefreshResult] = useState(null);
  // 保存后的豆瓣 ID 覆盖层：filmId -> doubanId（用于即时刷新徽标，无需重拉列表）
  const [overrides, setOverrides] = useState({});

  const handleRowSaved = (filmId, doubanId) => {
    setOverrides((o) => ({ ...o, [filmId]: doubanId }));
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

  // 将覆盖层应用到 films，使徽标即时反映刚保存的豆瓣 ID
  const rows = films.map((f) => {
    if (!(f.id in overrides)) return f;
    const doubanId = overrides[f.id];
    return { ...f, doubanId: doubanId || null };
  });

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal rating-manager" onClick={(e) => e.stopPropagation()}>
        <button className="modal-close" onClick={onClose} title="关闭"><Icon name="close" size={16} /></button>
        <h3><Icon name="star" size={16} /> 评分管理</h3>
        <div className="rating-manager-sub">
          共 {films.length} 条记录 · 维护豆瓣 ID 与评分数据源（豆瓣评分数据源暂未开发，现阶段仅可填写并保存豆瓣 ID）
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
              <RatingRow key={f.id} film={f} onSaved={handleRowSaved} />
            ))
          )}
        </div>
      </div>
    </div>
  );
}
