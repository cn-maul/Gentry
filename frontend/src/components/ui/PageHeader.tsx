import type { ReactNode } from 'react'
import { Link } from 'react-router'

interface PageHeaderProps {
  title: ReactNode
  desc?: ReactNode
  actions?: ReactNode
  /** 可选：页头返回链接，渲染在标题上方（如详情页） */
  backTo?: string
}

/** 统一的页头：返回链接（可选）+ 标题 + 描述（左）、操作按钮（右） */
export default function PageHeader({ title, desc, actions, backTo }: PageHeaderProps) {
  return (
    <header className="page-header">
      <div>
        {backTo && (
          <Link to={backTo} className="back-btn">
            ← 返回
          </Link>
        )}
        <h1>{title}</h1>
        {desc && <p className="page-desc">{desc}</p>}
      </div>
      {actions && <div className="header-actions">{actions}</div>}
    </header>
  )
}