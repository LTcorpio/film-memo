import { useState } from 'react';
import { searchMeta, saveMeta } from '../api.js';
import Icon from './Icon.jsx';

export default function MetaSearch({ film, onClose, onSaved }) {
  const [query, setQuery] = useState(film.name);
  const [category, setCategory] = useState(film.category);
  const [results, setResults] = useState([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [saving, setSaving] = useState(null); // 正在保存的候选 key
  const [configured, setConfigured] = useState(true);

  const run = async () => {
    if (!query.trim()) return;
    setLoading(true);
    setError(null);
    try {
      const data = await searchMeta(query, category);
      setConfigured(data.configured !== false);
      setResults(data.results || []);
    } catch (e) {
      setError(e.message);
      setResults([]);
    } finally {
      setLoading(false);
    }
  };

  const pick = async (r) => {
    const key = `${r.media_type}:${r.tmdb_id}`;
    setSaving(key);
    setError(null);
    try {
      await saveMeta(film.id, r.tmdb_id, r.media_type);
      onSaved();
    } catch (e) {
      setError(e.message);
    } finally {
      setSaving(null);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal meta-search" onClick={(e) => e.stopPropagation()}>
        <button className="modal-close" onClick={onClose} title="关闭"><Icon name="close" size={16} /></button>
        <h3>搜索元数据 — 「{film.name}」</h3>

        {!configured && (
          <div className="error-banner">
            TMDB 未配置。请在项目根目录 .env 设置 TMDB_ACCESS_TOKEN 或 TMDB_API_KEY 后重启后端。
          </div>
        )}

        <div className="search-row">
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && run()}
            placeholder="输入影视名称…"
            autoFocus
          />
          <select value={category} onChange={(e) => setCategory(e.target.value)}>
            <option value="">全部类型</option>
            <option value="电影">电影</option>
            <option value="电视剧">电视剧</option>
            <option value="网剧">网剧</option>
            <option value="综艺">综艺</option>
            <option value="动漫">动漫</option>
            <option value="纪录片">纪录片</option>
          </select>
          <button className="btn-primary" onClick={run} disabled={loading}>
            <Icon name="search" size={14} /> {loading ? '搜索中…' : '搜索'}
          </button>
        </div>

        {error && <div className="error-banner small"><Icon name="alert" size={14} /> {error}</div>}

        <div className="results-list">
          {results.length === 0 && !loading && (
            <div className="results-empty">输入名称并搜索，从结果中选择匹配项以填充元数据。</div>
          )}
          {results.map((r) => {
            const key = `${r.media_type}:${r.tmdb_id}`;
            return (
              <div className="result-item" key={key}>
                <div className="result-poster">
                  {r.posterUrl ? (
                    <img src={r.posterUrl} alt={r.title} loading="lazy" />
                  ) : (
                    <div className="poster-placeholder mini">
                      <span className="ph-cat">{r.media_type === 'tv' ? '剧' : '影'}</span>
                    </div>
                  )}
                </div>
                <div className="result-info">
                  <div className="result-title">
                    {r.title}
                    <span className="result-type">{r.media_type === 'tv' ? '电视剧' : '电影'}</span>
                    {r.release_year && <span className="result-year">{r.release_year}</span>}
                  </div>
                  {r.original_title && r.original_title !== r.title && (
                    <div className="result-orig">{r.original_title}</div>
                  )}
                  <div className="result-overview">{r.overview || '暂无简介'}</div>
                </div>
                <button
                  className="btn-primary small"
                  disabled={saving === key}
                  onClick={() => pick(r)}
                >
                  {saving === key ? '保存中…' : '选择'}
                </button>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
