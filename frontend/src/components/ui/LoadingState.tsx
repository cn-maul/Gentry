interface LoadingStateProps {
  text?: string
}

/** 统一的加载中状态 */
export default function LoadingState({ text = '加载中...' }: LoadingStateProps) {
  return (
    <div className="loading">
      <div className="spinner" />
      <p>{text}</p>
    </div>
  )
}