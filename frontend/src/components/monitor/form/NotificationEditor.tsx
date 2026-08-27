import type { MonitorFormState } from '@/lib/monitorForm'
import type { NotifyAccount } from '@/api/types'

interface NotificationEditorProps {
  value: MonitorFormState['notification']
  accounts?: NotifyAccount[]
  onChange: (value: MonitorFormState['notification']) => void
}

function serviceLabel(s: string) {
  if (s === 'pushplus') return 'PushPlus'
  if (s === 'webhook') return 'Webhook'
  if (s === 'serverchan') return 'Server酱'
  if (s === 'bark') return 'Bark'
  return s
}

export default function NotificationEditor({ value, accounts = [], onChange }: NotificationEditorProps) {
  function update<K extends keyof MonitorFormState['notification']>(key: K, v: MonitorFormState['notification'][K]) {
    onChange({ ...value, [key]: v })
  }

  function toggleAccount(id: number | string) {
    const ids = [...value.accountIds]
    const idx = ids.indexOf(id)
    if (idx >= 0) {
      ids.splice(idx, 1)
    } else {
      ids.push(id)
    }
    onChange({ ...value, accountIds: ids })
  }

  return (
    <div className="settings-section notification-editor">
      <div className="section-header">
        <h2>通知配置</h2>
      </div>

      <div className="form-group">
        <label>推送过滤</label>
        <div className="filter-mode-row">
          <label className={`radio-label${value.filter === 'all' ? ' active' : ''}`}>
            <input type="radio" checked={value.filter === 'all'} onChange={() => update('filter', 'all')} />
            有新内容就推送
          </label>
          <label className={`radio-label${value.filter === 'keyword' ? ' active' : ''}`}>
            <input type="radio" checked={value.filter === 'keyword'} onChange={() => update('filter', 'keyword')} />
            仅命中关键词时推送
          </label>
        </div>
        {value.filter === 'keyword' && (
          <div className="form-group mt-2">
            <label>推送关键词（多个用逗号隔开）</label>
            <input
              value={value.keywords}
              onChange={(e) => update('keywords', e.target.value)}
              className="form-input"
              placeholder="面试,录用,公示"
            />
          </div>
        )}
      </div>

      <div className="form-group">
        <label>推送账户</label>
        {accounts.length > 0 ? (
          <div className="accounts-grid">
            {accounts.map((acc) => (
              <label key={acc.id} className="account-checkbox">
                <input type="checkbox" value={acc.id} checked={value.accountIds.includes(acc.id)} onChange={() => toggleAccount(acc.id)} />
                <span className="acc-name">{acc.name}</span>
                <span className={`service-tag badge-${acc.service}`}>{serviceLabel(acc.service)}</span>
              </label>
            ))}
          </div>
        ) : (
          <p className="hint">暂无推送账户，请先在「推送通知」中添加</p>
        )}
      </div>
    </div>
  )
}
