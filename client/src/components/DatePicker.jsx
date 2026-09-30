import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { DayPicker } from 'react-day-picker';
import { zhCN } from 'react-day-picker/locale/zh-CN';
import 'react-day-picker/style.css';
import Icon from './Icon.jsx';

/**
 * 表单日期选择控件（单日期 + 日期范围两种形态，共用同一套面板）。
 *
 * 收起态是一枚按钮（显示当前日期/日期范围，空值弱化显示），展开为
 * 「快捷选项 + 日历」面板；日历网格、区间选中态与键盘可达性由
 * react-day-picker 渲染（年月标题为下拉式，可快速跳到任意年月）。
 * 交互口径参考 transit-memo 的日期范围筛选器：点击开合、Esc/点击外部收起、
 * 面板空间不足时贴另一侧展开；本控件位于弹窗内，展开方向与单/双月按所在
 * 弹窗（.modal）的可用宽度自适应，避免被弹窗裁剪。配色尺寸见 styles.css 的
 * .fm-date-* 与 .rdp-root 覆盖（随应用亮/暗主题变量自适应）。
 *
 * 日期一律为本地时区 'YYYY-MM-DD' 字符串（与后端 start_date/end_date 口径
 * 一致，不受 UTC 影响），空值为 null：
 *   - <DatePicker>      单日期：value + onChange(value)
 *   - <DateRangePicker> 日期范围：from/to + onChange(from, to)；单日 = 起止同一天
 */

const pad = (n) => String(n).padStart(2, '0');

/** Date → 'YYYY-MM-DD'（本地时区；空值返回 null） */
const toISO = (d) => (d ? `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` : null);

/** 'YYYY-MM-DD' → Date（本地时区；非法/空返回 undefined） */
const toDate = (s) => {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s || '');
  return m ? new Date(+m[1], +m[2] - 1, +m[3]) : undefined;
};

const addDays = (d, n) => {
  const x = new Date(d);
  x.setDate(x.getDate() + n);
  return x;
};

/** 按钮上的日期文案：同年省略年份，跨年自动补上 */
const fmt = (s) => {
  const d = toDate(s);
  if (!d) return '';
  const y = d.getFullYear() === new Date().getFullYear() ? '' : `${d.getFullYear()}年`;
  return `${y}${d.getMonth() + 1}月${d.getDate()}日`;
};

// 日历可跳转的年月范围：覆盖从早期电影到未来的日期录入
const CAL_START = new Date(1800, 0, 1);
const CAL_END = new Date(2100, 11, 31);

// 容器（弹窗）可用宽度下限：低于此值收起为单月面板，容纳不下双月
const CONTAINER_MIN_WIDTH = 700;

