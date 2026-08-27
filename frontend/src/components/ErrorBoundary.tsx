import { Component, type ReactNode } from 'react'

interface ErrorBoundaryState {
  error: Error | null
}

// 全局渲染错误兜底：任意子树抛错时给出可恢复的界面，而不是整站白屏
export default class ErrorBoundary extends Component<{ children: ReactNode }, ErrorBoundaryState> {
  state: ErrorBoundaryState = { error: null }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error }
  }

  render() {
    if (!this.state.error) return this.props.children
    return (
      <div className="empty" role="alert">
        <div className="empty-icon">💥</div>
        <p>页面渲染出错了</p>
        <p className="text-[0.8125rem] text-fg-muted">{this.state.error.message}</p>
        <div className="mt-4 flex justify-center gap-2">
          <button
            className="btn btn-primary btn-sm"
            onClick={() => {
              this.setState({ error: null })
            }}
          >
            重试
          </button>
          <button className="btn btn-ghost btn-sm" onClick={() => window.location.reload()}>
            刷新页面
          </button>
        </div>
      </div>
    )
  }
}
