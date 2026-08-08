import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  fetchFilms, fetchFilters, fetchStats, fetchFilm, createFilm,
  deleteMeta, deleteFilm,
} from './api.js';
import Filters from './components/Filters.jsx';
import FilmCard from './components/FilmCard.jsx';
import FilmDetail from './components/FilmDetail.jsx';
import FilmForm, { emptyFilmForm, filmFormToPatch } from './components/FilmForm.jsx';
import Paginator, { DEFAULT_ROWS, DEFAULT_LIST_SIZE } from './components/Paginator.jsx';
import Icon from './components/Icon.jsx';
import ContextMenu from './components/ContextMenu.jsx';
import ConfirmDialog from './components/ConfirmDialog.jsx';
import FilmList from './components/FilmList.jsx';
import RatingManager from './components/RatingManager.jsx';

const ROWS_KEY = 'film-memo:rows-per-page';
const VIEW_KEY = 'film-memo:view-mode';
const LIST_SIZE_KEY = 'film-memo:list-size';

function loadRows() {
  const v = Number(localStorage.getItem(ROWS_KEY));
  return Number.isFinite(v) && v > 0 ? v : DEFAULT_ROWS;
}

function loadViewMode() {
  const v = localStorage.getItem(VIEW_KEY);
  return v === 'list' ? 'list' : 'grid';
}

function loadListSize() {
  const v = Number(localStorage.getItem(LIST_SIZE_KEY));
  return Number.isFinite(v) && v > 0 ? v : DEFAULT_LIST_SIZE;
}

