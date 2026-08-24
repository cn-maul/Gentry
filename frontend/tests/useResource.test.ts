import { act, renderHook } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { useResource } from '../src/hooks/useResource'

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('useResource', () => {
  it('load 成功后写入 data 并清空 error', async () => {
    const fetcher = async () => ({ code: 0, data: [1, 2] })
    const { result } = renderHook(() => useResource<number[]>(fetcher))
    let p!: Promise<void>
    act(() => {
      p = result.current.load()
    })
    expect(result.current.loading).toBe(true)
    await act(async () => {
      await p
    })
    expect(result.current.data).toEqual([1, 2])
    expect(result.current.loading).toBe(false)
    expect(result.current.error).toBeNull()
  })

  it('load 失败置 error，不改动 data', async () => {
    const fetcher = async () => {
      throw new Error('网络错误')
    }
    const { result } = renderHook(() => useResource<number[]>(fetcher, { initial: [] }))
    await act(async () => {
      await result.current.load()
    })
    expect(result.current.data).toEqual([])
    expect(result.current.error).toBe('网络错误')
  })

  it('refresh 失败时保留旧 data', async () => {
    let fail = false
    const fetcher = async () => {
      if (fail) throw new Error('刷新失败')
      return { code: 0, data: ['old'] }
    }
    const { result } = renderHook(() => useResource<string[]>(fetcher))
    await act(async () => {
      await result.current.load()
    })
    expect(result.current.data).toEqual(['old'])

    fail = true
    let p!: Promise<void>
    act(() => {
      p = result.current.refresh()
    })
    expect(result.current.refreshing).toBe(true)
    await act(async () => {
      await p
    })
    expect(result.current.data).toEqual(['old'])
    expect(result.current.error).toBe('刷新失败')
    expect(result.current.refreshing).toBe(false)
  })

  it('防竞态：旧请求乱序返回不覆盖新请求结果', async () => {
    const first = deferred<{ code: number; data: string }>()
    const second = deferred<{ code: number; data: string }>()
    let call = 0
    const fetcher = () => {
      call++
      return (call === 1 ? first.promise : second.promise) as Promise<{ code: number; data: string }>
    }
    const { result } = renderHook(() => useResource<string>(fetcher))

    let p1!: Promise<void>
    let p2!: Promise<void>
    act(() => {
      p1 = result.current.load()
    })
    act(() => {
      p2 = result.current.load()
    })
    // 新请求先返回，旧请求后返回
    await act(async () => {
      second.resolve({ code: 0, data: 'new' })
      await p2
    })
    await act(async () => {
      first.resolve({ code: 0, data: 'stale' })
      await p1
    })

    expect(result.current.data).toBe('new')
  })
})