/** 单日期与范围共用的日历面板控件（按钮 + 下拉面板） */
function CalendarField({ mode, from, to, onChange, disabled, emptyText, title, presets }) {
  const [open, setOpen] = useState(false);
  const [flip, setFlip] = useState(false);
  const [month, setMonth] = useState(undefined);
  // 容器过窄时收起为单月面板（范围模式默认双月）
  const [tight, setTight] = useState(false);
  const ref = useRef(null);
  const popRef = useRef(null);

  // 可用空间测量：以所在弹窗（.modal）为边界，独立使用（无弹窗）时回退视口
  const measure = () => {
    const anchor = ref.current;
    if (!anchor) return null;
    const modalEl = anchor.closest('.modal');
    return {
      rect: anchor.getBoundingClientRect(),
      bounds: modalEl ? modalEl.getBoundingClientRect() : { left: 0, right: window.innerWidth },
    };
  };

  // 面板较宽: 展开侧空间不足时改为贴另一侧展开，避免溢出容器
  useLayoutEffect(() => {
    if (!open) return;
    const pop = popRef.current;
    const m = measure();
    if (!pop || !m) return;
    const { rect, bounds } = m;
    const spaceRight = bounds.right - rect.left - 8;
    const spaceLeft = rect.right - bounds.left - 8;
    setFlip(pop.offsetWidth > spaceRight && spaceLeft > spaceRight);
  }, [open, tight]);

  // 点击外部或按 Esc 收起面板
  // Esc 仅关闭本下拉: 阻止继续冒泡到 window，避免连带触发编辑弹窗的关闭确认
  useEffect(() => {
    if (!open) return undefined;
    const onDoc = (e) => {
      if (ref.current && !ref.current.contains(e.target)) setOpen(false);
    };
    const onKey = (e) => {
      if (e.key !== 'Escape') return;
      setOpen(false);
      e.stopPropagation();
    };
    document.addEventListener('mousedown', onDoc);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDoc);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  // 展开时测量容器宽度（决定单/双月）并把日历定位到当前值所在月，否则回到今天
  const toggle = () => {
    if (!open) {
      const m = measure();
      const w = m ? m.bounds.right - m.bounds.left : window.innerWidth;
      setTight(w < CONTAINER_MIN_WIDTH);
      setMonth(toDate(from) || toDate(to) || new Date());
    }
    setOpen(!open);
  };

  // 快捷选项: 应用到表单，并把日历同步到该日期所在月
  const applyPreset = (p) => {
    onChange(p.from, mode === 'range' ? p.to : null);
    setMonth(toDate(p.from) || new Date());
  };

  const activeKey = presets.find((p) => (
    mode === 'range' ? p.from === from && p.to === to : p.from === from
  ))?.key;

  const label = mode === 'range'
    ? (from && to
      ? (from === to ? fmt(from) : `${fmt(from)} – ${fmt(to)}`)
      : from ? `${fmt(from)} 起` : to ? `至 ${fmt(to)}` : emptyText)
    : (fmt(from) || emptyText);

  const hasValue = mode === 'range' ? Boolean(from || to) : Boolean(from);

  return (
    <span className="fm-date" ref={ref}>
      <button
        type="button"
        className={`fm-date-btn${hasValue ? ' on' : ''}`}
        disabled={disabled}
        onClick={toggle}
        aria-expanded={open}
        aria-haspopup="dialog"
        title={title}
      >
        <Icon name="calendar" size={14} className="fm-date-ic" />
        <span className="fm-date-text">{label}</span>
        {/* 展开/收起箭头：旋转 180° 表示展开态 */}
        <Icon name="chevron" size={14} className="fm-date-chev" />
      </button>

      {open && (
        <div
          className={`fm-date-pop${flip ? ' flip' : ''}`}
          ref={popRef}
          role="dialog"
          aria-label="选择日期"
        >
          {presets.length > 0 && (
            <div className="fm-date-presets">
              {presets.map((p) => (
                <button
                  key={p.key}
                  type="button"
                  className={`fm-date-preset${activeKey === p.key ? ' on' : ''}`}
                  onClick={() => applyPreset(p)}
                >
                  {p.label}
                </button>
              ))}
            </div>
          )}
          {mode === 'range' ? (
            <DayPicker
              mode="range"
              numberOfMonths={tight ? 1 : 2}
              locale={zhCN}
              captionLayout="dropdown"
              startMonth={CAL_START}
              endMonth={CAL_END}
              selected={{ from: toDate(from), to: toDate(to) }}
              onSelect={(r) => onChange(toISO(r?.from), toISO(r?.to))}
              month={month}
              onMonthChange={setMonth}
            />
          ) : (
            <DayPicker
              mode="single"
              locale={zhCN}
              captionLayout="dropdown"
              startMonth={CAL_START}
              endMonth={CAL_END}
              selected={toDate(from)}
              onSelect={(d) => onChange(toISO(d), null)}
              month={month}
              onMonthChange={setMonth}
            />
          )}
        </div>
      )}
    </span>
  );
}

/** 单日期选择（适用于上映日期等单值日期字段） */
export function DatePicker({ value, onChange, disabled, title, emptyText = '未选择' }) {
  return (
    <CalendarField
      mode="single"
      from={value ?? null}
      to={null}
      onChange={(v) => onChange?.(v)}
      disabled={disabled}
      emptyText={emptyText}
      title={title}
      presets={[{ key: 'clear', label: '清除', from: null, to: null }]}
    />
  );
}

/** 日期范围选择：开始/结束合并为一个控件（单日 = 起止同一天，一键直选） */
export function DateRangePicker({ from, to, onChange, disabled, title, emptyText = '未选择' }) {
  const today = toISO(new Date());
  const yesterday = toISO(addDays(new Date(), -1));
  return (
    <CalendarField
      mode="range"
      from={from ?? null}
      to={to ?? null}
      onChange={(f, t) => onChange?.(f, t)}
      disabled={disabled}
      emptyText={emptyText}
      title={title}
      presets={[
        { key: 'today', label: '今天', from: today, to: today },
        { key: 'yesterday', label: '昨天', from: yesterday, to: yesterday },
        { key: 'clear', label: '清除', from: null, to: null },
      ]}
    />
  );
}