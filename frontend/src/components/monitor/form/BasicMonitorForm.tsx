import type { MonitorFormState } from '@/lib/monitorForm'

interface BasicMonitorFormProps {
  value: MonitorFormState['basic']
  advanced?: boolean
  /** 可选分类列表（来自设置中的分类管理） */
  groups?: string[]
  /** 是否正在匹配规则（按钮显示「匹配中...」并禁用） */
  matching?: boolean
  /** 点击「匹配规则」按钮 */
  onMatchRule?: () => void
  onChange: (value: MonitorFormState['basic']) => void
}

export default function BasicMonitorForm({
  value,
  advanced = false,
  groups = [],
  matching = false,
  onMatchRule,
  onChange,
}: BasicMonitorFormProps) {
  function update<K extends keyof MonitorFormState['basic']>(key: K, v: MonitorFormState['basic'][K]) {
    onChange({ ...value, [key]: v })
  }

  if (advanced) {
    // 分类下拉：未分组时视为「默认」；编辑模式存在分类里没有的分组时补一个回退项
    const options: string[] = []
    for (const g of groups) {
      if (g.trim() && !options.includes(g)) options.push(g.trim())
    }
    const current = value.group.trim() || '默认'
    if (current && !options.includes(current)) options.push(current)

    return (
      <div className="advanced-panel-section basic-monitor-form">
        <h3>检查频率与分组</h3>
        <p className="advanced-section-desc">默认每小时检查一次，通常无需修改。分组可在「设置 → 分类管理」中维护。</p>
        <div className="form-group">
          <label>分类</label>
          <select value={current} onChange={(e) => update('group', e.target.value)} className="form-input">
            {options.map((g) => (
              <option key={g} value={g}>
                {g}
              </option>
            ))}
          </select>
        </div>
        <div className="form-group">
          <label>检查间隔（秒）</label>
          <input
            value={value.interval}
            onChange={(e) => update('interval', e.target.value)}
            className="form-input"
            type="number"
            min={10}
            placeholder="3600（默认1小时）"
          />
        </div>
      </div>
    )
  }

  return (
    <div className="settings-section basic-monitor-form">
      <div className="section-header">
        <h2>要监控哪个网页？</h2>
        <p className="section-desc">粘贴网址，点击「匹配规则」按已保存的规则识别需要关注的内容。</p>
      </div>
      <div className="form-group">
        <label>网页网址</label>
        <input
          value={value.url}
          onChange={(e) => update('url', e.target.value)}
          className="form-input primary-url"
          inputMode="url"
          autoFocus
          placeholder="https://example.com/notice"
        />
      </div>
      <div className="match-rule-row">
        <button
          className="btn btn-primary"
          type="button"
          disabled={matching || !value.url.trim()}
          onClick={onMatchRule}
        >
          {matching ? '匹配中...' : '匹配规则'}
        </button>
      </div>
    </div>
  )
}