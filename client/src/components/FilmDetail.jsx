import { useState, useRef, useEffect, useMemo } from 'react';
import {
  updateFilm, updateViewing, updateMeta, createFilm, deleteViewing,
  uploadImage, scrapeImage, deleteImage,
} from '../api.js';
import Icon from './Icon.jsx';
import PlatformTag from './PlatformTag.jsx';
import MetaSearch from './MetaSearch.jsx';
import ConfirmDialog from './ConfirmDialog.jsx';
import FilmForm, { filmToForm, filmFormToPatches, episodeUnit } from './FilmForm.jsx';
import { DatePicker } from './DatePicker.jsx';

const ICON_BASE = '/icon';

/** 影视简介：默认 3 行高度，内容过长时区域内滚动（自绘滚动条，避免原生滚动条出现白块） */
function Overview({ text }) {
  const viewRef = useRef(null);
  // 滑块位置 { height, top }，单位 px；null = 内容不超限，不显示
  const [thumb, setThumb] = useState(null);

  const update = () => {
    const el = viewRef.current;
    if (!el) return;
    const { scrollHeight, clientHeight, scrollTop } = el;
    if (scrollHeight <= clientHeight + 1) {
      setThumb(null);
      return;
    }
    const h = Math.max(20, (clientHeight / scrollHeight) * clientHeight);
    const maxScroll = scrollHeight - clientHeight;
    const top = maxScroll > 0 ? (scrollTop / maxScroll) * (clientHeight - h) : 0;
    setThumb({ height: h, top });
  };

  useEffect(() => {
    update();
    const el = viewRef.current;
    if (!el || typeof ResizeObserver === 'undefined') return undefined;
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => ro.disconnect();
  }, [text]);

  return (
    <div className="overview-wrap">
      <p ref={viewRef} className="overview" onScroll={update}>{text}</p>
      {thumb && (
        <div className="overview-scrollbar" aria-hidden="true">
          <div
            className="overview-thumb"
            style={{ height: thumb.height, transform: `translateY(${thumb.top}px)` }}
          />
        </div>
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

/** 元数据 → 编辑表单数据 */
function metaToForm(m) {
  return {
    title: m?.title || '',
    originalTitle: m?.originalTitle || '',
    genres: (m?.genres || []).join(', '),
    runtime: m?.runtime ?? '',
    voteAverage: m?.voteAverage ?? '',
    mediaType: m?.mediaType || '',
    overview: m?.overview || '',
    directors: (m?.directors || []).join(', '),
    cast: (m?.cast || []).join(', '),
    releaseDate: m?.releaseDate || '',
    status: m?.status || '',
    tagline: m?.tagline || '',
  };
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
          <DatePicker value={value.releaseDate} onChange={(v) => set('releaseDate', v)} title="上映日期" />
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

/**
 * 海报下方的评分与 ID：豆瓣 / IMDb 各占一行（上下排列），整行以来源主题色为底。
 * 每行左侧为来源 LOGO + 可点击的 ID，右侧为评分值（无评分时灰字「无评分」）。
 * ratingView：评分展示偏好（all / douban / imdb / none），只渲染被允许的来源；
 * 某来源既无 ID 也无评分时该行不渲染；两行都没有则整块不渲染。
 */
function DetailStats({ film, ratingView = 'all' }) {
  const { doubanId, doubanRating, imdbId, imdbRating } = film;
  const showDouban = ratingView === 'all' || ratingView === 'douban';
  const showImdb = ratingView === 'all' || ratingView === 'imdb';
  const hasDouban = showDouban && (Boolean(doubanId) || doubanRating > 0);
  const hasImdb = showImdb && (Boolean(imdbId) || imdbRating > 0);
  if (!hasDouban && !hasImdb) return null;
  return (
    <div className="detail-stats">
      {hasDouban && (
        <div className="detail-stat douban">
          <span className="detail-stat-left">
            <img className="detail-stat-logo" src={`${ICON_BASE}/douban.svg`} alt="豆瓣" />
            {doubanId && (
              <a
                className="detail-stat-id"
                href={`https://movie.douban.com/subject/${doubanId}/`}
                target="_blank"
                rel="noreferrer"
                title={`豆瓣: ${doubanId}`}
              >
                {doubanId}
              </a>
            )}
          </span>
          <span className={`detail-stat-value${doubanRating > 0 ? '' : ' empty'}`}>
            {doubanRating > 0 ? (
              <>
                {doubanRating.toFixed(1)}
                <span className="detail-stat-max"> / 10</span>
              </>
            ) : '无评分'}
          </span>
        </div>
      )}
      {hasImdb && (
        <div className="detail-stat imdb">
          <span className="detail-stat-left">
            <img className="detail-stat-logo" src={`${ICON_BASE}/imdb.svg`} alt="IMDb" />
            {imdbId && (
              <a
                className="detail-stat-id"
                href={`https://www.imdb.com/title/${imdbId}`}
                target="_blank"
                rel="noreferrer"
                title={`IMDb: ${imdbId}`}
              >
                {imdbId}
              </a>
            )}
          </span>
          <span className={`detail-stat-value${imdbRating > 0 ? '' : ' empty'}`}>
            {imdbRating > 0 ? (
              <>
                {imdbRating.toFixed(1)}
                <span className="detail-stat-max"> / 10</span>
              </>
            ) : '无评分'}
          </span>
        </div>
      )}
    </div>
  );
}

export default function FilmDetail({
  film, onClose, onChanged,
  initialEditing = false, initialMetaOpen = false,
  readOnly = false, ratingView = 'all',
}) {
  const metaOpen = initialMetaOpen && !readOnly;
  const [metaOpenState, setMetaOpen] = useState(metaOpen);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState(null);
  const [posterLoaded, setPosterLoaded] = useState(false);
  const [backdropLoaded, setBackdropLoaded] = useState(false);
  const [filmForm, setFilmForm] = useState(null);
  const [metaForm, setMetaForm] = useState(null);
  // 当前正在编辑的观看记录 id（null = 非编辑模式；负数 = 暂存的新增草稿）
  const [editingViewingId, setEditingViewingId] = useState(null);
  // 观看记录暂存区：新增草稿（仅存在于前端）与待移除的真实记录 id，点击「保存」后才提交
  const [stagedAdds, setStagedAdds] = useState([]);
  const [stagedRemoveIds, setStagedRemoveIds] = useState([]);
  // 草稿临时 id 序列（负数，避免与真实 id 冲突）
  const draftSeqRef = useRef(-1);
  // 移除确认弹窗
  const [removeConfirm, setRemoveConfirm] = useState(false);
  // 关闭前存在未保存修改的确认弹窗
  const [closeConfirm, setCloseConfirm] = useState(false);
  // 进入编辑时的原始数据快照，用于精确判断是否有未保存修改
  const pristineRef = useRef(null);
  const meta = film.metadata;
  const filmId = film.filmId ?? film.id;

  // 观看记录列表：完整详情来自后端；列表条目（未拉取详情时）退化为单条伪记录
  const viewings = film.viewings || [{
    id: film.id,
    watchYear: film.watchYear,
    startDate: film.startDate,
    endDate: film.endDate,
    platforms: film.platforms || [],
    location: film.location,
    notes: film.notes,
    watchStatus: film.watchStatus,
  }];

  // initialEditing 时立即进入编辑：优先匹配被点击的那条观看记录
  useEffect(() => {
    if (initialEditing && !readOnly && editingViewingId == null && !filmForm) {
      const target = viewings.find((v) => v.id === film.id) || viewings[0];
      startEdit(target);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // initialMetaOpen 由外部每次打开时传入，同步到内部状态
  useEffect(() => {
    setMetaOpen(metaOpen);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [metaOpen]);

  // 表单当前值 → 草稿的观看级字段（用于暂存新增记录的编辑内容）
  const formToDraft = (f) => ({
    watchYear: f.watchYear ?? null,
    startDate: f.startDate || null,
    endDate: f.endDate || null,
    watchStatus: f.watchStatus === 'watching' ? 'watching' : 'finished',
    platforms: (f.platformsRaw || '').split(',').map((s) => s.trim()).filter(Boolean),
    location: f.location || null,
    notes: f.notes || null,
  });

  const startEdit = (viewing) => {
    setEditingViewingId(viewing?.id ?? null);
    setErr(null);
    // 进入编辑时清空暂存区，保证全新的编辑会话
    setStagedAdds([]);
    setStagedRemoveIds([]);
    setFilmForm(filmToForm(film, viewing));
    setMetaForm(metaToForm(meta));
    // 记录原始数据快照，供精确比对未保存修改
    pristineRef.current = { film, viewings, meta };
  };

  const cancelEdit = () => {
    setEditingViewingId(null);
    setFilmForm(null);
    setMetaForm(null);
    setErr(null);
    // 取消即丢弃全部暂存变更（未提交的新增草稿/移除标记一并清空）
    setStagedAdds([]);
    setStagedRemoveIds([]);
  };

  const saveAll = async () => {
    setBusy(true);
    setErr(null);
    try {
      const { film: filmPatch, viewing: viewingPatch } = filmFormToPatches(filmForm);
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

      // 当前编辑中的草稿并入暂存列表（其余草稿在切换选项卡时已写回）
      const adds = stagedAdds.map((d) => (
        d.id === filmForm.viewingId ? { ...d, ...formToDraft(filmForm) } : d
      ));
      const currentId = filmForm.viewingId ?? editingViewingId;
      const currentRemoved = currentId != null && stagedRemoveIds.includes(currentId);

      // 1) 先保存影视级字段与元数据（可能的改名需先落实，后续按名称匹配追加观看记录）
      await Promise.all([
        updateFilm(filmId, filmPatch),
        updateMeta(filmId, metaPatch),
      ]);

      // 2) 更新当前观看记录 + 提交暂存的新增（新增必须先于移除执行，
      //    否则移除最后一条记录会连带删除影视，导致新增被建成丢失元数据的新影视）
      await Promise.all([
        currentId != null && currentId >= 0 && !currentRemoved
          ? updateViewing(currentId, viewingPatch)
          : null,
        ...adds.map((d) => createFilm({
          name: filmForm.name,
          watch_year: d.watchYear ?? null,
          start_date: d.startDate || null,
          end_date: d.endDate || null,
          watch_status: d.watchStatus === 'watching' ? 'watching' : 'finished',
          platforms_raw: d.platforms.length > 0 ? d.platforms.join(',') : null,
          location: d.location || null,
          notes: d.notes || null,
        })),
      ]);

      // 3) 最后执行暂存的移除（若移除的是最后一条记录，后端会连同影视与元数据一并删除）
      const removed = await Promise.all(stagedRemoveIds.map((id) => deleteViewing(id)));
      const filmDeleted = removed.some((r) => r?.filmDeleted);

      cancelEdit();
      onChanged();
      if (filmDeleted) onClose();
    } catch (e) {
      setErr(e.message);
    } finally {
      setBusy(false);
    }
  };

  const editing = editingViewingId != null;
  // 当前正在编辑的观看记录是否已暂存移除
  const currentViewingId = filmForm?.viewingId ?? editingViewingId;
  const viewingRemoved = currentViewingId != null && currentViewingId >= 0
    && stagedRemoveIds.includes(currentViewingId);

  // 精确判断是否有未保存修改：与进入编辑时的原始快照逐项比对
  const dirty = useMemo(() => {
    if (!editing) return false;
    if (stagedAdds.length > 0 || stagedRemoveIds.length > 0) return true;
    if (!filmForm || !metaForm) return false;
    const p = pristineRef.current;
    if (!p) return false;
    // 元数据字段比对
    if (JSON.stringify(metaForm) !== JSON.stringify(metaToForm(p.meta))) return true;
    // 影视级 + 当前观看记录字段比对
    const pristineViewing = p.viewings.find((v) => v.id === filmForm.viewingId);
    if (pristineViewing && JSON.stringify(filmForm) !== JSON.stringify(filmToForm(p.film, pristineViewing))) {
      return true;
    }
    return false;
  }, [editing, filmForm, metaForm, stagedAdds, stagedRemoveIds]);

  // 切换正在编辑的观看记录：写回当前草稿的编辑值，保留影视级字段的未保存修改
  const switchToViewing = (target) => {
    if (!target || !filmForm) return;
    if (filmForm.viewingId != null && filmForm.viewingId < 0) {
      const draft = formToDraft(filmForm);
      setStagedAdds((prev) => prev.map((d) => (d.id === filmForm.viewingId ? { ...d, ...draft } : d)));
    }
    setEditingViewingId(target.id);
    setFilmForm({
      ...filmToForm(film, target),
      // 影视级字段沿用当前编辑值，切换选项卡不丢失未保存的修改
      name: filmForm.name,
      category: filmForm.category,
      releaseYear: filmForm.releaseYear,
      totalEpisodes: filmForm.totalEpisodes,
      productionCountriesRaw: filmForm.productionCountriesRaw,
      imdbId: filmForm.imdbId,
      doubanId: filmForm.doubanId,
    });
  };

  // 编辑模式下切换要编辑的观看记录
  const onViewingSelect = (viewingId) => {
    if (viewingId === filmForm?.viewingId) return;
    const target = [...viewings, ...stagedAdds].find((v) => v.id === viewingId);
    if (!target) return;
    switchToViewing(target);
  };

  // 选项卡数据：真实记录（含暂存移除标记）+ 暂存的新增草稿
  const viewingOptions = [
    ...viewings.map((v) => ({
      id: v.id,
      watchYear: v.watchYear,
      pendingRemove: stagedRemoveIds.includes(v.id),
      isNew: false,
    })),
    ...stagedAdds.map((d) => ({
      id: d.id,
      watchYear: d.watchYear,
      pendingRemove: false,
      isNew: true,
    })),
  ];

  // 为该影视新增一条观看记录（仅暂存草稿，点击「保存」后才提交）
  const handleAddViewing = () => {
    if (!filmForm) return;
    setErr(null);
    const newId = draftSeqRef.current--;
    const draft = {
      id: newId,
      watchYear: null,
      startDate: null,
      endDate: null,
      watchStatus: 'finished',
      platforms: [],
      location: null,
      notes: null,
    };
    setStagedAdds((prev) => {
      // 当前若也是草稿，先写回其编辑值
      const base = filmForm.viewingId != null && filmForm.viewingId < 0
        ? prev.map((d) => (d.id === filmForm.viewingId ? { ...d, ...formToDraft(filmForm) } : d))
        : prev;
      return [...base, draft];
    });
    // 切换到新草稿（影视级字段保留当前编辑值）
    setFilmForm((f) => ({
      ...f,
      viewingId: newId,
      watchYear: null,
      startDate: null,
      endDate: null,
      watchStatus: 'finished',
      platformsRaw: '',
      location: '',
      notes: '',
    }));
    setEditingViewingId(newId);
  };

  // 移除当前观看记录（危险操作，二次确认；草稿直接丢弃，真实记录暂存标记）
  const handleRemoveViewing = () => {
    setRemoveConfirm(true);
  };

  const doRemoveViewing = () => {
    const id = filmForm?.viewingId ?? editingViewingId;
    if (id == null) {
      setRemoveConfirm(false);
      return;
    }
    if (id < 0) {
      // 暂存草稿：直接丢弃
      setStagedAdds((prev) => prev.filter((d) => d.id !== id));
    } else if (!stagedRemoveIds.includes(id)) {
      setStagedRemoveIds((prev) => [...prev, id]);
    }
    setRemoveConfirm(false);
    // 切换到第一个仍可编辑的选项卡（未标记移除的真实记录或草稿）
    const selectable = [
      ...viewings.filter((v) => v.id !== id && !stagedRemoveIds.includes(v.id)),
      ...stagedAdds.filter((d) => d.id !== id),
    ];
    const fallback = selectable[0]
      ?? [...viewings.filter((v) => v.id !== id), ...stagedAdds.filter((d) => d.id !== id)][0];
    if (fallback) switchToViewing(fallback);
    // 无任何剩余记录时不切换：表单停留在已标记移除的记录上，字段禁用并显示提示
  };

  // 点击遮罩关闭：编辑中存在未保存修改时先二次确认
  const handleOverlayClose = () => {
    if (editing && dirty) setCloseConfirm(true);
    else onClose();
  };

  return (
    <>
    <div
      className="modal-overlay"
      onMouseDown={(e) => { if (e.target === e.currentTarget) handleOverlayClose(); }}
    >
      <div className="modal film-detail" onClick={(e) => e.stopPropagation()}>
        <button className="modal-close" onClick={onClose} title="关闭"><Icon name="close" size={16} /></button>

        {/* 编辑态标题行：为右上角绝对定位的 .modal-close 留出独立空间，
            避免它压住表单第一行控件；固定在滚动区之上，不随内容滚动 */}
        {editing && (
          <div className="detail-head">
            {/* 标题图标尺寸 = 标题字号（18px），与「新增记录」「评分管理」两个弹窗的标题图标一致 */}
            <h3><Icon name="edit" size={18} /> 编辑「{film.name}」</h3>
          </div>
        )}

        <div className="detail-scroll">
          {meta?.backdropUrl && !editing && (
            <div className="detail-backdrop">
              <img
                src={meta.backdropUrl}
                alt=""
                className={backdropLoaded ? 'loaded' : ''}
                onLoad={() => setBackdropLoaded(true)}
              />
            </div>
          )}

          <div className={`detail-main${editing ? ' edit-layout' : ''}`}>
            {!editing && (
              <div className="detail-poster-col">
                <div className="detail-poster">
                  {meta?.posterUrl ? (
                    <img
                      src={meta.posterUrl}
                      alt={meta.title || film.name}
                      className={posterLoaded ? 'loaded' : ''}
                      onLoad={() => setPosterLoaded(true)}
                    />
                  ) : (
                    <div className="poster-placeholder big">
                      <span className="ph-cat">{film.category}</span>
                      <span className="ph-name">{film.name}</span>
                    </div>
                  )}
                </div>
                <DetailStats film={film} ratingView={ratingView} />
                {/* margin-top: auto 让提示贴住列底，评分块则紧贴海报下方 */}
                <div className="detail-meta-hint">
                  {meta?.mediaType === 'tv' ? '电视剧' : meta?.mediaType === 'movie' ? '电影' : ''}
                  {meta?.updatedAt ? ` · 更新于 ${meta.updatedAt.slice(0, 10)}` : ''}
                </div>
              </div>
            )}

            {editing ? (
              <>
                <div className="edit-media-col">
                  <div className="meta-poster-preview">
                    {meta?.posterUrl ? (
                      <img
                        src={meta.posterUrl}
                        alt={meta.title || film.name}
                        className={posterLoaded ? 'loaded' : ''}
                        onLoad={() => setPosterLoaded(true)}
                      />
                    ) : (
                      <div className="poster-placeholder">
                        <span className="ph-cat">{film.category}</span>
                        <span className="ph-name">{film.name}</span>
                      </div>
                    )}
                  </div>
                  <ImageTools
                    filmId={filmId}
                    type="poster"
                    label="海报"
                    hasLocal={Boolean(meta?.posterLocal)}
                    hasRemote={Boolean(meta?.posterPath)}
                    onChanged={onChanged}
                    onError={setErr}
                  />
                  <ImageTools
                    filmId={filmId}
                    type="backdrop"
                    label="背景图"
                    hasLocal={Boolean(meta?.backdropLocal)}
                    hasRemote={Boolean(meta?.backdropPath)}
                    onChanged={onChanged}
                    onError={setErr}
                  />
                </div>
                <div className="edit-forms-col">
                  <FilmForm
                    value={filmForm}
                    onChange={setFilmForm}
                    viewingOptions={viewingOptions}
                    onViewingSelect={onViewingSelect}
                    onAddViewing={handleAddViewing}
                    onRemoveViewing={handleRemoveViewing}
                    viewingActionsDisabled={busy}
                    viewingRemoved={viewingRemoved}
                    pendingCount={{ adds: stagedAdds.length, removes: stagedRemoveIds.length }}
                  />
                  <h3 className="edit-section-title"><Icon name="film" size={16} /> 影视元数据</h3>
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
                  {film.totalEpisodes > 0 && episodeUnit(film.category) && (
                    <span className="tag" title="总集/期数"><Icon name="film" size={12} /> 共 {film.totalEpisodes} {episodeUnit(film.category)}</span>
                  )}
                  {meta?.releaseDate && <span className="tag">{meta.releaseDate.slice(0, 4)}</span>}
                  {meta?.runtime > 0 && (
                    <span className="tag"><Icon name="clock" size={12} /> {meta.runtime} 分钟</span>
                  )}
                  {meta?.genres?.map((g) => <span key={g} className="tag genre">{g}</span>)}
                </div>

                {meta?.tagline && <p className="meta-tagline">“{meta.tagline}”</p>}
                {meta?.overview && <Overview text={meta.overview} />}

                {/* 影视元数据 */}
                <MetaInfo meta={meta} film={film} />

                {/* 观看记录 —— 该影视的全部观看记录 */}
                <div className="section-divider"><span>观看记录</span></div>
                {viewings.map((v, idx) => {
                  const dateRange = [v.startDate, v.endDate]
                    .filter(Boolean)
                    .filter((val, i, a) => a.indexOf(val) === i)
                    .join(' ~ ');
                  return (
                    <div className="viewing-record" key={v.id ?? idx}>
                      <div className="viewing-record-head">
                        <span className="viewing-index">
                          <Icon name="calendar" size={12} /> 第 {idx + 1} 次观看{v.watchYear ? ` · ${v.watchYear} 年` : ''}
                          {v.watchStatus === 'watching' && (
                            <span className="watching-tag" title="正在观看；看完后编辑本条记录补上结束观看日期即可">
                              <Icon name="clock" size={11} /> 正在观看
                            </span>
                          )}
                        </span>
                        {!readOnly && (
                          <button
                            type="button"
                            className="btn-secondary small"
                            onClick={() => startEdit(v)}
                          >
                            <Icon name="edit" size={12} /> 编辑
                          </button>
                        )}
                      </div>
                      <dl className="watch-info">
                        <div><dt>观看年份</dt><dd>{v.watchYear || '—'}</dd></div>
                        <div><dt>观看日期</dt><dd>{dateRange || '—'}</dd></div>
                        {v.platforms.length > 0 && (
                          <div>
                            <dt>观看平台</dt>
                            <dd className="platform-list">
                              {v.platforms.map((p) => <PlatformTag key={p} name={p} size={16} />)}
                            </dd>
                          </div>
                        )}
                        {v.location && (
                          <div><dt>观看地点</dt><dd>{v.location}</dd></div>
                        )}
                      </dl>
                      {v.notes && (
                        <dl className="watch-info">
                          <div><dt>备注</dt><dd className="notes-dd">{v.notes}</dd></div>
                        </dl>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>

        {/* 编辑模式：底部固定保存/取消栏 */}
        {editing && (
          <div className="detail-footer">
            {err && <div className="error-banner small"><Icon name="alert" size={14} /> {err}</div>}
            <div className="meta-actions">
              <button className="btn-primary" disabled={busy} onClick={saveAll}>
                <Icon name="save" size={14} /> {busy ? '保存中…' : '保存'}
              </button>
              <button className="btn-secondary" disabled={busy} onClick={cancelEdit}>取消</button>
              {dirty && (
                <span className="editor-dirty-hint"><Icon name="alert" size={12} /> 有未保存修改</span>
              )}
            </div>
          </div>
        )}
      </div>

      {metaOpenState && (
        <MetaSearch
          film={{ ...film, id: filmId }}
          onClose={() => setMetaOpen(false)}
          onSaved={() => {
            setMetaOpen(false);
            onChanged();
          }}
        />
      )}
    </div>

    <ConfirmDialog
      open={removeConfirm}
      title="移除观看记录"
      message={currentViewingId != null && currentViewingId < 0
        ? '确定移除这条尚未保存的新增记录？其中已填写的内容将丢弃。'
        : `确定移除「${film.name}」的当前观看记录？\n移除将在点击「保存」后生效；若该影视仅剩此条记录，其元数据与本地图片也将一并删除。`}
      confirmText="移除"
      danger
      onConfirm={doRemoveViewing}
      onCancel={() => setRemoveConfirm(false)}
    />

    <ConfirmDialog
      open={closeConfirm}
      title="有未保存的修改"
      message="关闭后将丢失未保存的修改，确定要关闭吗？"
      confirmText="放弃修改"
      cancelText="继续编辑"
      danger
      onConfirm={onClose}
      onCancel={() => setCloseConfirm(false)}
    />
    </>
  );
}
