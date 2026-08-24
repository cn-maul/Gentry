type BadgeType = 'success' | 'error' | 'default'

export default function StatusBadge({ status = 'unknown' }: { status?: string }) {
  let type: BadgeType = 'default'
  if (status === 'running') type = 'success'
  else if (status === 'error') type = 'error'

  let label = '未知'
  if (status === 'running') label = '运行中'
  else if (status === 'stopped') label = '已停止'
  else if (status === 'error') label = '异常'

  const palette: Record<BadgeType, string> = {
    success: 'bg-success-bg text-success [&>span]:bg-success',
    error: 'bg-error-bg text-error [&>span]:bg-error',
    default: 'bg-elevated text-fg-muted [&>span]:bg-fg-muted',
  }

  return (
    <span
      className={`inline-flex items-center gap-[0.35rem] rounded-full px-[0.55rem] py-[0.15rem] text-[0.6875rem] font-semibold capitalize tracking-[0.3px] ${palette[type]}`}
    >
      <span className="h-1.5 w-1.5 rounded-full" />
      {label}
    </span>
  )
}
