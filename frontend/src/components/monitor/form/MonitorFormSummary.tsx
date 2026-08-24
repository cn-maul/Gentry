import type { MonitorFormState } from '@/lib/monitorForm'
import './MonitorFormSummary.css'

function formatInterval(s: number) {
  if (!s) return '—'
  if (s >= 3600) return `${Math.round(s / 3600)} 小时`
  if (s >= 60) return `${Math.round(s / 60)} 分钟`
  return `${s} 秒`
}

export default function MonitorFormSummary({ form }: { form: MonitorFormState }) {
  const fieldNames = form.extraction.fields.map((f) => f.name).filter(Boolean).join(', ') || '—'

  const notificationLabel =
    form.notification.filter === 'keyword' ? `关键词匹配: ${form.notification.keywords}` : '所有新内容'

  return (
    <div className="form-summary">
      <div className="section-header">
        <h2>确认配置</h2>
      </div>

      <div className="summary-grid">
        <div className="summary-row">
          <span className="summary-label">名称</span>
          <span className="summary-value">{form.basic.name || '—'}</span>
        </div>
        <div className="summary-row">
          <span className="summary-label">URL</span>
          <span className="summary-value url-value">{form.basic.url || '—'}</span>
        </div>
        {form.basic.group && (
          <div className="summary-row">
            <span className="summary-label">分组</span>
            <span className="summary-value">{form.basic.group}</span>
          </div>
        )}
        <div className="summary-row">
          <span className="summary-label">检查间隔</span>
          <span className="summary-value">{formatInterval(Number(form.basic.interval))}</span>
        </div>
        <div className="summary-row">
          <span className="summary-label">内容区域</span>
          <code className="summary-code">{form.extraction.containerSelector || '—'}</code>
        </div>
        {form.extraction.itemSelector && (
          <div className="summary-row">
            <span className="summary-label">列表项</span>
            <code className="summary-code">{form.extraction.itemSelector}</code>
          </div>
        )}
        <div className="summary-row">
          <span className="summary-label">字段</span>
          <span className="summary-value">{fieldNames}</span>
        </div>

        <div className="summary-divider"></div>
        <div className="summary-row">
          <span className="summary-label">通知方式</span>
          <span className="summary-value">{notificationLabel}</span>
        </div>
        <div className="summary-row">
          <span className="summary-label">推送账户</span>
          <span className="summary-value">{form.notification.accountIds.length} 个</span>
        </div>
      </div>

      <div className="summary-baseline">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
          <circle cx="12" cy="12" r="10" />
          <line x1="12" y1="16" x2="12" y2="12" />
          <line x1="12" y1="8" x2="12.01" y2="8" />
        </svg>
        <span>首次检查仅建立基线，不发送通知</span>
      </div>
    </div>
  )
}
