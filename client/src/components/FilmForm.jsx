import Icon from './Icon.jsx';
import { DateRangePicker } from './DatePicker.jsx';

export const CATEGORY_OPTS = ['电影', '电视剧', '网剧', '综艺', '动漫', '纪录片', '短剧'];

/** 类别对应的集数单位：电影无集数（返回 null），综艺用「期」，其余用「集」 */
export function episodeUnit(category) {
  if (category === '电影') return null;
  if (category === '综艺') return '期';
  return '集';
}

/** 年份输入：仅接受最多四位的合法年份（禁止六位等非法年份） */
function YearInput({ value, onChange, disabled }) {
  return (
    <input
      type="number"
      min={1000}
      max={9999}
      disabled={disabled}
      value={value ?? ''}
      onChange={(e) => {
        const digits = e.target.value.replace(/\D/g, '').slice(0, 4);
        onChange(digits ? Number(digits) : null);
      }}
    />
  );
}

/** 观看状态备选项：value 与后端 viewing.watch_status 取值一一对应。
    数组顺序即点击循环顺序；icon 复用 Icon.jsx 里既有的两个图标
    （clock = 时长 / check = 对勾），不新增素材 */
const STATUS_OPTS = [
  { value: 'watching', label: '正在观看', icon: 'clock' },
  { value: 'finished', label: '已看完', icon: 'check' },
];

/**
 * 观看状态开关（单枚按钮，点击在两个状态之间循环切换）。
 * 按钮上显示的是「当前状态」（图标 + 文字），而不是「点击后的结果」：
 * 处于 watching 时整枚按钮（边框 + 文字 + 图标）转强调色，与列表里那条
 * 「正在观看」标记（.row-watching）同色，两处语义一眼对应；finished 时是普通描边色。
 * 单枚按钮的信息量低于原来的单选组，故状态由「图标 + 文字 + 颜色」三重表达，
 * 并由 title 补出「点击后变成什么」，弥补「看不出下一态」这个短处。
 * 几何沿用上一版单选按钮：38px 高（全局 border-box，含自身 1px 边框）= .edit-form 输入框等高。
 * a11y：按钮文字本身就是当前状态名，故用 aria-pressed 表达「是否处于 watching」；
 * 不用 role="switch" —— 那要求按钮名是开关的名字而非状态名，与本控件的文案冲突。
 * 图标取 14px：stroke 图标在小尺寸下视觉偏轻，比 13px 的汉字大一档才等重 ——
 * 同 .viewing-removed-hint（12px 字配 13px 图标）的取向，不是「图标尺寸 = 字号」那一条
 * （那条适用于标题处的大字号图标）。
 * 纯 CSS 实现，无测量逻辑（与已被替换掉的「分段切换器」不同，这里没有滑块要定位）。
 */
function StatusToggle({ value, disabled, onChange }) {
  const cur = STATUS_OPTS.find((o) => o.value === value) ?? STATUS_OPTS[STATUS_OPTS.length - 1];
  const next = STATUS_OPTS[(STATUS_OPTS.indexOf(cur) + 1) % STATUS_OPTS.length];
  const isWatching = cur.value === 'watching';
  return (
    <button
      type="button"
      className={`status-toggle${isWatching ? ' on' : ''}`}
      aria-pressed={isWatching}
      disabled={disabled}
      title={`当前：${cur.label}；点击切换为${next.label}`}
      onClick={() => onChange(next.value)}
    >
      <Icon name={cur.icon} size={14} />
      {cur.label}
    </button>
  );
}

/**
 * 影视 + 观看记录编辑表单（可在新增/编辑复用）。
 * 分为两组：
 *  - 元信息（影视级）：名称、类别、观看年份、上映年份、总集数、制片国家、IMDb、豆瓣 ID
 *  - 观看记录（观看级）：观看日期（开始–结束合并为一个范围选择）、观看平台、观看地点、备注
 * viewingOptions 的条目支持 isNew / pendingRemove 标记，
 * 用于展示「暂存待保存」的新增/移除状态（点击保存后才真正提交）。
 */
