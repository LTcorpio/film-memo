import { useState, useRef, useEffect } from 'react';
import {
  saveMeta, deleteMeta, updateFilm, updateMeta,
  uploadImage, scrapeImage, deleteImage, deleteFilm,
} from '../api.js';
import Icon from './Icon.jsx';
import PlatformTag from './PlatformTag.jsx';
import MetaSearch from './MetaSearch.jsx';
import FilmForm, { filmToForm, filmFormToPatch } from './FilmForm.jsx';

/** 影视简介：长文自动截断，点击展开/收起 */
function Overview({ text }) {
  const [expanded, setExpanded] = useState(false);
  const [clamped, setClamped] = useState(false);
  const ref = useRef(null);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.maxHeight = 'none';
    const fullHeight = el.scrollHeight;
    el.style.maxHeight = '';
    // 收起态高度由 CSS max-height 控制，3 行约 1.7em * 3
    const lineHeight = parseFloat(getComputedStyle(el).lineHeight) || 22;
    const visibleHeight = lineHeight * 3 + 4;
    setClamped(fullHeight > visibleHeight + 2);
  }, [text]);

  return (
    <div className="overview-wrap">
      {clamped && !expanded && (
        <button
          type="button"
          className="overview-toggle inline-end"
          onClick={() => setExpanded(true)}
        >
          展开 <Icon name="chevron" size={12} />
        </button>
      )}
      <p ref={ref} className={`overview${expanded ? ' expanded' : ''}`}>{text}</p>
      {clamped && expanded && (
        <button
          type="button"
          className="overview-toggle floated"
          onClick={() => setExpanded(false)}
        >
          收起 <Icon name="chevron-up" size={12} />
        </button>
      )}
    </div>
  );
}

/** 影片元数据展示：默认显示部分项，其余可展开 */
function MetaInfo({ meta, film }) {
  const [expanded, setExpanded] = useState(false);

  // 默认显示的字段：导演、编剧、主演、制片国家、上映日期
  const primary = [];
  if (meta?.directors?.length > 0) {
    primary.push(<div key="directors"><dt>导演</dt><dd>{meta.directors.join(' / ')}</dd></div>);
  }
  if (meta?.writers?.length > 0) {
    primary.push(<div key="writers"><dt>编剧</dt><dd>{meta.writers.join(' / ')}</dd></div>);
  }
  if (meta?.cast?.length > 0) {
    primary.push(<div key="cast"><dt>主演</dt><dd>{meta.cast.join(' / ')}</dd></div>);
  }
  const countries = meta?.productionCountries?.length
    ? meta.productionCountries
    : film.productionCountries;
  if (countries.length > 0) {
    primary.push(<div key="countries"><dt>制片国家</dt><dd>{countries.join(' / ')}</dd></div>);
  }
  if (meta?.releaseDate) {
    primary.push(<div key="releaseDate"><dt>上映日期</dt><dd>{meta.releaseDate}</dd></div>);
  }

  // 可折叠的额外字段
  const extra = [];
  if (meta?.cinematographers?.length > 0) {
    extra.push(<div key="cinematographers"><dt>摄影</dt><dd>{meta.cinematographers.join(' / ')}</dd></div>);
  }
  if (meta?.composers?.length > 0) {
    extra.push(<div key="composers"><dt>配乐</dt><dd>{meta.composers.join(' / ')}</dd></div>);
  }
  if (meta?.producers?.length > 0) {
    extra.push(<div key="producers"><dt>制片人</dt><dd>{meta.producers.join(' / ')}</dd></div>);
  }
  if (meta?.productionCompanies?.length > 0) {
    extra.push(<div key="companies"><dt>制片公司</dt><dd>{meta.productionCompanies.map((c) => c.name).join(' / ')}</dd></div>);
  }
  if (meta?.spokenLanguages?.length > 0) {
    extra.push(<div key="spoken"><dt>对白语言</dt><dd>{meta.spokenLanguages.map((l) => l.name || l.english_name).filter(Boolean).join(' / ')}</dd></div>);
  }
  if (meta?.mediaType === 'tv' && meta?.numberOfSeasons > 0) {
    extra.push(<div key="seasons"><dt>季数</dt><dd>{meta.numberOfSeasons} 季{meta?.numberOfEpisodes > 0 ? ` · ${meta.numberOfEpisodes} 集` : ''}</dd></div>);
  }
  if (meta?.contentRating) {
    extra.push(<div key="rating"><dt>分级</dt><dd>{meta.contentRating}</dd></div>);
  }
  if (meta?.budget > 0) {
    extra.push(<div key="budget"><dt>预算</dt><dd>$ {meta.budget.toLocaleString()}</dd></div>);
  }
  if (meta?.revenue > 0) {
    extra.push(<div key="revenue"><dt>票房</dt><dd>$ {meta.revenue.toLocaleString()}</dd></div>);
  }
  if (meta?.keywords?.length > 0) {
    extra.push(
      <div className="meta-keywords" key="keywords">
        <dt>关键词</dt>
        <dd>{meta.keywords.map((k) => <span key={k} className="tag genre">{k}</span>)}</dd>
      </div>
    );
  }
  if (meta?.homepage) {
    extra.push(<div key="homepage"><dt>主页</dt><dd><a href={meta.homepage} target="_blank" rel="noreferrer">{meta.homepage}</a></dd></div>);
  }

  const hasExtra = extra.length > 0;
  const toggle = hasExtra ? (
    <button
      type="button"
      className="meta-toggle"
      onClick={() => setExpanded((v) => !v)}
    >
      {expanded ? '收起' : `展开剩余 ${extra.length} 项`} <Icon name={expanded ? 'chevron-up' : 'chevron'} size={12} />
    </button>
  ) : null;

  const items = expanded ? [...primary, ...extra] : primary;
  // 将 toggle 挂到最后一个字段所在行，与其同行
  if (items.length > 0 && toggle) {
    const last = items[items.length - 1];
    items[items.length - 1] = (
      <div key={`${last.key}-wrap`} className="meta-last-row">
        {last}
        {toggle}
      </div>
    );
  }

  return (
    <dl className="meta-info">
      {items}
      {items.length === 0 && toggle}
    </dl>
  );
}

