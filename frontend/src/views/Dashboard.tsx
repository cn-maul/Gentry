import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import {
  fetchMonitors,
  fetchStats,
  fetchCategories,
  createCategory,
  renameCategory,
  deleteCategory,
  startMonitor,
  stopMonitor,
  deleteMonitor,
} from '../api/monitors'
import type { Category, Monitor, Stats } from '../api/types'
import MonitorCard from '../components/MonitorCard'
import { useToastMessages } from '../hooks/useToastMessages'
import { useResource } from '../hooks/useResource'
import PageHeader from '../components/ui/PageHeader'
import EmptyState from '../components/ui/EmptyState'
import LoadingState from '../components/ui/LoadingState'
import ConfirmModal from '../components/ui/ConfirmModal'
import Toasts from '../components/ui/Toasts'
import Modal from '../components/ui/Modal'

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

  // ── 分类（监控分组）──
  const [categories, setCategories] = useState<Category[]>([])
  const [activeCategory, setActiveCategory] = useState('全部')
  const [showCategoryManager, setShowCategoryManager] = useState(false)
  const [newCategoryName, setNewCategoryName] = useState('')
  const [categoryBusy, setCategoryBusy] = useState(false)
  const [categoryMsg, setCategoryMsg] = useState('')
  const [categoryError, setCategoryError] = useState('')
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editingName, setEditingName] = useState('')

  function loadCategories() {
    fetchCategories()
      .then((res) => {
        if (res.code === 0) setCategories(res.data || [])
      })
      .catch(() => {})
  }

  useEffect(() => {
    loadCategories()
  }, [])

  async function handleCreateCategory() {
    const name = newCategoryName.trim()
    if (!name) return
    setCategoryBusy(true)
    setCategoryError('')
    setCategoryMsg('')
    try {
      await createCategory(name)
      setNewCategoryName('')
      setCategoryMsg(`分类「${name}」已创建`)
      loadCategories()
    } catch (e) {
      setCategoryError(e instanceof Error ? e.message : '创建失败')
    } finally {
      setCategoryBusy(false)
    }
  }

  async function handleRenameCategory(id: number) {
    const name = editingName.trim()
    if (!name) return
    setCategoryBusy(true)
    setCategoryError('')
    setCategoryMsg('')
    try {
      await renameCategory(id, name)
      setEditingId(null)
      setEditingName('')
      setCategoryMsg('分类已重命名')
      loadCategories()
    } catch (e) {
      setCategoryError(e instanceof Error ? e.message : '重命名失败')
    } finally {
      setCategoryBusy(false)
    }
  }

  async function handleDeleteCategory(id: number, name: string) {
    if (!window.confirm(`确定删除分类「${name}」吗？其下监控器将迁移到「默认」`)) return
    setCategoryBusy(true)
    setCategoryError('')
    setCategoryMsg('')
    try {
      await deleteCategory(id)
      setCategoryMsg(`分类「${name}」已删除`)
      if (activeCategory === name) setActiveCategory('全部')
      loadCategories()
    } catch (e) {
      setCategoryError(e instanceof Error ? e.message : '删除失败')
    } finally {
      setCategoryBusy(false)
    }
  }

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
    // 分类筛选：activeCategory 为「全部」时不过滤，否则只看该分类下的监控
    const scoped = activeCategory === '全部' ? filtered : filtered.filter((m) => (m.group || '默认') === activeCategory)
    const map: Record<string, { name: string; items: Monitor[] }> = {}
    for (const m of scoped) {
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
  }, [filtered, activeCategory])

  // 每个分类下的监控数量（用于 chips 角标与筛选空态）
  const categoryCounts = useMemo(() => {
    const counts: Record<string, number> = {}
    for (const m of monitors || []) {
      const g = (m.group || '').trim() || '默认'
      counts[g] = (counts[g] || 0) + 1
    }
    return counts
  }, [monitors])

  const allCategoryNames = useMemo(() => {
    const names = (categories || []).map((c) => c.name)
    if (!names.includes('默认')) names.unshift('默认')
    return names
  }, [categories])

  useEffect(() => {
    loadData()
  }, [loadData])

  // Esc 关闭删除确认弹窗由 ConfirmModal 内部处理

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
      <PageHeader
        title="监控总览"
        desc={
          <>
            管理和监控网页内容变更
            {lastUpdatedText && (
              <>
                {' · '}
                <span className="text-[0.75rem] text-fg-muted">{lastUpdatedText}</span>
              </>
            )}
          </>
        }
        actions={
          <Link to="/add" className="btn btn-primary">
            新增监控器
          </Link>
        }
      />

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

      {/* 分类筛选 chips */}
      <div className="category-chips">
        <button
          type="button"
          className={`chip${activeCategory === '全部' ? ' active' : ''}`}
          onClick={() => setActiveCategory('全部')}
        >
          全部
          <span className="chip-count">{(monitors || []).length}</span>
        </button>
        {allCategoryNames.map((name) => (
          <button
            key={name}
            type="button"
            className={`chip${activeCategory === name ? ' active' : ''}`}
            onClick={() => setActiveCategory(name)}
          >
            {name}
            <span className="chip-count">{categoryCounts[name] ?? 0}</span>
          </button>
        ))}
        <button type="button" className="chip chip-ghost" onClick={() => setShowCategoryManager(true)}>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="13" height="13">
            <path d="M12 5v14" />
            <path d="M5 12h14" />
          </svg>
          管理分类
        </button>
      </div>

      <Toasts success={successMsg} error={pageErrorMsg} />

      {loading ? (
        <LoadingState text="加载中..." />
      ) : error ? (
        <EmptyState
          icon="❌"
          title="加载失败"
          desc={<span className="text-[0.8125rem] text-fg-muted">{error}</span>}
          action={
            <button className="btn btn-primary btn-sm" onClick={loadData}>
              重试
            </button>
          }
        />
      ) : !monitorsView || monitorsView.length === 0 ? (
        <EmptyState
          icon="📡"
          title="还没有监控任务"
          desc="粘贴网址创建监控；内容区域由已保存的扫描规则识别，可在「规则库」中维护"
          action={
            <Link to="/add" className="btn btn-primary">
              创建第一个监控
            </Link>
          }
          hints={['支持公告更新', '规则驱动识别', '多渠道推送']}
        />
      ) : filtered.length === 0 ? (
        activeCategory !== '全部' ? (
          <EmptyState
            icon="🗂️"
            title={`分类「${activeCategory}」下暂无监控`}
            desc="可切换回全部，或在该分类下新建监控"
            action={
              <button className="btn btn-ghost btn-sm" onClick={() => setActiveCategory('全部')}>
                查看全部监控
              </button>
            }
          />
        ) : (
          <EmptyState
            icon="🔍"
            title="没有匹配的监控器"
            desc="换个关键词试试，或清空搜索"
            action={
              <button className="btn btn-ghost btn-sm" onClick={() => setQuery('')}>
                清空搜索
              </button>
            }
          />
        )
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
      <ConfirmModal
        open={deleteTarget !== null}
        title="确认删除"
        danger
        confirmText="确认删除"
        busy={deleteTarget ? pendingNames.has(deleteTarget) : false}
        onConfirm={handleDelete}
        onCancel={() => setDeleteTarget(null)}
      >
        <p>确定要删除监控器「{deleteTarget}」吗？</p>
        <p className="mt-2">删除后无法恢复。</p>
      </ConfirmModal>

      {/* 分类管理弹窗 */}
      <Modal open={showCategoryManager} title="分类管理" onClose={() => setShowCategoryManager(false)}>
        <div className="form-group">
          <label>新建分类</label>
          <div className="category-create-row">
            <input
              value={newCategoryName}
              onChange={(e) => setNewCategoryName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleCreateCategory()
              }}
              className="form-input"
              placeholder="如 新闻资讯"
            />
            <button className="btn btn-primary" disabled={categoryBusy || !newCategoryName.trim()} onClick={handleCreateCategory}>
              添加
            </button>
          </div>
        </div>

        <div className="category-list">
          {(categories || []).map((cat) => (
            <div key={cat.id} className="category-row">
              {editingId === cat.id ? (
                <>
                  <input
                    value={editingName}
                    onChange={(e) => setEditingName(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') handleRenameCategory(cat.id)
                      if (e.key === 'Escape') {
                        setEditingId(null)
                        setEditingName('')
                      }
                    }}
                    className="form-input"
                    autoFocus
                  />
                  <button className="btn btn-sm btn-primary" disabled={categoryBusy} onClick={() => handleRenameCategory(cat.id)}>
                    保存
                  </button>
                  <button
                    className="btn btn-sm btn-ghost"
                    onClick={() => {
                      setEditingId(null)
                      setEditingName('')
                    }}
                  >
                    取消
                  </button>
                </>
              ) : (
                <>
                  <span className="category-name">{cat.name}</span>
                  <span className="category-count">{categoryCounts[cat.name] ?? 0} 个监控</span>
                  <div className="category-actions">
                    <button
                      className="btn btn-sm btn-ghost"
                      disabled={categoryBusy}
                      onClick={() => {
                        setEditingId(cat.id)
                        setEditingName(cat.name)
                      }}
                    >
                      重命名
                    </button>
                    <button
                      className="btn btn-sm btn-ghost btn-danger-text"
                      disabled={categoryBusy || cat.name === '默认'}
                      title={cat.name === '默认' ? '默认分类不可删除' : '删除后其下监控器迁移到「默认」'}
                      onClick={() => handleDeleteCategory(cat.id, cat.name)}
                    >
                      删除
                    </button>
                  </div>
                </>
              )}
            </div>
          ))}
          {(categories || []).length === 0 && (
            <p className="hint">
              <span className="empty-inline">暂无分类，可新建自定义分类；未分类的监控器属于「默认」。</span>
            </p>
          )}
        </div>
        {categoryMsg && <p className="hint group-hint">{categoryMsg}</p>}
        {categoryError && <p className="hint error-hint">{categoryError}</p>}
      </Modal>
    </div>
  )
}