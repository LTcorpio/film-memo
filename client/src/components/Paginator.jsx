import { useState } from 'react';
import Icon from './Icon.jsx';

/** 默认每页行数 */
export const DEFAULT_ROWS = 4;
/** 可选行数 */
export const ROW_OPTIONS = [2, 4, 6, 8];
/** 列表模式默认每页条数 */
export const DEFAULT_LIST_SIZE = 20;
/** 列表模式可选每页条数 */
export const LIST_SIZE_OPTIONS = [10, 20, 30, 50];

/**
 * 生成要显示的页码（带省略号）。
 * 规则：始终包含首末页；当前页附近显示连续 3 页（左中右各 1）。
 * 前段（1~4）和后段始终展示足够连续页码，避免省略号紧邻当前页导致误触末页。
 */
function pageList(current, total) {
  if (total <= 7) {
    return Array.from({ length: total }, (_, i) => i + 1);
  }
  const pages = [1];
  // 前段：当前页 ≤ 4 时，展示 2,3,4,5 再省略，避免当前页旁出现省略号
  if (current <= 4) {
    for (let i = 2; i <= 5; i++) pages.push(i);
    pages.push('…', total);
    return pages;
  }
  // 后段：当前页 ≥ total-3 时，省略后展示末尾 4 页
  if (current >= total - 3) {
    pages.push('…');
    for (let i = total - 4; i <= total; i++) pages.push(i);
    return pages;
  }
  // 中段：左省略、当前页前后各 1、右省略、末页
  pages.push('…', current - 1, current, current + 1, '…', total);
  return pages;
}

export default function Paginator({
  page,
  pageCount,
  onChange,
  rows,
  onRowsChange,
  total,
  mode = 'grid',
  listSize,
  onListSizeChange,
}) {
  const [jump, setJump] = useState('');
  const pages = pageList(page, pageCount);
  const prevDisabled = page <= 1;
  const nextDisabled = page >= pageCount;

  if (pageCount <= 1 && total === 0) return null;

  const doJump = () => {
    const n = parseInt(jump, 10);
    if (!Number.isFinite(n)) return;
    const target = Math.min(Math.max(1, n), Math.max(1, pageCount));
    onChange(target);
    setJump('');
  };

  return (
    <nav className="paginator" aria-label="分页">
      <div className="page-pages">
        <button
          type="button"
          className="page-btn icon-btn"
          disabled={prevDisabled}
          onClick={() => onChange(page - 1)}
          title="上一页"
        >
          <Icon name="chevron-left" size={16} />
        </button>
        {pages.map((p, i) =>
          p === '…' ? (
            <span key={`e${i}`} className="page-ellipsis">…</span>
          ) : (
            <button
              type="button"
              key={p}
              className={`page-btn${p === page ? ' active' : ''}`}
              onClick={() => onChange(p)}
            >
              {p}
            </button>
          )
        )}
        <button
          type="button"
          className="page-btn icon-btn"
          disabled={nextDisabled}
          onClick={() => onChange(page + 1)}
          title="下一页"
        >
          <Icon name="chevron-right" size={16} />
        </button>
      </div>

      <div className="page-tools">
        {mode === 'list' ? (
          <label className="page-size">
            <select value={listSize} onChange={(e) => onListSizeChange(Number(e.target.value))}>
              {LIST_SIZE_OPTIONS.map((n) => (
                <option key={n} value={n}>{n}</option>
              ))}
            </select>
            条/页
          </label>
        ) : (
          <label className="page-size">
            每页
            <select value={rows} onChange={(e) => onRowsChange(Number(e.target.value))}>
              {ROW_OPTIONS.map((r) => (
                <option key={r} value={r}>{r} 行</option>
              ))}
            </select>
          </label>
        )}
        <label className="page-jump">
          跳至
          <input
            type="number"
            min="1"
            max={Math.max(1, pageCount)}
            value={jump}
            placeholder={page}
            onChange={(e) => setJump(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && doJump()}
          />
          页
          <button type="button" className="page-jump-btn" onClick={doJump}>跳转</button>
        </label>
        <span className="page-total">共 {total} 条 / {Math.max(1, pageCount)} 页</span>
      </div>
    </nav>
  );
}
