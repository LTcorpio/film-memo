import { useState, useEffect } from 'react';
import { updateFilm, refreshRatings, stopRatings, fetchRatingStatus } from '../api.js';
import Icon from './Icon.jsx';

/** 进度百分比（total 尚未就绪时为 0） */
function jobPercent(job) {
  if (!job || !job.total) return 0;
  return Math.floor((job.done / job.total) * 100);
}

/** 时长：毫秒 → 「3 分 20 秒」 */
function formatDuration(ms) {
  const total = Math.max(0, Math.round(ms / 1000));
  const m = Math.floor(total / 60);
  return m > 0 ? `${m} 分 ${total % 60} 秒` : `${total} 秒`;
}

/** 当前阶段文案（对应后端 phase） */
function phaseText(job) {
  switch (job.phase) {
    case 'prep':
      return '准备数据…';
    case 'imdb':
      return '下载并解析 IMDb 数据集…';
    case 'douban':
      return job.total ? '抓取豆瓣评分' : '抓取豆瓣评分…';
    case 'write':
      return '写入数据库…';
    case 'done':
      return '已完成';
    default:
      return '';
  }
}

/** 进度条右侧：运行中显示已用/预计剩余，结束后显示用时 */
function progressTiming(job, percent) {
  if (!job.startedAt) return '';
  const started = new Date(job.startedAt).getTime();
  if (Number.isNaN(started)) return '';
  if (!job.running) {
    const end = job.finishedAt ? new Date(job.finishedAt).getTime() : Date.now();
    return `用时 ${formatDuration(end - started)}`;
  }
  const elapsed = Date.now() - started;
  const parts = [`已用 ${formatDuration(elapsed)}`];
  // 按已完成的平均耗时外推剩余时间（done 为 0 时无从估算）
  if (job.done > 0 && job.total > job.done) {
    const remain = (elapsed / job.done) * (job.total - job.done);
    parts.push(`预计剩余 ${formatDuration(remain)}`);
  }
  if (percent > 0) parts.push(`${percent}%`);
  return parts.join(' · ');
}

