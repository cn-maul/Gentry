import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { fetchMonitors, fetchStats, startMonitor, stopMonitor, deleteMonitor } from '../api/monitors'
import type { Monitor, Stats } from '../api/types'
import MonitorCard from '../components/MonitorCard'
import { useToastMessages } from '../hooks/useToastMessages'
import { useResource } from '../hooks/useResource'

interface MonitorOverride {
  isRunning?: boolean
  deleted?: boolean
}

function formatNum(n: number) {
  if (n >= 10000) return (n / 10000).toFixed(1) + '万'
  if (n >= 1000) return n.toLocaleString('zh-CN')
  return n
}

const EMPTY_STATS: Stats = {
  total_monitors: 0,
  running_monitors: 0,
  total_updates: 0,
  updates_last_hour: 0,
  unnotified_updates: 0,
  pushed_today: 0,
  total_accounts: 0,
}

export default function Dashboard() {
  const navigate = useNavigate()
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const { successMsg, pageErrorMsg, showSuccess, showError } = useToastMessages()

  const {
    data: monitors,
    loading,
    error,
    load: loadData,
    refresh,
  } = useResource<Monitor[]>(fetchMonitors, { initial: [] })

  // 系统统计（15s 轮询）
  const [stats, setStats] = useState<Stats>(EMPTY_STATS)
  const [lastUpdated, setLastUpdated] = useState<number | null>(null)
  const [, setTick] = useState(0)

  useEffect(() => {
    let cancelled = false
    const loadStats = async () => {
      try {
        const res = await fetchStats()
        if (!cancelled && res.code === 0 && res.data) {
          setStats((prev) => ({ ...prev, ...res.data }))
          setLastUpdated(Date.now())
        }
      } catch {
        /* 静默：保留上次数据 */
      }
    }
    loadStats()
    const statsTimer = setInterval(loadStats, 15000)
    const tickTimer = setInterval(() => setTick((t) => t + 1), 10000)
    return () => {
      cancelled = true
      clearInterval(statsTimer)
      clearInterval(tickTimer)
    }
  }, [])

  let lastUpdatedText = ''
  if (lastUpdated) {
    const diff = Math.floor((Date.now() - lastUpdated) / 1000)
    if (diff < 10) lastUpdatedText = '刚刚更新'
    else if (diff < 60) lastUpdatedText = `${diff}秒前更新`
    else lastUpdatedText = `${Math.floor(diff / 60)}分钟前更新`
  }

  // 乐观更新：操作成功后本地立即生效，refresh 拉到新数据后自动清除
  const [overrides, setOverrides] = useState<Record<string, MonitorOverride>>({})
  useEffect(() => {
    setOverrides({})
  }, [monitors])

  const monitorsView = useMemo(
    () =>
      (monitors || [])
        .map((m) => (overrides[m.name] ? { ...m, ...overrides[m.name] } : m))
        .filter((m) => !overrides[m.name]?.deleted),
    [monitors, overrides],
  )

  // 本地搜索：按名称 / 网址过滤
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return monitorsView
    return monitorsView.filter((m) => m.name.toLowerCase().includes(q) || (m.url || '').toLowerCase().includes(q))
  }, [monitorsView, query])

  // 目标级 pending：操作期间只禁用对应卡片的按钮，不触发全页 loading
  const [pendingNames, setPendingNames] = useState<Set<string>>(new Set())

  function setPending(name: string, on: boolean) {
    setPendingNames((prev) => {
      const next = new Set(prev)
      if (on) next.add(name)
      else next.delete(name)
      return next
    })
  }

  const groupList = useMemo(() => {
    const map: Record<string, { name: string; items: Monitor[] }> = {}
    for (const m of filtered) {
      const g = m.group || '默认'
      if (!map[g]) map[g] = { name: g, items: [] }
      map[g].items.push(m)
    }
    const keys = Object.keys(map).sort((a, b) => {
      if (a === '默认') return -1
      if (b === '默认') return 1
      return a.localeCompare(b, 'zh')
    })
    return keys.map((k) => map[k])
  }, [filtered])

  useEffect(() => {
    loadData()
  }, [loadData])

  // Esc 关闭删除确认弹窗
  useEffect(() => {
    if (!deleteTarget) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setDeleteTarget(null)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [deleteTarget])

  async function toggleMonitor(name: string, start: boolean) {
    setPending(name, true)
    try {
      await (start ? startMonitor(name) : stopMonitor(name))
      showSuccess(start ? `「${name}」已启动` : `「${name}」已暂停`)
      // 本地立即更新状态，再后台校准
      setOverrides((prev) => ({ ...prev, [name]: { ...prev[name], isRunning: start, deleted: false } }))
      refresh()
    } catch (e) {
      showError((start ? '启动失败: ' : '暂停失败: ') + (e instanceof Error ? e.message : ''))
    } finally {
      setPending(name, false)
    }
  }

  async function handleDelete() {
    const name = deleteTarget
    setDeleteTarget(null)
    if (!name) return
    setPending(name, true)
    try {
      await deleteMonitor(name)
      showSuccess(`「${name}」已删除`)
      // 本地移除，再后台校准
      setOverrides((prev) => ({ ...prev, [name]: { ...prev[name], deleted: true } }))
      refresh()
    } catch (e) {
      showError('删除失败: ' + (e instanceof Error ? e.message : ''))
    } finally {
      setPending(name, false)
    }
  }

  const runningPercent = !stats.total_monitors ? 0 : Math.round((stats.running_monitors / stats.total_monitors) * 100)

  return (
    <div className="dashboard">
      <div className="page-header">
        <div>
          <h1>监控总览</h1>
          <p className="page-desc">
            管理和监控网页内容变更
            {lastUpdatedText && (
              <>
                {' · '}
                <span className="text-[0.75rem] text-fg-muted">{lastUpdatedText}</span>
              </>
            )}
          </p>
        </div>
        <div className="header-actions">
          <Link to="/add" className="btn btn-primary">
            新增监控器
          </Link>
        </div>
      </div>

      <div className="stats-strip">
        <div className="stat-cell">
          <span className="stat-label">运行中监控</span>
          <span className="stat-num">
            {stats.running_monitors}
            <span className="align-baseline text-[0.85em] font-semibold text-fg-muted"> / {stats.total_monitors}</span>
          </span>
          <div className="stat-progress">
            <div className="stat-progress-fill" style={{ width: `${runningPercent}%` }} />
          </div>
        </div>
        <div className="stat-cell">
          <span className="stat-label">今日推送</span>
          <span className="stat-num accent">{formatNum(stats.pushed_today)}</span>
          <span className="stat-sub">累计 {formatNum(stats.total_updates)} 条变更</span>
        </div>
        <div className="stat-cell">
          <span className="stat-label">待推送更新</span>
          <span className={`stat-num${stats.unnotified_updates > 0 ? ' warn' : ''}`}>{stats.unnotified_updates}</span>
          <span className="stat-sub">近 1 小时更新 {stats.updates_last_hour} 条</span>
        </div>
        <div className="stat-cell">
          <span className="stat-label">推送账户</span>
          <span className="stat-num">{stats.total_accounts}</span>
          <span className="stat-sub">渠道就绪状态随时可调整</span>
        </div>
      </div>

      <div className="dashboard-toolbar">
        <div className="search-box">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
            <circle cx="11" cy="11" r="7" />
            <line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
          <input
            className="form-input"
            placeholder="搜索监控名称或网址..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        </div>
        <div className="toolbar-spacer" />
      </div>

      {successMsg && <div className="toast toast-success">{successMsg}</div>}
      {pageErrorMsg && <div className="toast toast-warning">{pageErrorMsg}</div>}

      {loading ? (
        <div className="loading">
          <div className="spinner" />
          <p>加载中...</p>
        </div>
      ) : error ? (
        <div className="empty">
          <div className="empty-icon">❌</div>
          <p>加载失败</p>
          <p className="mt-1 text-[0.8125rem] text-fg-muted">{error}</p>
          <button className="btn btn-primary btn-sm mt-4" onClick={loadData}>
            重试
          </button>
        </div>
      ) : !monitorsView || monitorsView.length === 0 ? (
        <div className="empty">
          <div className="empty-icon">📡</div>
          <p className="empty-title">还没有监控任务</p>
          <p className="empty-desc">粘贴网址创建监控；内容区域由已保存的扫描规则识别，可在「规则库」中维护</p>
          <Link to="/add" className="btn btn-primary mt-5">
            创建第一个监控
          </Link>
          <div className="empty-hints">
            <span>支持公告更新</span>
            <span className="hint-dot" />
            <span>规则驱动识别</span>
            <span className="hint-dot" />
            <span>多渠道推送</span>
          </div>
        </div>
      ) : filtered.length === 0 ? (
        <div className="empty">
          <div className="empty-icon">🔍</div>
          <p className="empty-title">没有匹配的监控器</p>
          <p className="empty-desc">换个关键词试试，或清空搜索</p>
          <button className="btn btn-ghost btn-sm mt-4" onClick={() => setQuery('')}>
            清空搜索
          </button>
        </div>
      ) : (
        groupList.map((group) => (
          <div key={group.name} className="group-section">
            <div className="group-header">
              <h2 className="group-title">{group.name}</h2>
              <span className="group-count">{group.items.length}</span>
            </div>
            <div className="group-list">
              {group.items.map((m) => (
                <MonitorCard
                  key={m.name}
                  monitor={m}
                  pending={pendingNames.has(m.name)}
                  onStart={() => toggleMonitor(m.name, true)}
                  onStop={() => toggleMonitor(m.name, false)}
                  onEdit={() => navigate(`/edit/${encodeURIComponent(m.name)}`)}
                  onDelete={() => setDeleteTarget(m.name)}
                  onView={() => navigate(`/monitor/${encodeURIComponent(m.name)}`)}
                />
              ))}
            </div>
          </div>
        ))
      )}

      {/* 删除确认弹窗：作为覆盖层渲染在列表之上，不再替换整个列表 */}
      {deleteTarget && (
        <div
          className="modal-overlay"
          onClick={(e) => {
            if (e.target === e.currentTarget) setDeleteTarget(null)
          }}
        >
          <div className="modal-container" role="dialog" aria-modal="true" aria-labelledby="dashboard-delete-title">
            <div className="modal-header">
              <h2 id="dashboard-delete-title">确认删除</h2>
              <button className="modal-close" onClick={() => setDeleteTarget(null)}>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="18" height="18">
                  <line x1="18" y1="6" x2="6" y2="18" />
                  <line x1="6" y1="6" x2="18" y2="18" />
                </svg>
              </button>
            </div>
            <div className="modal-body">
              <p>确定要删除监控器「{deleteTarget}」吗？</p>
              <p className="mt-2">删除后无法恢复。</p>
            </div>
            <div className="modal-footer">
              <button className="btn btn-ghost" onClick={() => setDeleteTarget(null)}>
                取消
              </button>
              <button className="btn btn-danger" onClick={handleDelete}>
                确认删除
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}