import { useCallback, useRef, useState } from 'react'

// useResource 封装单个异步资源的加载状态。
// fetcher 返回后端标准响应 { code, message, data }（业务码异常已由 API 拦截器抛出）。
//
// 返回：
//   data       资源数据（响应的 .data）
//   loading    首次/前台加载中
//   refreshing 后台刷新中（保留旧 data）
//   error      最近一次失败的错误消息（成功后清空）
//   load()     前台加载
//   refresh()  后台刷新，失败时保留旧 data
export function useResource<T>(
  fetcher: () => Promise<{ data?: T } | T>,
  { initial = null }: { initial?: T | null } = {},
) {
  const [data, setData] = useState<T | null>(initial)
  const [loading, setLoading] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // 请求序号防竞态：旧响应不覆盖新响应
  const seq = useRef(0)

  const run = useCallback(
    async (background: boolean) => {
      const current = ++seq.current
      if (background) setRefreshing(true)
      else setLoading(true)
      try {
        const res = await fetcher()
        if (current !== seq.current) return
        setData(res && (res as { data?: T }).data !== undefined ? (res as { data: T }).data : (res as T))
        setError(null)
      } catch (e) {
        if (current !== seq.current) return
        setError(e instanceof Error ? e.message : '加载失败')
      } finally {
        if (current === seq.current) {
          setLoading(false)
          setRefreshing(false)
        }
      }
    },
    [fetcher],
  )

  const load = useCallback(() => run(false), [run])
  const refresh = useCallback(() => run(true), [run])

  return { data, loading, refreshing, error, load, refresh }
}
