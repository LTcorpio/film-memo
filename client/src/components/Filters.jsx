export default function Filters({ value, onChange, onReset, options, activeCount }) {
  const set = (k, v) => onChange({ ...value, [k]: v });

  return (
    <div className="filters">
      <div className="filters-row">
        <label>
          观看年份
          <select value={value.watchYear} onChange={(e) => set('watchYear', e.target.value)}>
            <option value="">全部</option>
            {(options?.watchYears || []).map((y) => (
              <option key={y} value={y}>{y}</option>
            ))}
          </select>
        </label>

        <label>
          上映年份
          <select value={value.releaseYear} onChange={(e) => set('releaseYear', e.target.value)}>
            <option value="">全部</option>
            {(options?.releaseYears || []).map((y) => (
              <option key={y} value={y}>{y}</option>
            ))}
          </select>
        </label>

        <label>
          观看平台
          <select value={value.platform} onChange={(e) => set('platform', e.target.value)}>
            <option value="">全部</option>
            {(options?.platforms || []).map((p) => (
              <option key={p} value={p}>{p}</option>
            ))}
          </select>
        </label>

        <label className="search">
          搜索名称
          <input
            type="search"
            value={value.q}
            placeholder="输入影视名称…"
            onChange={(e) => set('q', e.target.value)}
          />
        </label>

        {activeCount > 0 && (
          <button className="btn-reset" onClick={onReset}>
            清除筛选 ({activeCount})
          </button>
        )}
      </div>
    </div>
  );
}