/** 单行：选中标记 + 剧集名称 + 类型徽章 + 各来源「ID + 精简评分」徽章 + ID 输入 + 保存按钮 */
function RatingRow({ film, selected, onToggle, onSaved }) {
  const [imdbId, setImdbId] = useState(film.imdbId || '');
  const [doubanId, setDoubanId] = useState(film.doubanId || '');
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [err, setErr] = useState(null);

  // 点行切换选中；输入框/按钮/label 上的点击交给原有交互，不误触发选中
  const onRowClick = (e) => {
    if (e.target.closest('input, button, label, a')) return;
    onToggle(film.filmId);
  };

  // 归一化：trim 后空值视为 ''，与 save 的持久化逻辑（trim || null）一致
  const norm = (v) => (v && v.trim()) || '';
  const persistedImdbId = norm(film.imdbId);
  const persistedDoubanId = norm(film.doubanId);
  const dirty = norm(imdbId) !== persistedImdbId || norm(doubanId) !== persistedDoubanId;
  // 徽章内只显示精简评分
  const imdbScore = film.imdbRating > 0 ? film.imdbRating.toFixed(1) : '—';
  const doubanScore = film.doubanRating > 0 ? film.doubanRating.toFixed(1) : '—';

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
    <div
      className={`rating-row${selected ? ' selected' : ''}`}
      onClick={onRowClick}
      title={selected ? '已选中，点击取消' : '点击选中，更新时只处理选中的影视'}
    >
      <span className={`rating-row-pick${selected ? ' on' : ''}`} aria-hidden="true">
        {selected && <Icon name="check" size={12} />}
      </span>
      <div className="rating-row-info">
        <div className="rating-row-title">
          {film.name}
          <span className="rating-row-cat">{film.category || '—'}</span>
        </div>
        <div className="rating-row-meta">
          {persistedImdbId && (
            <span className="rating-source-badge imdb">IMDb {imdbScore}</span>
          )}
          {persistedDoubanId && (
            <span className="rating-source-badge douban">豆瓣 {doubanScore}</span>
          )}
          {!persistedImdbId && !persistedDoubanId && (
            <span className="rating-source-badge none">未填 ID</span>
          )}
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

/** 同步时间格式化：ISO → 本地 YYYY-MM-DD HH:mm */
function formatSyncedAt(iso) {
  if (!iso) return '尚未拉取';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '尚未拉取';
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}

/** 单源刷新结果摘要：已更新 x · 跳过 y · 失败 z */
function sourceResultText(s) {
  return `${s.updated} 已更新 · ${s.skipped} 跳过 · ${s.failed} 失败`;
}

/** 后台任务的进度区块：阶段文案 + 进度条 + 计时；结束时展示结果摘要 */
function JobProgress({ job }) {
  const percent = jobPercent(job);
  const summary = (job.result?.sources || [])
    .map((s) => `${s.source === 'douban' ? '豆瓣' : 'IMDb'} ${sourceResultText(s)}`)
    .join(' · ');
  // 被手动停止时进度停在原处，不假装跑满
  const width = job.running || job.stopped ? percent : 100;

  return (
    <div className="rating-progress">
      <div className="rating-progress-top">
        <span className="rating-progress-phase">
          {job.running ? phaseText(job) : job.stopped ? '已停止' : '已完成'}
          {job.running && job.phase === 'douban' && job.total > 0 && (
            <b> {job.done} / {job.total}</b>
          )}
        </span>
        <span className="rating-progress-eta">{progressTiming(job, percent)}</span>
      </div>
      <div className={`rating-progress-track${job.running ? '' : ' done'}`}>
        <div className="rating-progress-fill" style={{ width: `${width}%` }} />
      </div>
      {!job.running && summary && (
        <div className="rating-progress-result">
          {job.stopped && <span className="rating-progress-stopped">已停止，保留已完成部分 · </span>}
          {job.result.total === 0
            ? '数据已完整，无需补充'
            : `${summary}（共 ${job.result.total}）`}
        </div>
      )}
      {job.error && (
        <div className="rating-progress-result err">
          <Icon name="alert" size={12} /> {job.error}
        </div>
      )}
    </div>
  );
}

export default function RatingManager({ films, filters, job, onClose, onChanged }) {
  const [refreshErr, setRefreshErr] = useState(null);
  // 点击后到进度轮询反映出来之间有短暂间隙，用本地状态立即给出反馈
  const [starting, setStarting] = useState(false);
  // 各来源最近一次拉取记录：{ douban: {...}, imdb: {...} }
  const [syncStatus, setSyncStatus] = useState({});
  // 保存后的覆盖层：filmId -> { imdbId, doubanId }（用于即时刷新徽标，无需重拉列表）
  const [overrides, setOverrides] = useState({});
  // 手动勾选的影视（filmId 集合）：非空时更新只作用于这些条目
  const [selected, setSelected] = useState(() => new Set());
  const [stopping, setStopping] = useState(false);

  const loadStatus = async () => {
    try {
      const res = await fetchRatingStatus();
      const map = {};
      (res.sources || []).forEach((s) => { map[s.source] = s; });
      setSyncStatus(map);
    } catch {
      // 状态读取失败不阻塞主流程，仅不展示时间
    }
  };

  useEffect(() => { loadStatus(); }, []);

  // 轮询拿到运行中的任务后，撤下本地「启动中」占位
  useEffect(() => {
    if (job?.running) setStarting(false);
  }, [job]);

  useEffect(() => {
    if (!starting) return undefined;
    const t = setTimeout(() => setStarting(false), 4000); // 兜底：轮询未及时反映时撤下
    return () => clearTimeout(t);
  }, [starting]);

  // 任务结束（或刷新结果更新）后同步「上次更新」时间
  useEffect(() => {
    if (job && !job.running && job.finishedAt) loadStatus();
  }, [job?.finishedAt]);

  const running = Boolean(job?.running) || starting;

  const handleRowSaved = (filmId, payload) => {
    setOverrides((o) => ({ ...o, [filmId]: payload }));
  };

  const doRefresh = async (source) => {
    setRefreshErr(null);
    setStarting(true);
    try {
      // 有勾选时只更新勾选的影视（按 id 过滤），否则按当前筛选条件；
      // 后台任务：立即返回，进度由 App 统一轮询下发
      const scope = selected.size > 0 ? { ...filters, ids: [...selected] } : filters;
      await refreshRatings(scope, source);
    } catch (e) {
      setRefreshErr(e.message);
      setStarting(false);
    }
  };

  const doStop = async () => {
    setRefreshErr(null);
    setStopping(true);
    try {
      await stopRatings();
    } catch (e) {
      setRefreshErr(e.message);
    } finally {
      setStopping(false);
    }
  };

  const toggleSelect = (filmId) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(filmId)) next.delete(filmId);
      else next.add(filmId);
      return next;
    });
  };

  // 列表随筛选/搜索变化后，剔除已不可见的选中项，避免对看不到的条目误操作
  useEffect(() => {
    const visible = new Set(films.map((f) => f.filmId));
    setSelected((prev) => {
      if ([...prev].every((id) => visible.has(id))) return prev;
      return new Set([...prev].filter((id) => visible.has(id)));
    });
  }, [films]);

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

  // 任务来源与按钮对应关系：source 为空串表示两个来源都跑
  const busySource = running ? (job?.source ?? '') : undefined;
  // 选中数量：按钮文案据此变化，让「只更新选中项」这件事在按下前就可见
  const pickCount = selected.size;
  const pickSuffix = pickCount > 0 ? ` (${pickCount})` : '';
  const allPicked = rows.length > 0 && rows.every((f) => selected.has(f.filmId));
  const toggleAll = () =>
    setSelected(allPicked ? new Set() : new Set(rows.map((f) => f.filmId)));

  return (
    <div
      className="modal-overlay"
      onMouseDown={(e) => { if (e.target === e.currentTarget) onClose(); }}
    >
      <div className="modal rating-manager" onClick={(e) => e.stopPropagation()}>
        <button className="modal-close" onClick={onClose} title="关闭"><Icon name="close" size={16} /></button>
        {/* 标题图标尺寸 = 标题字号（18px），沿用 .app-header h1「图标 26px 配 26px 字」的规则 */}
        <h3><Icon name="star" size={18} /> 评分管理</h3>
        <div className="rating-manager-sub">
          共 {films.length} 条记录 · 统一维护 IMDb 号与豆瓣 ID，评分一次拉取后存入数据库，无需重复请求
        </div>

        <div className="rating-sync">
          <div className="rating-sync-item">
            <span className="rating-sync-name douban">豆瓣</span>
            <span className="rating-sync-time">上次更新：{formatSyncedAt(syncStatus.douban?.lastSyncedAt)}</span>
          </div>
          <div className="rating-sync-item">
            <span className="rating-sync-name imdb">IMDb</span>
            <span className="rating-sync-time">上次更新：{formatSyncedAt(syncStatus.imdb?.lastSyncedAt)}</span>
          </div>
        </div>

        <div className="rating-toolbar">
          <button
            type="button"
            className={`btn-primary${busySource === 'douban' ? ' busy' : ''}`}
            disabled={running || films.length === 0}
            onClick={() => doRefresh('douban')}
            title="补齐缺失的豆瓣评分（已有评分的会跳过）"
          >
            <Icon name="refresh" size={14} /> 更新豆瓣评分{pickSuffix}
          </button>
          <button
            type="button"
            className={`btn-primary${busySource === 'imdb' ? ' busy' : ''}`}
            disabled={running || films.length === 0}
            onClick={() => doRefresh('imdb')}
            title="补齐缺失的 IMDb 评分（已有评分的会跳过）"
          >
            <Icon name="refresh" size={14} /> 更新 IMDb 评分{pickSuffix}
          </button>
          <button
            type="button"
            className={`btn-secondary${busySource === '' ? ' busy' : ''}`}
            disabled={running || films.length === 0}
            onClick={() => doRefresh()}
            title="补齐两个来源缺失的评分"
          >
            <Icon name="refresh" size={14} /> 全部更新{pickSuffix}
          </button>
          {(Boolean(job?.running) || starting) && (
            <button
              type="button"
              className="btn-danger small"
              disabled={stopping}
              onClick={doStop}
              title="停止当前任务；已抓取到的结果会保留并写入"
            >
              <Icon name="close" size={14} /> {stopping ? '停止中…' : '停止'}
            </button>
          )}
          {refreshErr && <span className="rating-result err"><Icon name="alert" size={12} /> {refreshErr}</span>}
          {running && <span className="rating-result">任务在后台运行，可关闭本窗口</span>}
        </div>

        {job && (job.running || job.finishedAt) && <JobProgress job={job} />}

        <div className="rating-pickbar">
          <button
            type="button"
            className="rating-pickall"
            disabled={rows.length === 0}
            onClick={toggleAll}
          >
            {allPicked ? '取消全选' : '全选本页'}
          </button>
          <span className="rating-picksum">
            {pickCount > 0
              ? `已选 ${pickCount} 项，点击更新只处理选中的影视`
              : '点击影视行可多选；未选中时按当前筛选条件更新'}
          </span>
        </div>

        <div className="rating-list">
          {rows.length === 0 ? (
            <div className="results-empty">无匹配记录，请先调整筛选条件。</div>
          ) : (
            rows.map((f) => (
              <RatingRow
                key={f.filmId}
                film={f}
                selected={selected.has(f.filmId)}
                onToggle={toggleSelect}
                onSaved={handleRowSaved}
              />
            ))
          )}
        </div>
      </div>
    </div>
  );
}
