import { ref } from 'vue'

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
export function useResource(fetcher, { initial = null } = {}) {
  const data = ref(initial)
  const loading = ref(false)
  const refreshing = ref(false)
  const error = ref(null)

  // 请求序号防竞态：旧响应不覆盖新响应
  let seq = 0

  async function run(background) {
    const current = ++seq
    if (background) refreshing.value = true
    else loading.value = true
    try {
      const res = await fetcher()
      if (current !== seq) return
      data.value = res && res.data !== undefined ? res.data : res
      error.value = null
    } catch (e) {
      if (current !== seq) return
      error.value = e.message || '加载失败'
    } finally {
      if (current === seq) {
        loading.value = false
        refreshing.value = false
      }
    }
  }

  const load = () => run(false)
  const refresh = () => run(true)

  return { data, loading, refreshing, error, load, refresh }
}
