import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { NavLink, useLocation } from 'react-router'
import { fetchStats } from '../api/monitors'
import type { Stats } from '../api/types'
import UpdatePanel from './UpdatePanel'
import './AppLayout.css'

const STORAGE_KEY = 'gentry_theme'

export default function AppLayout({ children }: { children: ReactNode }) {
  const { pathname } = useLocation()
  const showRightSidebar = pathname === '/'

  const [isDark, setIsDark] = useState(false)
  const [statsLoading, setStatsLoading] = useState(true)
  const [statsError, setStatsError] = useState(false)
  const [statsOk, setStatsOk] = useState(true)
  const [lastUpdated, setLastUpdated] = useState<number | null>(null)
  const lastUpdatedRef = useRef<number | null>(null)
  // 10s ticker：触发重渲染以刷新“x 分钟前更新”文案
  const [, setTick] = useState(0)

  const [stats, setStats] = useState<Stats>({
    total_monitors: 0,
    running_monitors: 0,
    total_updates: 0,
    updates_last_hour: 0,
    unnotified_updates: 0,
    pushed_today: 0,
    total_accounts: 0,
  })

  const monitorPercent = !stats.total_monitors
    ? 0
    : Math.round((stats.running_monitors / stats.total_monitors) * 100)

  let lastUpdatedText = ''
  if (lastUpdated) {
    const diff = Math.floor((Date.now() - lastUpdated) / 1000)
    if (diff < 10) lastUpdatedText = '刚刚更新'
    else if (diff < 60) lastUpdatedText = `${diff}秒前更新`
    else lastUpdatedText = `${Math.floor(diff / 60)}分钟前更新`
  }

  function formatNum(n: number) {
    if (n >= 10000) return (n / 10000).toFixed(1) + '万'
    if (n >= 1000) return n.toLocaleString('zh-CN')
    return n
  }

  function applyTheme(dark: boolean) {
    document.documentElement.classList.toggle('dark', dark)
    setIsDark(dark)
    localStorage.setItem(STORAGE_KEY, dark ? 'dark' : 'light')
  }

  const loadStats = useCallback(async () => {
    try {
      const res = await fetchStats()
      if (res.code === 0 && res.data) {
        setStats((prev) => ({ ...prev, ...res.data }))
        setStatsOk(true)
        setStatsError(false)
        lastUpdatedRef.current = Date.now()
        setLastUpdated(lastUpdatedRef.current)
      } else {
        setStatsOk(false)
      }
    } catch {
      setStatsOk(false)
      if (!lastUpdatedRef.current) setStatsError(true)
    } finally {
      setStatsLoading(false)
    }
  }, [])

  useEffect(() => {
    const saved = localStorage.getItem(STORAGE_KEY)
    applyTheme(saved === 'dark')
  }, [])

  useEffect(() => {
    loadStats()
    const statsTimer = setInterval(loadStats, 15000)
    const tickTimer = setInterval(() => setTick((t) => t + 1), 10000)
    return () => {
      clearInterval(statsTimer)
      clearInterval(tickTimer)
    }
  }, [loadStats])

  const navItemClass = ({ isActive }: { isActive: boolean }) => 'nav-item' + (isActive ? ' active' : '')

  return (
    <div className="layout">
      {/* ═══ Left Sidebar ═══ */}
      <aside className="sidebar-left">
        <div className="sidebar-brand">
          <NavLink to="/" className="brand-link">
            <span className="brand-icon">
              <svg viewBox="0 0 24 24" fill="currentColor" width="22" height="22">
                <circle cx="12" cy="12" r="3" />
                <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8z" />
                <path d="M12 6c-3.31 0-6 2.69-6 6s2.69 6 6 6 6-2.69 6-6-2.69-6-6-6zm0 10c-2.21 0-4-1.79-4-4s1.79-4 4-4 4 1.79 4 4-1.79 4-4 4z" />
              </svg>
            </span>
            <span className="brand-text">Gentry</span>
          </NavLink>
        </div>

        <nav className="sidebar-nav">
          <NavLink to="/" end className={navItemClass}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="18" height="18">
              <rect x="3" y="3" width="7" height="7" rx="1" />
              <rect x="14" y="3" width="7" height="7" rx="1" />
              <rect x="3" y="14" width="7" height="7" rx="1" />
              <rect x="14" y="14" width="7" height="7" rx="1" />
            </svg>
            <span>监控列表</span>
          </NavLink>
          <NavLink to="/push" className={navItemClass}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="18" height="18">
              <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
              <path d="M13.73 21a2 2 0 0 1-3.46 0" />
            </svg>
            <span>通知方式</span>
          </NavLink>
          <NavLink to="/scan-rules" className={navItemClass}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="18" height="18">
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
              <polyline points="14 2 14 8 20 8" />
              <line x1="16" y1="13" x2="8" y2="13" />
              <line x1="16" y1="17" x2="8" y2="17" />
            </svg>
            <span>高级规则</span>
          </NavLink>
          <NavLink to="/settings" className={navItemClass}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="18" height="18">
              <circle cx="12" cy="12" r="3" />
              <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z" />
            </svg>
            <span>设置</span>
          </NavLink>
        </nav>

        <div className="sidebar-footer">
          <button className="nav-item theme-btn" title={isDark ? '切换亮色模式' : '切换暗色模式'} onClick={() => applyTheme(!isDark)}>
            {isDark ? (
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="18" height="18">
                <circle cx="12" cy="12" r="5" />
                <line x1="12" y1="1" x2="12" y2="3" />
                <line x1="12" y1="21" x2="12" y2="23" />
                <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
                <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
                <line x1="1" y1="12" x2="3" y2="12" />
                <line x1="21" y1="12" x2="23" y2="12" />
                <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" />
                <line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
              </svg>
            ) : (
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="18" height="18">
                <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
              </svg>
            )}
            <span>{isDark ? '亮色' : '暗色'}</span>
          </button>
          <UpdatePanel />
        </div>
      </aside>

      {/* ═══ Main Content ═══ */}
      <main className={`main-content${showRightSidebar ? ' has-right-sidebar' : ''}`}>{children}</main>

      {/* ═══ Right Sidebar: Stats ═══ */}
      {showRightSidebar && (
        <aside className="sidebar-right">
          <div className="stats-panel">
            <div className="panel-header">
              <h3>系统概览</h3>
              <span
                className={`status-dot ${statsOk ? 'dot-ok' : 'dot-err'}`}
                title={statsOk ? '正常运行' : '连接异常'}
              />
            </div>

            {statsLoading && stats.total_monitors === 0 ? (
              <>
                <div className="skel" />
                <div className="skel skel-sm" />
                <div className="skel-row">
                  <div className="skel" />
                  <div className="skel" />
                </div>
              </>
            ) : statsError ? (
              <p className="stats-error">暂时无法读取统计</p>
            ) : (
              <>
                {/* System Status */}
                <div className="stat-card">
                  <div className="stat-hero">
                    <span className="stat-hero-num">{stats.running_monitors}</span>
                    <span className="stat-hero-sep">/</span>
                    <span className="stat-hero-total">{stats.total_monitors}</span>
                  </div>
                  <p className="stat-hero-label">监控器运行中</p>
                  <div className="progress-track">
                    <div className="stats-progress-fill" style={{ width: `${monitorPercent}%` }} />
                  </div>
                </div>

                {/* Recent Activity */}
                <div className="stat-section">
                  <p className="stats-section-title">近期活动</p>
                  <div className="stat-grid">
                    <div className="grid-cell">
                      <span className="grid-num blue">{stats.updates_last_hour}</span>
                      <span className="grid-label">近1小时更新</span>
                    </div>
                    <div className="grid-cell">
                      <span className={`grid-num${stats.unnotified_updates > 0 ? ' orange' : ''}`}>
                        {stats.unnotified_updates}
                      </span>
                      <span className="grid-label">待推送</span>
                    </div>
                  </div>
                </div>

                {/* Cumulative */}
                <div className="stat-section">
                  <p className="stats-section-title">累计数据</p>
                  <div className="stat-row">
                    <span className="row-label">今日已推送</span>
                    <span className="row-value green">{stats.pushed_today}</span>
                  </div>
                  <div className="stat-row">
                    <span className="row-label">变更记录</span>
                    <span className="row-value">{formatNum(stats.total_updates)}</span>
                  </div>
                  <div className="stat-row">
                    <span className="row-label">推送账户</span>
                    <span className="row-value">{stats.total_accounts}</span>
                  </div>
                </div>
              </>
            )}

            <div className="panel-footer">
              <span className="updated-text">{lastUpdatedText}</span>
            </div>
          </div>
        </aside>
      )}
    </div>
  )
}
