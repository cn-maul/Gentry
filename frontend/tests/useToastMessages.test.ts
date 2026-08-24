import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useToastMessages } from '../src/hooks/useToastMessages'

describe('useToastMessages', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('success 与 error 使用独立 timer：后触发的 error 不清除 success 的定时', () => {
    const { result } = renderHook(() => useToastMessages())

    act(() => result.current.showSuccess('已保存'))
    expect(result.current.successMsg).toBe('已保存')

    // 展示 error 会清空 success（避免重叠）
    act(() => result.current.showError('失败了'))
    expect(result.current.successMsg).toBe('')
    expect(result.current.pageErrorMsg).toBe('失败了')

    // error 定时 5 秒后消失
    act(() => {
      vi.advanceTimersByTime(5000)
    })
    expect(result.current.pageErrorMsg).toBe('')
  })

  it('error 展示中触发 success 会立即清掉 error', () => {
    const { result } = renderHook(() => useToastMessages())

    act(() => result.current.showError('失败了'))
    act(() => result.current.showSuccess('成功'))
    expect(result.current.pageErrorMsg).toBe('')
    expect(result.current.successMsg).toBe('成功')

    // success 3 秒后消失，且不会因 error 的残留 timer 提前消失
    act(() => {
      vi.advanceTimersByTime(2999)
    })
    expect(result.current.successMsg).toBe('成功')
    act(() => {
      vi.advanceTimersByTime(1)
    })
    expect(result.current.successMsg).toBe('')
  })

  it('连续两次 success 重置定时', () => {
    const { result } = renderHook(() => useToastMessages())

    act(() => result.current.showSuccess('第一条'))
    act(() => {
      vi.advanceTimersByTime(2000)
    })
    act(() => result.current.showSuccess('第二条'))
    act(() => {
      vi.advanceTimersByTime(2000)
    })
    // 距第二条仅 2 秒，应仍在展示
    expect(result.current.successMsg).toBe('第二条')
    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(result.current.successMsg).toBe('')
  })
})
