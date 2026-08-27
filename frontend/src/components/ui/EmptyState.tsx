import type { ReactNode } from 'react'

interface EmptyStateProps {
  icon?: ReactNode
  title: ReactNode
  desc?: ReactNode
  action?: ReactNode
  /** 特性提示列表：渲染为「文本 · 文本 · 文本」，中间自动插入圆点分隔 */
  hints?: ReactNode[]
}

/** 统一的空状态提示：图标 + 标题 + 描述 +（可选）操作 +（可选）特性提示 */
export default function EmptyState({ icon = '📭', title, desc, action, hints }: EmptyStateProps) {
  return (
    <div className="empty">
      {icon && <div className="empty-icon">{icon}</div>}
      <p className="empty-title">{title}</p>
      {desc && <p className="empty-desc">{desc}</p>}
      {action && <div className="mt-5">{action}</div>}
      {hints && hints.length > 0 && (
        <div className="empty-hints">
          {hints.map((hint, i) => (
            <span key={i}>
              {i > 0 && <span className="hint-dot" />}
              {hint}
            </span>
          ))}
        </div>
      )}
    </div>
  )
}