/** 元数据编辑表单 */
function MetaForm({ value, onChange }) {
  const set = (k, v) => onChange({ ...value, [k]: v });
  return (
    <div className="edit-form">
      <div className="form-row">
        <label>标题
          <input value={value.title || ''} onChange={(e) => set('title', e.target.value)} />
        </label>
        <label>原名
          <input value={value.originalTitle || ''} onChange={(e) => set('originalTitle', e.target.value)} />
        </label>
      </div>
      <div className="form-row">
        <label>类型（逗号分隔）
          <input value={value.genres || ''} onChange={(e) => set('genres', e.target.value)} />
        </label>
        <label>时长（分钟）
          <input type="number" value={value.runtime ?? ''} onChange={(e) => set('runtime', e.target.value ? Number(e.target.value) : null)} />
        </label>
        <label>评分
          <input type="number" step="0.1" value={value.voteAverage ?? ''} onChange={(e) => set('voteAverage', e.target.value ? Number(e.target.value) : null)} />
        </label>
        <label>媒体类型
          <select value={value.mediaType || ''} onChange={(e) => set('mediaType', e.target.value || null)}>
            <option value="">—</option>
            <option value="movie">电影</option>
            <option value="tv">电视剧</option>
          </select>
        </label>
      </div>
      <div className="form-row">
        <label>导演（逗号分隔）
          <input value={value.directors || ''} onChange={(e) => set('directors', e.target.value)} />
        </label>
        <label>上映日期
          <input type="date" value={value.releaseDate || ''} onChange={(e) => set('releaseDate', e.target.value || null)} />
        </label>
        <label>状态
          <input value={value.status || ''} placeholder="如 Released" onChange={(e) => set('status', e.target.value)} />
        </label>
      </div>
      <label>主演（逗号分隔）
        <input value={value.cast || ''} onChange={(e) => set('cast', e.target.value)} />
      </label>
      <label>宣传语
        <input value={value.tagline || ''} onChange={(e) => set('tagline', e.target.value)} />
      </label>
      <label>简介
        <textarea rows={4} value={value.overview || ''} onChange={(e) => set('overview', e.target.value)} />
      </label>
    </div>
  );
}