export default function App() {
  const [filters, setFilters] = useState({
    watchYear: '', releaseYear: '', platform: '', category: '', q: '',
  });
  const [films, setFilms] = useState([]);
  const [filterOpts, setFilterOpts] = useState(null);
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [selectedFilm, setSelectedFilm] = useState(null);
  const [detailInitial, setDetailInitial] = useState({ editing: false, metaOpen: false });
  const [contextMenu, setContextMenu] = useState(null);
  const [confirmState, setConfirmState] = useState(null);
  const [confirmBusy, setConfirmBusy] = useState(false);
  const [confirmError, setConfirmError] = useState(null);
  const [addingFilm, setAddingFilm] = useState(false);
  const [ratingsOpen, setRatingsOpen] = useState(false);
  const [page, setPage] = useState(1);
  const [rows, setRows] = useState(loadRows);
  const [cols, setCols] = useState(1);
  const [viewMode, setViewMode] = useState(loadViewMode);
  const [listSize, setListSize] = useState(loadListSize);
  const gridRef = useRef(null);

  // 测量影片网格的列数（auto-fill 随视口变化）
  // 依赖说明：
  //  - viewMode：切回海报模式时重测
  //  - loading：影片加载完成、film-grid 实际挂载后才有 gridRef.current 可测。
  //    初次 mount 时 loading=true，网格未渲染、ref 为空会提前 return，cols 停在 1，
  //    导致 pageSize = rows*1（「2行」变「2条」）。loading 翻 false 后重跑此处即可修复。
  //    cols 一旦测得正确值即稳定（auto-fill 仅随视口变化，由 ResizeObserver 监听），
  //    改 rows 无需重测，pageSize = rows * cols 自然正确。
  useEffect(() => {
    if (viewMode !== 'grid') return;
    const el = gridRef.current;
    if (!el) return;
    const measure = () => {
      const tpl = getComputedStyle(el).gridTemplateColumns;
      const n = tpl ? tpl.split(' ').filter(Boolean).length : 0;
      setCols((c) => (n > 0 ? n : c));
    };
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, [viewMode, loading]);

  // 弹窗打开期间锁定背景滚动
  const modalOpen = Boolean(selectedFilm || addingFilm || contextMenu || confirmState || ratingsOpen);
  useEffect(() => {
    if (!modalOpen) return;
    const prev = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    return () => { document.body.style.overflow = prev; };
  }, [modalOpen]);

  const pageSize = viewMode === 'list' ? listSize : Math.max(1, rows * cols);

  // 切换显示模式：持久化并回到第一页
  const changeViewMode = (m) => {
    setViewMode(m);
    try { localStorage.setItem(VIEW_KEY, m); } catch {}
    setPage(1);
  };

  // 列表模式改变每页条数：持久化并回到第一页
  const changeListSize = (n) => {
    setListSize(n);
    try { localStorage.setItem(LIST_SIZE_KEY, String(n)); } catch {}
    setPage(1);
  };

  // 加载筛选项 & 统计（仅一次）
  useEffect(() => {
    fetchFilters().then(setFilterOpts).catch(setError);
    fetchStats().then(setStats).catch(() => {});
  }, []);

  // 按筛选加载影片列表
  const loadFilms = useCallback(() => {
    setLoading(true);
    setError(null);
    fetchFilms(filters)
      .then(setFilms)
      .catch(setError)
      .finally(() => setLoading(false));
  }, [filters]);

  useEffect(() => {
    const t = setTimeout(loadFilms, 150); // 简单防抖
    return () => clearTimeout(t);
  }, [loadFilms]);

  const activeFilterCount = useMemo(
    () => Object.values(filters).filter((v) => v !== '' && v != null).length,
    [filters]
  );

  // 筛选条件变化时回到第一页，并立即标记加载中避免显示旧数据闪烁
  const updateFilters = (next) => {
    setPage(1);
    setFilters(next);
    setLoading(true);
  };

  const resetFilters = () => {
    setPage(1);
    setFilters({ watchYear: '', releaseYear: '', platform: '', category: '', q: '' });
    setLoading(true);
  };

  // 改变每页行数：持久化并回到第一页
  const changeRows = (r) => {
    setRows(r);
    try { localStorage.setItem(ROWS_KEY, String(r)); } catch {}
    setPage(1);
  };

  // 右键菜单
  const handleCardContextMenu = (e, film) => {
    e.preventDefault();
    setContextMenu({ film, x: e.clientX, y: e.clientY });
  };

  const openDetail = (film, mode) => {
    setDetailInitial({
      editing: mode === 'edit',
      metaOpen: mode === 'scrape',
    });
    setSelectedFilm(film);
    setContextMenu(null);
  };

  // 危险操作二次确认
  const requestConfirm = (opts) => {
    setConfirmError(null);
    setConfirmState(opts);
  };

  const runConfirm = async () => {
    if (!confirmState?.action) return;
    setConfirmBusy(true);
    setConfirmError(null);
    try {
      await confirmState.action();
      setConfirmState(null);
    } catch (e) {
      setConfirmError(e.message);
    } finally {
      setConfirmBusy(false);
    }
  };

  const refreshAll = (filmId) => {
    loadFilms();
    fetchStats().then(setStats).catch(() => {});
    if (filmId != null && selectedFilm?.id === filmId) {
      fetchFilm(filmId).then(setSelectedFilm).catch(() => {});
    }
  };

  const handleDeleteMeta = (film) => {
    requestConfirm({
      title: '删除元数据',
      message: `确定删除「${film.name}」的元数据？本地图片也将一并删除。`,
      confirmText: '删除',
      danger: true,
      action: async () => {
        await deleteMeta(film.id);
        refreshAll(film.id);
      },
    });
  };

  const handleDeleteFilm = (film) => {
    requestConfirm({
      title: '删除观影记录',
      message: `确定删除观影记录「${film.name}」？\n该操作将同时删除其元数据与本地图片，且不可恢复。`,
      confirmText: '删除',
      danger: true,
      action: async () => {
        await deleteFilm(film.id);
        if (selectedFilm?.id === film.id) setSelectedFilm(null);
        refreshAll(film.id);
      },
    });
  };

  const buildContextMenuItems = (film) => {
    const items = [
      { type: 'item', label: '刮削元数据', icon: 'search', onClick: () => openDetail(film, 'scrape') },
      { type: 'item', label: '编辑', icon: 'edit', onClick: () => openDetail(film, 'edit') },
    ];
    if (film.hasMetadata) {
      items.push({ type: 'divider' });
      items.push({
        type: 'item', label: '删除元数据', icon: 'trash', danger: true,
        onClick: () => handleDeleteMeta(film),
      });
    }
    // items.push({ type: 'divider' });
    items.push({
      type: 'item', label: '删除记录', icon: 'trash', danger: true,
      onClick: () => handleDeleteFilm(film),
    });
    return items;
  };

  const pageCount = Math.ceil(films.length / pageSize);
  const effectivePage = Math.min(page, Math.max(1, pageCount));
  const pagedFilms = films.slice(
    (effectivePage - 1) * pageSize,
    effectivePage * pageSize
  );

  return (
    <div className="app">
      <header className="app-header">
        <h1><Icon name="film" size={26} /> 影视观看记录</h1>
        <div className="header-actions">
          <div className="header-stats">
            {stats && (
              <>
                <span>共 <b>{stats.total}</b> 部</span>
                <span>已刮削元数据 <b>{stats.withMetadata}</b>/{stats.total}</span>
              </>
            )}
          </div>
          <button className="btn-primary" onClick={() => setAddingFilm(true)}>
            <Icon name="plus" size={14} /> 新增观影记录
          </button>
        </div>
      </header>

      {stats && (
        <CategoryBreakdown
          stats={stats}
          active={filters.category}
          onSelect={(k) =>
            updateFilters((f) => ({ ...f, category: f.category === k ? '' : k }))
          }
        />
      )}

      <Filters
        value={filters}
        onChange={updateFilters}
        onReset={resetFilters}
        options={filterOpts}
        activeCount={activeFilterCount}
        onOpenRatings={() => setRatingsOpen(true)}
      />

      {error && <div className="error-banner"><Icon name="alert" size={16} /> {error.message}</div>}

      <div className="results-meta">
        <span>
          {loading
            ? '加载中…'
            : films.length === 0
              ? '无匹配记录'
              : `第 ${effectivePage}/${pageCount || 1} 页 · 每页 ${pageSize} 条 · 共 ${films.length} 条结果`}
        </span>
        <div className="view-toggle" role="group" aria-label="显示模式">
          <button
            type="button"
            className={viewMode === 'grid' ? 'active' : ''}
            onClick={() => changeViewMode('grid')}
            title="海报模式"
          >
            <Icon name="grid" size={15} />
          </button>
          <button
            type="button"
            className={viewMode === 'list' ? 'active' : ''}
            onClick={() => changeViewMode('list')}
            title="列表模式"
          >
            <Icon name="list" size={15} />
          </button>
        </div>
      </div>

      {loading ? (
        <div className="grid-loading">加载中…</div>
      ) : films.length === 0 ? (
        <div className="empty">无匹配记录</div>
      ) : viewMode === 'grid' ? (
        <div className="film-grid" ref={gridRef}>
          {pagedFilms.map((f) => (
            <FilmCard
              key={f.id}
              film={f}
              onClick={() => {
                setDetailInitial({ editing: false, metaOpen: false });
                setSelectedFilm(f);
              }}
              onContextMenu={(e) => handleCardContextMenu(e, f)}
            />
          ))}
        </div>
      ) : (
        <FilmList
          films={pagedFilms}
          onClick={(f) => {
            setDetailInitial({ editing: false, metaOpen: false });
            setSelectedFilm(f);
          }}
          onContextMenu={handleCardContextMenu}
        />
      )}

      <Paginator
        page={effectivePage}
        pageCount={pageCount}
        onChange={setPage}
        rows={rows}
        onRowsChange={changeRows}
        total={films.length}
        mode={viewMode}
        listSize={listSize}
        onListSizeChange={changeListSize}
      />

      {selectedFilm && (
        <FilmDetail
          film={selectedFilm}
          onClose={() => setSelectedFilm(null)}
          initialEditing={detailInitial.editing}
          initialMetaOpen={detailInitial.metaOpen}
          onChanged={() => {
            // 刷新列表中的该条记录与统计
            fetchFilm(selectedFilm.id).then(setSelectedFilm).catch(() => {});
            loadFilms();
            fetchStats().then(setStats).catch(() => {});
          }}
        />
      )}

      {addingFilm && (
        <AddFilmModal
          onClose={() => setAddingFilm(false)}
          onCreated={(film) => {
            setAddingFilm(false);
            loadFilms();
            fetchStats().then(setStats).catch(() => {});
            setSelectedFilm(film);
          }}
        />
      )}

      {contextMenu && (
        <ContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          items={buildContextMenuItems(contextMenu.film)}
          onClose={() => setContextMenu(null)}
        />
      )}

      <ConfirmDialog
        open={Boolean(confirmState)}
        title={confirmState?.title}
        message={confirmState?.message}
        confirmText={confirmState?.confirmText}
        danger={confirmState?.danger}
        busy={confirmBusy}
        error={confirmError}
        onConfirm={runConfirm}
        onCancel={() => { setConfirmState(null); setConfirmError(null); }}
      />

      {ratingsOpen && (
        <RatingManager
          films={films}
          filters={filters}
          onClose={() => {
            setRatingsOpen(false);
            // 关闭时刷新列表，保证下次打开与详情弹窗的豆瓣 ID 一致
            loadFilms();
            fetchStats().then(setStats).catch(() => {});
          }}
          onChanged={() => {
            loadFilms();
            fetchStats().then(setStats).catch(() => {});
          }}
        />
      )}
    </div>
  );
}

function AddFilmModal({ onClose, onCreated }) {
  const [form, setForm] = useState(emptyFilmForm());
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState(null);

  const save = async () => {
    if (!form.name?.trim()) {
      setErr('名称不能为空');
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      const film = await createFilm(filmFormToPatch(form));
      onCreated(film);
    } catch (e) {
      setErr(e.message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal add-film-modal" onClick={(e) => e.stopPropagation()}>
        <button className="modal-close" onClick={onClose} title="关闭"><Icon name="close" size={16} /></button>
        <h3><Icon name="plus" size={16} /> 新增观影记录</h3>
        {err && <div className="error-banner small"><Icon name="alert" size={14} /> {err}</div>}
        <FilmForm value={form} onChange={setForm} />
        <div className="detail-footer">
          <div className="meta-actions">
            <button className="btn-primary" disabled={busy} onClick={save}>
              <Icon name="save" size={14} /> {busy ? '保存中…' : '保存'}
            </button>
            <button className="btn-secondary" disabled={busy} onClick={onClose}>取消</button>
          </div>
        </div>
      </div>
    </div>
  );
}

function CategoryBreakdown({ stats, active, onSelect }) {
  const total = stats.total;
  const noMetaCount = stats.withoutMetadata ?? 0;
  const NO_META = '__no_meta__';
  return (
    <div className="cat-cards">
      <button
        type="button"
        className={`cat-card${active === '' ? ' active' : ''}`}
        onClick={() => onSelect('')}
        title={`全部: ${total} 部`}
      >
        <span className="cat-card-name">全部</span>
        <span className="cat-card-count">{total}</span>
      </button>
      {noMetaCount > 0 && (
        <button
          type="button"
          className={`cat-card cat-card-no-meta${active === NO_META ? ' active' : ''}`}
          onClick={() => onSelect(NO_META)}
          title={`无元数据: ${noMetaCount} 部`}
        >
          <span className="cat-card-name"><Icon name="alert" size={13} /> 无元数据</span>
          <span className="cat-card-count">{noMetaCount}</span>
        </button>
      )}
      {stats.byCategory.map((x) => (
        <button
          type="button"
          key={x.k}
          className={`cat-card${active === x.k ? ' active' : ''}`}
          onClick={() => onSelect(x.k)}
          title={`${x.k}: ${x.c} 部`}
        >
          <span className="cat-card-name">{x.k}</span>
          <span className="cat-card-count">{x.c}</span>
        </button>
      ))}
    </div>
  );
}