export default function FilmForm({
  value, onChange, viewingOptions, onViewingSelect,
  onAddViewing, onRemoveViewing, viewingActionsDisabled,
  viewingRemoved = false, pendingCount,
}) {
  const set = (k, v) => onChange({ ...value, [k]: v });
  const pendingParts = [];
  if (pendingCount?.adds > 0) pendingParts.push(`新增 ${pendingCount.adds} 条`);
  if (pendingCount?.removes > 0) pendingParts.push(`移除 ${pendingCount.removes} 条`);
  return (
    <div className="edit-form">
      <div className="form-row">
        <label>名称
          <input value={value.name || ''} onChange={(e) => set('name', e.target.value)} />
        </label>
        <label>类别
          <select value={value.category || ''} onChange={(e) => set('category', e.target.value)}>
            <option value="">—</option>
            {CATEGORY_OPTS.map((c) => <option key={c} value={c}>{c}</option>)}
          </select>
        </label>
      </div>
      <div className="form-row">
        <label>上映年份
          <YearInput value={value.releaseYear} onChange={(v) => set('releaseYear', v)} />
        </label>
        {episodeUnit(value.category) && (
          <label>总{episodeUnit(value.category)}数
            <input type="number" value={value.totalEpisodes ?? ''} onChange={(e) => set('totalEpisodes', e.target.value ? Number(e.target.value) : null)} />
          </label>
        )}
        <label>制片国家（按 / 分割）
          <input value={value.productionCountriesRaw || ''} onChange={(e) => set('productionCountriesRaw', e.target.value)} />
        </label>
      </div>
      <div className="form-row">
        <label>IMDb 号
          <input value={value.imdbId || ''} onChange={(e) => set('imdbId', e.target.value || null)} />
        </label>
        <label>豆瓣 ID
          <input value={value.doubanId || ''} placeholder="如 1292052" onChange={(e) => set('doubanId', e.target.value)} />
        </label>
      </div>

      <h3 className="edit-section-title"><Icon name="clock" size={16} /> 观看记录</h3>
      {viewingOptions && onViewingSelect && (
        <div className="viewing-tabs">
          <div className="viewing-tab-list" role="tablist" aria-label="选择要编辑的观看记录">
            {viewingOptions.map((o, i) => {
              // 激活中的选项卡跟随表单实时数据，编辑未保存也能立即反馈
              const year = o.id === value.viewingId ? value.watchYear : o.watchYear;
              const cls = [
                'viewing-tab',
                o.id === value.viewingId ? 'active' : '',
                o.isNew ? 'pending-add' : '',
                o.pendingRemove ? 'pending-remove' : '',
              ].filter(Boolean).join(' ');
              return (
                <button
                  type="button"
                  role="tab"
                  key={o.id}
                  className={cls}
                  aria-selected={o.id === value.viewingId}
                  title={o.isNew ? '新增记录，保存后生效' : o.pendingRemove ? '已标记移除，保存后生效' : undefined}
                  onClick={() => { if (o.id !== value.viewingId) onViewingSelect(o.id); }}
                >
                  {`第 ${i + 1} 次${year ? ` · ${year}` : ''}`}
                </button>
              );
            })}
          </div>
          <div className="viewing-tab-actions">
            {onAddViewing && (
              <button
                type="button"
                className="btn-secondary small"
                disabled={viewingActionsDisabled}
                onClick={onAddViewing}
                title="为该影视新增一条观看记录（保存后生效）"
              >
                <Icon name="plus" size={12} /> 新增
              </button>
            )}
            {onRemoveViewing && (
              <button
                type="button"
                className="btn-danger small"
                disabled={viewingActionsDisabled || viewingRemoved}
                onClick={onRemoveViewing}
                title={viewingRemoved ? '该记录已标记移除' : '移除当前观看记录（保存后生效）'}
              >
                <Icon name="trash" size={12} /> 移除
              </button>
            )}
          </div>
          {pendingParts.length > 0 && (
            <div className="viewing-pending-note">
              <Icon name="alert" size={12} /> 待保存变更：{pendingParts.join('，')}，点击「保存」后生效
            </div>
          )}
        </div>
      )}
      {viewingRemoved && (
        <div className="viewing-removed-hint">
          <Icon name="alert" size={13} /> 该观看记录已标记为移除，点击「保存」后生效
        </div>
      )}
      {/* 这格用 div 而非 label：label 对 button 而言是 labelable 容器，
          点标题「观看状态」四个字会顺带触发里面的按钮（单枚按钮形态下＝直接切状态）。
          样式由 .edit-form .field 与 label 等价承担。 */}
      <div className="form-row">
        <div className="field">观看状态
          <StatusToggle
            value={value.watchStatus}
            disabled={viewingRemoved}
            onChange={(v) => set('watchStatus', v)}
          />
        </div>
      </div>
      <div className="form-row">
        <label>观看年份
          <YearInput value={value.watchYear} onChange={(v) => set('watchYear', v)} disabled={viewingRemoved} />
        </label>
        <label>观看日期
          <DateRangePicker
            from={value.startDate}
            to={value.endDate}
            disabled={viewingRemoved}
            title="开始观看日期 – 结束观看日期（选单日表示当天看完）"
            onChange={(from, to) => onChange({ ...value, startDate: from, endDate: to })}
          />
        </label>
      </div>
      <label>观看平台（逗号分隔）
        <input disabled={viewingRemoved} value={value.platformsRaw || ''} placeholder="爱奇艺,腾讯视频" onChange={(e) => set('platformsRaw', e.target.value)} />
      </label>
      <label>观看地点
        <input disabled={viewingRemoved} value={value.location || ''} onChange={(e) => set('location', e.target.value)} />
      </label>
      <label>备注
        <textarea rows={2} disabled={viewingRemoved} value={value.notes || ''} onChange={(e) => set('notes', e.target.value)} />
      </label>
    </div>
  );
}

