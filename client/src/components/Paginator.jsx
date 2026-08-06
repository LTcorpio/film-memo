import { useState } from 'react';
import Icon from './Icon.jsx';

/** 默认每页行数 */
export const DEFAULT_ROWS = 4;
/** 可选行数 */
export const ROW_OPTIONS = [2, 4, 6, 8];

/**
 * 生成要显示的页码（带省略号）。
 * 规则：始终包含首末页，当前页左右各显示 siblings 个页码。
 */
function pageList(current, total, siblings = 1) {
  if (total <= 7) {
    return Array.from({ length: total }, (_, i) => i + 1);
  }
  const pages = [];
  const left = Math.max(2, current - siblings);
  const right = Math.min(total - 1, current + siblings);
  pages.push(1);
  if (left > 2) pages.push('…');
  for (let i = left; i <= right; i++) pages.push(i);
  if (right < total - 1) pages.push('…');
  pages.push(total);
  return pages;
}

export default function Paginator({
  page,
  pageCount,
  onChange,
  rows,
  onRowsChange,
  total,
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
        <label className="page-size">
          每页
          <select value={rows} onChange={(e) => onRowsChange(Number(e.target.value))}>
            {ROW_OPTIONS.map((r) => (
              <option key={r} value={r}>{r} 行</option>
            ))}
          </select>
        </label>
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
          <button type="button" className="page-jump-btn" onClick={doJump}>Go</button>
        </label>
        <span className="page-total">共 {total} 条 / {Math.max(1, pageCount)} 页</span>
      </div>
    </nav>
  );
}
