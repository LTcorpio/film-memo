import Icon from './Icon.jsx';

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

/** 日期输入：限定四位年份（min/max），忽略超出四位年份的非法输入 */
export function DateInput({ value, onChange, disabled }) {
  return (
    <input
      type="date"
      min="1000-01-01"
      max="9999-12-31"
      disabled={disabled}
      value={value || ''}
      onChange={(e) => {
        const v = e.target.value;
        if (v === '' || /^\d{4}-\d{2}-\d{2}$/.test(v)) onChange(v || null);
      }}
    />
  );
}

/**
 * 影视 + 观看记录编辑表单（可在新增/编辑复用）。
 * 分为两组：
 *  - 元信息（影视级）：名称、类别、观看年份、上映年份、总集数、制片国家、IMDb、豆瓣 ID
 *  - 观看记录（观看级）：开始/结束观看日期、观看平台、观看地点、备注
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
      <div className="form-row">
        <label>观看年份
          <YearInput value={value.watchYear} onChange={(v) => set('watchYear', v)} disabled={viewingRemoved} />
        </label>
        <label>开始观看日期
          <DateInput value={value.startDate} onChange={(v) => set('startDate', v)} disabled={viewingRemoved} />
        </label>
        <label>结束观看日期
          <DateInput value={value.endDate} onChange={(v) => set('endDate', v)} disabled={viewingRemoved} />
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
    platformsRaw: (viewing?.platforms || []).join(','),
    location: viewing?.location ?? null,
    notes: viewing?.notes ?? null,
  };
}
