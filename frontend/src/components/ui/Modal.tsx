import { useEffect, type ReactNode } from 'react'

interface ModalProps {
  open: boolean
  title: ReactNode
  /** 弹窗主体内容（string 或节点） */
  children: ReactNode
  footer?: ReactNode
  onClose: () => void
  /** 是否允许点击遮罩关闭，默认 true */
  dismissable?: boolean
}

/**
 * 通用的表单型弹窗容器：Esc / 遮罩点击可关闭；
 * 渲染为 modal-overlay → modal-container，样式在 components.css。
 * footer 为空时不渲染底部栏（主体自带操作按钮时使用）。
 */
export default function Modal({ open, title, children, footer, onClose, dismissable = true }: ModalProps) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return null

  return (
    <div
      className="modal-overlay"
      onClick={dismissable ? (e) => e.target === e.currentTarget && onClose() : undefined}
    >
      <div className="modal-container" role="dialog" aria-modal="true" aria-labelledby="dsh-modal-title">
        <div className="modal-header">
          <h2 id="dsh-modal-title">{title}</h2>
          <button className="modal-close" onClick={onClose} aria-label="关闭">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="18" height="18">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>
        <div className="modal-body">{children}</div>
        {footer && <div className="modal-footer">{footer}</div>}
      </div>
    </div>
  )
}