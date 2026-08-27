interface ToastsProps {
  success?: string
  error?: string
}

/** 统一的页面级 toast：成功（绿）/ 错误（红），渲染在内容区顶部 */
export default function Toasts({ success, error }: ToastsProps) {
  return (
    <>
      {success && <div className="toast toast-success">{success}</div>}
      {error && <div className="toast toast-error">{error}</div>}
    </>
  )
}