import type { UpdateRecord } from '../api/types'

interface UpdateTableProps {
  records?: UpdateRecord[]
  loading?: boolean
}

function formatTime(t?: string) {
  if (!t) return '—'
  const d = new Date(t)
  return d.toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

export default function UpdateTable({ records = [], loading = false }: UpdateTableProps) {
  if (loading) {
    return (
      <div className="loading">
        <div className="spinner" />
        <p>加载中...</p>
      </div>
    )
  }

  if (records.length === 0) {
    return (
      <div className="empty">
        <p>暂无更新记录</p>
      </div>
    )
  }

  return (
    <table className="data-table">
      <thead>
        <tr>
          <th>时间</th>
          <th>标题</th>
          <th>通知</th>
        </tr>
      </thead>
      <tbody>
        {records.map((r) => (
          <tr key={r.ID}>
            <td className="whitespace-nowrap text-xs text-fg-muted">{formatTime(r.CreatedAt)}</td>
            <td>
              <a href={r.URL} target="_blank" rel="noopener" className="text-fg no-underline transition hover:text-brand">
                {r.Title}
              </a>
            </td>
            <td>
              <span
                className={`inline-block rounded-full px-2 py-[0.1rem] text-[0.6875rem] font-semibold tracking-[0.5px] ${
                  r.Notified ? 'bg-success-bg text-success' : 'bg-elevated text-fg-muted'
                }`}
              >
                {r.Notified ? '已推送' : '待推送'}
              </span>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
