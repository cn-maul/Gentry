import { useCallback, useEffect, useRef, useState } from 'react'

export function useToastMessages() {
  const [successMsg, setSuccessMsg] = useState('')
  const [pageErrorMsg, setPageErrorMsg] = useState('')
  const successTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const errorTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    return () => {
      if (successTimer.current) clearTimeout(successTimer.current)
      if (errorTimer.current) clearTimeout(errorTimer.current)
    }
  }, [])

  const showSuccess = useCallback((msg: string) => {
    // 清掉正在展示的错误，避免两条 toast 重叠
    if (errorTimer.current) clearTimeout(errorTimer.current)
    setPageErrorMsg('')
    setSuccessMsg(msg)
    if (successTimer.current) clearTimeout(successTimer.current)
    successTimer.current = setTimeout(() => setSuccessMsg(''), 3000)
  }, [])

  const showError = useCallback((msg: string) => {
    if (successTimer.current) clearTimeout(successTimer.current)
    setSuccessMsg('')
    setPageErrorMsg(msg)
    if (errorTimer.current) clearTimeout(errorTimer.current)
    errorTimer.current = setTimeout(() => setPageErrorMsg(''), 5000)
  }, [])

  return { successMsg, pageErrorMsg, showSuccess, showError }
}