/** 图片替换工具条 */
function ImageTools({ filmId, type, label, hasLocal, hasRemote, onChanged, onError }) {
  const fileRef = useRef(null);
  const [busy, setBusy] = useState(false);

  const onUpload = async (e) => {
    const file = e.target.files?.[0];
    e.target.value = '';
    if (!file) return;
    setBusy(true);
    try {
      await uploadImage(filmId, type, file);
      onChanged();
    } catch (err) {
      onError(err.message);
    } finally {
      setBusy(false);
    }
  };

  const onScrape = async () => {
    setBusy(true);
    try {
      await scrapeImage(filmId, type);
      onChanged();
    } catch (err) {
      onError(err.message);
    } finally {
      setBusy(false);
    }
  };

  const onDelete = async () => {
    if (!hasLocal) return;
    if (!confirm(`确定删除本地${label}？将回退到远程 TMDB 图（若有）。`)) return;
    setBusy(true);
    try {
      await deleteImage(filmId, type);
      onChanged();
    } catch (err) {
      onError(err.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="image-tools">
      <span className="image-tools-label">{label}</span>
      <button type="button" className="btn-secondary small" disabled={busy} onClick={() => fileRef.current?.click()} title="上传本地图片替换">
        <Icon name="upload" size={13} />
      </button>
      <button type="button" className="btn-secondary small" disabled={busy || !hasRemote} onClick={onScrape} title="从 TMDB 重新下载到本地">
        <Icon name="download" size={13} />
      </button>
      <button type="button" className="btn-danger small" disabled={busy || !hasLocal} onClick={onDelete} title="删除本地图片">
        <Icon name="trash" size={13} />
      </button>
      <input ref={fileRef} type="file" accept="image/*" hidden onChange={onUpload} />
    </div>
  );
}

export default function FilmDetail({ film, onClose, onChanged, onDelete }) {
  const [metaOpen, setMetaOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState(null);
  const [editing, setEditing] = useState(false);
  const [filmForm, setFilmForm] = useState(null);
  const [metaForm, setMetaForm] = useState(null);
  const meta = film.metadata;

  const startEdit = () => {
    setEditing(true);
    setErr(null);
    setFilmForm(filmToForm(film));
    setMetaForm({
      title: meta?.title || '',
      originalTitle: meta?.originalTitle || '',
      genres: (meta?.genres || []).join(', '),
      runtime: meta?.runtime ?? '',
      voteAverage: meta?.voteAverage ?? '',
      mediaType: meta?.mediaType || '',
      overview: meta?.overview || '',
      directors: (meta?.directors || []).join(', '),
      cast: (meta?.cast || []).join(', '),
      releaseDate: meta?.releaseDate || '',
      status: meta?.status || '',
      tagline: meta?.tagline || '',
    });
  };

  const cancelEdit = () => {
    setEditing(false);
    setErr(null);
  };

  const saveAll = async () => {
    setBusy(true);
    setErr(null);
    try {
      const filmPatch = filmFormToPatch(filmForm);
      const metaPatch = {
        title: metaForm.title || null,
        original_title: metaForm.originalTitle || null,
        genres: metaForm.genres.split(',').map((s) => s.trim()).filter(Boolean),
        runtime: metaForm.runtime ?? null,
        vote_average: metaForm.voteAverage ?? null,
        media_type: metaForm.mediaType || null,
        overview: metaForm.overview || null,
        directors: metaForm.directors.split(',').map((s) => s.trim()).filter(Boolean),
        cast: metaForm.cast.split(',').map((s) => s.trim()).filter(Boolean),
        release_date: metaForm.releaseDate || null,
        status: metaForm.status || null,
        tagline: metaForm.tagline || null,
      };
      await Promise.all([
        updateFilm(film.id, filmPatch),
        updateMeta(film.id, metaPatch),
      ]);
      setEditing(false);
      onChanged();
    } catch (e) {
      setErr(e.message);
    } finally {
      setBusy(false);
    }
  };

  const handleDelete = async () => {
    if (!confirm('确定删除该影片的元数据？')) return;
    setBusy(true);
    setErr(null);
    try {
      await deleteMeta(film.id);
      onChanged();
    } catch (e) {
      setErr(e.message);
    } finally {
      setBusy(false);
    }
  };

  const handleDeleteFilm = async () => {
    if (!confirm(`确定删除观影记录「${film.name}」？该操作将同时删除其元数据与本地图片，且不可恢复。`)) return;
    setBusy(true);
    setErr(null);
    try {
      await deleteFilm(film.id);
      onDelete?.();
    } catch (e) {
      setErr(e.message);
    } finally {
      setBusy(false);
    }
  };

  const dateRange = [film.startDate, film.endDate]
    .filter(Boolean)
    .filter((v, i, a) => a.indexOf(v) === i)
    .join(' ~ ');

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal film-detail" onClick={(e) => e.stopPropagation()}>
        <button className="modal-close" onClick={onClose} title="关闭"><Icon name="close" size={16} /></button>

        <div className="detail-scroll">
          {meta?.backdropUrl && !editing && (
            <div className="detail-backdrop">
              <img src={meta.backdropUrl} alt="" />
            </div>
          )}

          <div className={`detail-main${editing ? ' edit-layout' : ''}`}>
            {!editing && (
              <div className="detail-poster">
                {meta?.posterUrl ? (
                  <img src={meta.posterUrl} alt={meta.title || film.name} />
                ) : (
                  <div className="poster-placeholder big">
                    <span className="ph-cat">{film.category}</span>
                    <span className="ph-name">{film.name}</span>
                  </div>
                )}
              </div>
            )}

            {editing ? (
              <>
                <div className="edit-media-col">
                  <div className="meta-poster-preview">
                    {meta?.posterUrl ? (
                      <img src={meta.posterUrl} alt={meta.title || film.name} />
                    ) : (
                      <div className="poster-placeholder">
                        <span className="ph-cat">{film.category}</span>
                        <span className="ph-name">{film.name}</span>
                      </div>
                    )}
                  </div>
                  <ImageTools
                    filmId={film.id}
                    type="poster"
                    label="海报"
                    hasLocal={Boolean(meta?.posterLocal)}
                    hasRemote={Boolean(meta?.posterPath)}
                    onChanged={onChanged}
                    onError={setErr}
                  />
                  <ImageTools
                    filmId={film.id}
                    type="backdrop"
                    label="背景图"
                    hasLocal={Boolean(meta?.backdropLocal)}
                    hasRemote={Boolean(meta?.backdropPath)}
                    onChanged={onChanged}
                    onError={setErr}
                  />
                </div>
                <div className="edit-forms-col">
                  <h3 className="edit-section-title"><Icon name="edit" size={16} /> 编辑观看记录</h3>
                  <FilmForm value={filmForm} onChange={setFilmForm} />
                  <h3 className="edit-section-title"><Icon name="info" size={16} /> 编辑元数据</h3>
                  <MetaForm value={metaForm} onChange={setMetaForm} />
                </div>
              </>
            ) : (
              <div className="detail-info">
                <h2>{meta?.title || film.name}</h2>
                {meta?.originalTitle && meta.originalTitle !== meta.title && (
                  <div className="original-title">{meta.originalTitle}</div>
                )}

                <div className="detail-tags">
                  <span className="cat-tag">{film.category}</span>
                  {meta?.releaseDate && <span className="tag">{meta.releaseDate.slice(0, 4)}</span>}
                  {meta?.runtime > 0 && (
                    <span className="tag"><Icon name="clock" size={12} /> {meta.runtime} 分钟</span>
                  )}
                  {meta?.genres?.map((g) => <span key={g} className="tag genre">{g}</span>)}
                  {meta?.voteAverage > 0 && (
                    <span className="tag rating"><Icon name="star" size={12} /> {meta.voteAverage.toFixed(1)}</span>
                  )}
                </div>

                {meta?.tagline && <p className="meta-tagline">“{meta.tagline}”</p>}
                {meta?.overview && <Overview text={meta.overview} />}

                {/* 影片元数据 */}
                <MetaInfo meta={meta} film={film} />

                {/* 观看数据 —— 仅 6 项 */}
                <div className="section-divider"><span>观看记录</span></div>
                <dl className="watch-info">
                  <div><dt>观看年份</dt><dd>{film.watchYear || '—'}</dd></div>
                  <div><dt>观看日期</dt><dd>{dateRange || '—'}</dd></div>
                  {film.totalEpisodes > 0 && (
                    <div><dt>总集数</dt><dd>{film.totalEpisodes}</dd></div>
                  )}
                  {film.platforms.length > 0 && (
                    <div>
                      <dt>观看平台</dt>
                      <dd className="platform-list">
                        {film.platforms.map((p) => <PlatformTag key={p} name={p} size={16} />)}
                      </dd>
                    </div>
                  )}
                  {film.location && (
                    <div><dt>观看地点</dt><dd>{film.location}</dd></div>
                  )}
                  {film.imdbId && (
                    <div>
                      <dt>IMDb</dt>
                      <dd>
                        <a href={`https://www.imdb.com/title/${film.imdbId}`} target="_blank" rel="noreferrer">
                          {film.imdbId} <Icon name="external" size={11} />
                        </a>
                      </dd>
                    </div>
                  )}
                </dl>
                {film.notes && (
                  <dl className="watch-info">
                    <div><dt>备注</dt><dd className="notes-dd">{film.notes}</dd></div>
                  </dl>
                )}
              </div>
            )}
          </div>
        </div>

        {/* 固定底部操作栏 —— 始终可见 */}
        <div className="detail-footer">
          {err && <div className="error-banner small"><Icon name="alert" size={14} /> {err}</div>}
          <div className="meta-actions">
            {editing ? (
              <>
                <button className="btn-primary" disabled={busy} onClick={saveAll}>
                  <Icon name="save" size={14} /> {busy ? '保存中…' : '保存'}
                </button>
                <button className="btn-secondary" disabled={busy} onClick={cancelEdit}>取消</button>
              </>
            ) : (
              <>
                {!film.hasMetadata && (
                  <button className="btn-primary" disabled={busy} onClick={() => setMetaOpen(true)}>
                    <Icon name="search" size={14} /> 搜索填充元数据
                  </button>
                )}
                {film.hasMetadata && (
                  <>
                    <button className="btn-secondary" disabled={busy} onClick={() => setMetaOpen(true)}>
                      <Icon name="refresh" size={14} /> 重新搜索
                    </button>
                    <button className="btn-danger" disabled={busy} onClick={handleDelete}>
                      <Icon name="trash" size={14} /> 删除元数据
                    </button>
                  </>
                )}
                <button className="btn-secondary" disabled={busy} onClick={startEdit}>
                  <Icon name="edit" size={14} /> 编辑
                </button>
                <button className="btn-danger" disabled={busy} onClick={handleDeleteFilm}>
                  <Icon name="trash" size={14} /> 删除记录
                </button>
                <span className="meta-hint">
                  {meta?.mediaType === 'tv' ? '电视剧' : meta?.mediaType === 'movie' ? '电影' : ''}
                  {meta?.updatedAt ? ` · 更新于 ${meta.updatedAt.slice(0, 10)}` : ''}
                </span>
              </>
            )}
          </div>
        </div>
      </div>

      {metaOpen && (
        <MetaSearch
          film={film}
          onClose={() => setMetaOpen(false)}
          onSaved={() => {
            setMetaOpen(false);
            onChanged();
          }}
        />
      )}
    </div>
  );
}
