import type { MonitorFormState } from '@/lib/monitorForm'
import './BasicMonitorForm.css'

interface BasicMonitorFormProps {
  value: MonitorFormState['basic']
  advanced?: boolean
  onChange: (value: MonitorFormState['basic']) => void
}

export default function BasicMonitorForm({ value, advanced = false, onChange }: BasicMonitorFormProps) {
  function update<K extends keyof MonitorFormState['basic']>(key: K, v: MonitorFormState['basic'][K]) {
    onChange({ ...value, [key]: v })
  }

  if (advanced) {
    return (
      <div className="advanced-panel-section basic-monitor-form">
        <h3>检查频率与分组</h3>
        <p className="advanced-section-desc">默认每小时检查一次，通常无需修改。</p>
        <div className="form-group">
          <label>分组</label>
          <input value={value.group} onChange={(e) => update('group', e.target.value)} className="form-input" placeholder="默认" />
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
        <p className="section-desc">粘贴网址，系统会按已保存的扫描规则识别需要关注的内容。名称可留空。</p>
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
      <div className="form-group optional-name">
        <label>
          监控名称 <span>可选</span>
        </label>
        <input
          value={value.name}
          onChange={(e) => update('name', e.target.value)}
          className="form-input"
          placeholder="留空将自动使用网站名称"
        />
      </div>
    </div>
  )
}
