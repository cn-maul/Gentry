import { useEffect, type ReactNode } from 'react'

interface ConfirmModalProps {
  open: boolean
  title: ReactNode
  /** 弹窗主体内容（string 或节点） */
  children: ReactNode
  confirmText?: string
  cancelText?: string
  danger?: boolean
  busy?: boolean
  onConfirm: () => void
  onCancel: () => void
}

/**
 * 统一的确认弹窗：Esc / 遮罩点击 / 取消按钮均可关闭；
 * danger 时确认按钮呈危险样式，busy 时禁用并显示文案。
 * 默认以「modal-overlay → modal-container」结构渲染（样式在 components.css）。
 */
export default function ConfirmModal({
  open,
  title,
  children,
  confirmText = '确认',
  cancelText = '取消',
  danger = false,
  busy = false,
  onConfirm,
  onCancel,
}: ConfirmModalProps) {
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onCancel()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onCancel])

  if (!open) return null

  return (
    <div
      className="modal-overlay"
      onClick={(e) => {
        if (e.target === e.currentTarget) onCancel()
      }}
    >
      <div className="modal-container" role="dialog" aria-modal="true" aria-labelledby="dsh-confirm-title">
        <div className="modal-header">
          <h2 id="dsh-confirm-title">{title}</h2>
          <button className="modal-close" onClick={onCancel} aria-label="关闭">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="18" height="18">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>
        <div className="modal-body">{children}</div>
        <div className="modal-footer">
          <button className="btn btn-ghost" onClick={onCancel} disabled={busy}>
            {cancelText}
          </button>
          <button className={danger ? 'btn btn-danger' : 'btn btn-primary'} onClick={onConfirm} disabled={busy}>
            {busy ? '处理中...' : confirmText}
          </button>
        </div>
      </div>
    </div>
  )
}