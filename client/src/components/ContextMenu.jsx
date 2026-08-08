import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import Icon from './Icon.jsx';

/**
 * 右键上下文菜单
 * items: Array<
 *   { type: 'item', label, icon?, onClick, danger?, disabled? } |
 *   { type: 'divider' }
 * >
 */
export default function ContextMenu({ x, y, items, onClose }) {
  const ref = useRef(null);
  const [pos, setPos] = useState({ x, y });

  // 智能定位：避免溢出视口
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    let nx = x;
    let ny = y;
    if (x + rect.width > window.innerWidth - 8) {
      nx = window.innerWidth - rect.width - 8;
    }
    if (y + rect.height > window.innerHeight - 8) {
      ny = window.innerHeight - rect.height - 8;
    }
    if (nx < 8) nx = 8;
    if (ny < 8) ny = 8;
    setPos({ x: nx, y: ny });
  }, [x, y]);

  // 关闭：外部点击 / ESC / 滚动 / 窗口尺寸变化
  useEffect(() => {
    const onDown = (e) => {
      if (ref.current && !ref.current.contains(e.target)) onClose();
    };
    const onKey = (e) => {
      if (e.key === 'Escape') onClose();
    };
    const onScroll = () => onClose();
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    window.addEventListener('scroll', onScroll, true);
    window.addEventListener('resize', onScroll);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
      window.removeEventListener('scroll', onScroll, true);
      window.removeEventListener('resize', onScroll);
    };
  }, [onClose]);

  return (
    <div className="ctx-menu" ref={ref} style={{ left: pos.x, top: pos.y }}>
      {items.map((it, i) => {
        if (it.type === 'divider') {
          return <div key={i} className="ctx-divider" />;
        }
        return (
          <button
            key={i}
            type="button"
            className={`ctx-item${it.danger ? ' danger' : ''}`}
            disabled={it.disabled}
            onClick={() => { it.onClick(); onClose(); }}
          >
            {it.icon && <Icon name={it.icon} size={14} />}
            <span>{it.label}</span>
          </button>
        );
      })}
    </div>
  );
}