/** 默认空表单（用于新增） */
export function emptyFilmForm() {
  return {
    viewingId: null,
    name: '',
    category: '',
    watchYear: null,
    releaseYear: null,
    totalEpisodes: null,
    productionCountriesRaw: '',
    imdbId: '',
    doubanId: '',
    startDate: null,
    endDate: null,
    watchStatus: 'finished',
    platformsRaw: '',
    location: '',
    notes: '',
  };
}

/** 前端表单 → 后端字段名（snake_case）：拆分为影视级 / 观看级两组 */
export function filmFormToPatches(f) {
  return {
    film: {
      name: f.name,
      category: f.category || null,
      release_year: f.releaseYear ?? null,
      total_episodes: f.totalEpisodes ?? null,
      production_countries_raw: f.productionCountriesRaw || null,
      imdb_id: f.imdbId || null,
      douban_id: f.doubanId?.trim() || null,
    },
    viewing: {
      watch_year: f.watchYear ?? null,
      start_date: f.startDate || null,
      end_date: f.endDate || null,
      watch_status: f.watchStatus === 'watching' ? 'watching' : 'finished',
      platforms_raw: f.platformsRaw || null,
      location: f.location || null,
      notes: f.notes || null,
    },
  };
}

/** 影视数据 + 观看记录 → 表单数据 */
export function filmToForm(film, viewing) {
  return {
    viewingId: viewing?.id ?? null,
    name: film.name,
    category: film.category,
    watchYear: viewing?.watchYear ?? null,
    releaseYear: film.releaseYear,
    totalEpisodes: film.totalEpisodes,
    productionCountriesRaw: film.productionCountriesRaw || '',
    imdbId: film.imdbId,
    doubanId: film.doubanId,
    startDate: viewing?.startDate ?? null,
    endDate: viewing?.endDate ?? null,
    // 缺失（旧数据 / 列表条目未拉取详情）一律按「已看完」处理
    watchStatus: viewing?.watchStatus === 'watching' ? 'watching' : 'finished',
    platformsRaw: (viewing?.platforms || []).join(','),
    location: viewing?.location ?? null,
    notes: viewing?.notes ?? null,
  };
}
