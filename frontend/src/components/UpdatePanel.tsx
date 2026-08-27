import { useEffect, useRef, useState } from 'react'
import { fetchVersion, checkUpdate, applyUpdate, fetchUpdateStatus } from '../api/monitors'

type PanelState = 'idle' | 'checking' | 'available' | 'uptodate' | 'error' | 'downloading' | 'restarting' | 'failed'

// 重启等待上限：150 × 2s ≈ 5 分钟，超时放弃并回到 idle
const RESTART_POLL_LIMIT = 150

export default function UpdatePanel() {
  const [version, setVersion] = useState('')
  const [state, setState] = useState<PanelState>('idle')
  const [latestVersion, setLatestVersion] = useState('')
  const [errorMsg, setErrorMsg] = useState('')
  const statusTimer = useRef<ReturnType<typeof setInterval> | null>(null)
  const delayedTimers = useRef<ReturnType<typeof setTimeout>[]>([])
  const downloadURL = useRef('')

  const later = (fn: () => void, ms: number) => {
    delayedTimers.current.push(setTimeout(fn, ms))
  }

  const stopPolling = () => {
    if (statusTimer.current) {
      clearInterval(statusTimer.current)
      statusTimer.current = null
    }
  }

  useEffect(() => {
    let cancelled = false
    fetchVersion()
      .then((res) => {
        if (!cancelled && res.code === 0 && res.data) setVersion(res.data.version)
      })
      .catch(() => {})
    return () => {
      cancelled = true
      stopPolling()
      for (const t of delayedTimers.current) clearTimeout(t)
      delayedTimers.current = []
    }
  }, [])

  async function handleClick() {
    setState('checking')
    try {
      const res = await checkUpdate()
      if (res.code === 0 && res.data) {
        if (res.data.has_update) {
          setLatestVersion(res.data.latest_version)
          downloadURL.current = res.data.download_url
          setState('available')
        } else {
          setState('uptodate')
          later(() => setState('idle'), 2000)
        }
      } else {
        setState('error')
        later(() => setState('idle'), 2000)
      }
    } catch {
      setState('error')
      later(() => setState('idle'), 2000)
    }
  }

  async function handleUpdate() {
    if (!downloadURL.current) return
    setState('downloading')
    try {
      await applyUpdate()
      pollStatus()
    } catch {
      setState('failed')
      setErrorMsg('请求失败')
      later(() => setState('idle'), 3000)
    }
  }

  function pollStatus() {
    // 阶段一：轮询 /update/status 直到 done/error；
    // 阶段二（waitingRestart）：服务正在重启，改为探测版本接口，
    // 确认经历过一次不可达（restartConfirmed）且重新可达后，刷新版本号并回到 idle。
    let waitingRestart = false
    let restartConfirmed = false
    let ticks = 0
    stopPolling()
    statusTimer.current = setInterval(async () => {
      ticks++
      if (ticks > RESTART_POLL_LIMIT) {
        stopPolling()
        setState('idle')
        return
      }
      try {
        if (!waitingRestart) {
          const res = await fetchUpdateStatus()
          if (res.code !== 0 || !res.data) return
          if (res.data.status === 'done') {
            waitingRestart = true
            setState('restarting')
          } else if (res.data.status === 'error') {
            setErrorMsg(res.data.message || '更新失败')
            setState('failed')
            stopPolling()
            later(() => setState('idle'), 5000)
          }
          // 其余状态（下载中等）继续轮询
        } else {
          const v = await fetchVersion()
          if (v.code === 0 && v.data?.version) {
            // 老进程可能尚未退出；超过约 1 分钟仍可达则视为原地热更新成功
            if (restartConfirmed || ticks > 30) {
              stopPolling()
              setVersion(v.data.version)
              setState('idle')
            }
          } else {
            restartConfirmed = true
          }
        }
      } catch {
        if (!waitingRestart) {
          // 更新过程中服务不可达：视为正在重启，转入阶段二继续等待恢复
          waitingRestart = true
          setState('restarting')
        } else {
          restartConfirmed = true
        }
      }
    }, 2000)
  }

  return (
    <div className="update-panel">
      <div className="version-row">
        <span>当前版本</span>
        <span className="version-current">{version || '—'}</span>
      </div>

      {state === 'available' ? (
        <button className="update-btn" onClick={handleUpdate}>
          升级到 {latestVersion}
        </button>
      ) : (
        <button
          className="version-btn"
          disabled={state === 'checking' || state === 'downloading' || state === 'restarting'}
          onClick={handleClick}
        >
          {state === 'idle' && <span>检查更新</span>}
          {state === 'checking' && <span>检查中...</span>}
          {state === 'uptodate' && <span>已是最新版本</span>}
          {state === 'error' && <span>检查失败，点击重试</span>}
          {state === 'restarting' && <span>服务重启中...</span>}
          {state === 'failed' && <span>{errorMsg || '更新失败，点击重试'}</span>}
        </button>
      )}
      {state === 'downloading' && (
        <div className="progress-bar">
          <div className="progress-fill fill-anim" />
        </div>
      )}
    </div>
  )
}
