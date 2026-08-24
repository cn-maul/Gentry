import StatusBadge from './StatusBadge'
import type { Monitor } from '../api/types'

interface MonitorCardProps {
  monitor: Monitor
  pending?: boolean
  onStart: () => void
  onStop: () => void
  onEdit: () => void
  onDelete: () => void
  onView: () => void
}

function formatTime(t?: string) {
  if (!t) return '—'
  const d = new Date(t)
  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  if (sameDay) return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
}

export default function MonitorCard({ monitor, pending = false, onStart, onStop, onEdit, onDelete, onView }: MonitorCardProps) {
  const isRunning = !!monitor.is_running
  const isError = !!monitor.last_error
  const statusText = isError ? 'error' : isRunning ? 'running' : 'stopped'

  const cardBase = 'group flex cursor-pointer select-none items-center gap-4 rounded-lg px-4 py-3 transition max-md:flex-wrap max-md:gap-2'
  const cardBg = isError
    ? 'bg-[rgba(243,114,127,0.06)] hover:bg-[rgba(243,114,127,0.1)]'
    : 'bg-card hover:bg-hover'
  const cardStopped = !isRunning ? 'opacity-55' : ''

  const iconBtn =
    'inline-flex items-center justify-center rounded-full bg-transparent p-[0.4rem] text-fg-muted opacity-0 transition group-hover:opacity-100 hover:bg-active hover:text-fg max-md:opacity-100'

  return (
    <div className={`${cardBase} ${cardBg} ${cardStopped}`} onClick={onView}>
      <div className="flex min-w-[140px] shrink-0 items-center gap-[0.65rem] max-md:min-w-0 max-md:flex-1">
        <StatusBadge status={statusText} />
        <span className="truncate text-sm font-bold text-fg">{monitor.name}</span>
      </div>

      <div className="min-w-0 flex-1">
        <span className="block truncate text-xs text-fg-muted" title={monitor.url}>
          {monitor.url}
        </span>
      </div>

      <div className="flex shrink-0 items-center gap-2 max-md:order-10 max-md:w-full">
        {monitor.last_check && <span className="whitespace-nowrap text-xs text-fg-muted">{formatTime(monitor.last_check)}</span>}
        {typeof monitor.updates_count === 'number' && monitor.updates_count > 0 && (
          <span className="whitespace-nowrap rounded-full bg-elevated px-2 py-[0.15rem] text-[0.6875rem] font-bold text-fg">
            {monitor.updates_count}
          </span>
        )}
        {monitor.last_error && (
          <span className="whitespace-nowrap rounded-full bg-error-bg px-2 py-[0.15rem] text-[0.6875rem] font-bold text-error" title={monitor.last_error}>
            错误
          </span>
        )}
      </div>

      <div
        className="flex shrink-0 items-center gap-1"
        onClick={(e) => {
          e.stopPropagation()
        }}
      >
        <button
          className={`flex h-9 w-9 items-center justify-center rounded-full border-none p-0 transition disabled:pointer-events-none disabled:opacity-40 ${
            isRunning ? 'bg-elevated text-fg hover:scale-[1.08] hover:bg-hover' : 'bg-brand text-black hover:scale-[1.08] hover:bg-brand-hover'
          }`}
          disabled={pending}
          onClick={() => (isRunning ? onStop() : onStart())}
          title={isRunning ? '暂停' : '启动'}
        >
          {isRunning ? (
            <svg viewBox="0 0 24 24" fill="currentColor" width="16" height="16">
              <rect x="6" y="4" width="4" height="16" rx="1" />
              <rect x="14" y="4" width="4" height="16" rx="1" />
            </svg>
          ) : (
            <svg viewBox="0 0 24 24" fill="currentColor" width="16" height="16">
              <path d="M8 5v14l11-7z" />
            </svg>
          )}
        </button>
        <button
          className={`${iconBtn} disabled:pointer-events-none disabled:opacity-40`}
          title="编辑"
          disabled={pending}
          onClick={onEdit}
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="h-[18px] w-[18px]">
            <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
            <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
          </svg>
        </button>
        <button
          className={`${iconBtn} hover:text-error disabled:pointer-events-none disabled:opacity-40`}
          title="删除"
          disabled={pending}
          onClick={onDelete}
        >
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="h-[18px] w-[18px]">
            <polyline points="3 6 5 6 21 6" />
            <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
          </svg>
        </button>
      </div>
    </div>
  )
}
