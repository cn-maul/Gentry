import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import {
  fetchMonitor,
  fetchUpdates,
  fetchMonitorConfig,
  fetchAccounts,
  updateNotifyAccounts,
  startMonitor,
  stopMonitor,
  deleteMonitor,
  markAllNotified,
  markRead,
  manualCheck,
} from '../api/monitors'
import type { Monitor, NotifyAccount, UpdateRecord } from '../api/types'
import StatusBadge from '../components/StatusBadge'
import UpdateTable from '../components/UpdateTable'
import { useToastMessages } from '../hooks/useToastMessages'
import PageHeader from '../components/ui/PageHeader'
import Toasts from '../components/ui/Toasts'
import LoadingState from '../components/ui/LoadingState'
import EmptyState from '../components/ui/EmptyState'
import ConfirmModal from '../components/ui/ConfirmModal'

const UPDATES_PAGE_SIZE = 20

function formatTime(t?: string) {
  if (!t) return '—'
  return new Date(t).toLocaleString('zh-CN')
}

// check_interval 由后端以「秒」为单位返回（database/models.go），注意与 time.Duration 纳秒区分
function formatInterval(seconds?: number) {
  if (!seconds || seconds <= 0) return '—'
  const s = Math.floor(seconds)
  if (s >= 3600) return `${Math.round(s / 3600)} 小时`
  if (s >= 60) return `${Math.round(s / 60)} 分钟`
  return `${s} 秒`
}

function formatDuration(ns?: number) {
  if (!ns) return '—'
  const ms = Math.round(ns / 1e6)
  if (ms >= 1000) return `${(ms / 1000).toFixed(1)}s`
  return `${ms}ms`
}

function serviceLabel(s: string) {
  if (s === 'pushplus') return 'PushPlus'
  if (s === 'webhook') return 'Webhook'
  if (s === 'serverchan') return 'Server酱'
  return s
}

export default function MonitorDetail() {
  const { name: routeName } = useParams()
  const navigate = useNavigate()
  const { successMsg, pageErrorMsg, showSuccess, showError } = useToastMessages()

  const [monitor, setMonitor] = useState<Monitor | null>(null)
  const [records, setRecords] = useState<UpdateRecord[]>([])
  const [updatesPage, setUpdatesPage] = useState(1)
  const [updatesTotal, setUpdatesTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [updatesLoading, setUpdatesLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [actionLoading, setActionLoading] = useState(false)
  const [markLoading, setMarkLoading] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)

  // 推送账户选择
  const [allAccounts, setAllAccounts] = useState<NotifyAccount[]>([])
  const [selectedAccountIDs, setSelectedAccountIDs] = useState<Array<number | string>>([])

  const totalUpdatePages = Math.max(1, Math.ceil(updatesTotal / UPDATES_PAGE_SIZE))

  async function loadUpdates(page = 1) {
    if (!routeName) return
    setUpdatesLoading(true)
    try {
      const res = await fetchUpdates(routeName, { page, size: UPDATES_PAGE_SIZE })
      if (res.code === 0 && res.data) {
        setRecords(res.data.records || [])
        const total = res.data.total || 0
        setUpdatesTotal(total)
        const totalPages = Math.max(1, Math.ceil(total / UPDATES_PAGE_SIZE))
        if (page > totalPages) {
          setUpdatesPage(totalPages)
          await loadUpdates(totalPages)
          return
        }
        setUpdatesPage(page)
      }
    } catch {
      /* ignore */
    } finally {
      setUpdatesLoading(false)
    }
  }

  async function loadData() {
    if (!routeName) return
    setLoading(true)
    setError(null)
    try {
      const [res, configRes, acctsRes] = await Promise.all([
        fetchMonitor(routeName),
        fetchMonitorConfig(routeName).catch(() => null),
        fetchAccounts().catch(() => ({ code: -1, message: '', data: [] as NotifyAccount[] })),
      ])
      if (res.code === 0) {
        setMonitor(res.data)
        loadUpdates()
        markRead(routeName).catch(() => {})
      } else {
        setError(res.message || '监控器不存在')
      }

      // 加载推送账户列表
      setAllAccounts((acctsRes.code === 0 ? acctsRes.data : []) || [])

      // 解析当前监控器的启用账户 ID
      if (configRes && configRes.code === 0 && configRes.data) {
        setSelectedAccountIDs(configRes.data.notify_account_ids || [])
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : '加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadData()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [routeName])

  async function toggleRun() {
    if (!routeName || !monitor) return
    setActionLoading(true)
    try {
      if (monitor.is_running) {
        await stopMonitor(routeName)
        showSuccess('监控器已暂停')
      } else {
        await startMonitor(routeName)
        showSuccess('监控器已启动')
      }
      await loadData()
    } catch (e) {
      showError('操作失败: ' + (e instanceof Error ? e.message : ''))
    } finally {
      setActionLoading(false)
    }
  }

  async function handleDelete() {
    if (!routeName) return
    setActionLoading(true)
    try {
      await deleteMonitor(routeName)
      navigate('/')
    } catch (e) {
      showError('删除失败: ' + (e instanceof Error ? e.message : ''))
      setShowDeleteConfirm(false)
      setActionLoading(false)
    }
  }

  async function handleMarkAll() {
    if (!routeName) return
    setMarkLoading(true)
    try {
      const res = await markAllNotified(routeName)
      const n = res.data?.updated || 0
      showSuccess(`已将 ${n} 条记录标为已推送`)
      await loadUpdates(updatesPage)
    } catch (e) {
      showError('操作失败: ' + (e instanceof Error ? e.message : ''))
    } finally {
      setMarkLoading(false)
    }
  }

  function toggleAccount(accountID: number | string) {
    setSelectedAccountIDs((prev) => {
      const idx = prev.indexOf(accountID)
      if (idx >= 0) {
        const next = [...prev]
        next.splice(idx, 1)
        return next
      }
      return [...prev, accountID]
    })
  }

  async function saveAccounts() {
    if (!routeName) return
    setActionLoading(true)
    try {
      await updateNotifyAccounts(routeName, selectedAccountIDs)
      showSuccess(selectedAccountIDs.length > 0 ? '推送账户已保存' : '已关闭所有推送账户')
    } catch (e) {
      showError('保存失败: ' + (e instanceof Error ? e.message : ''))
    } finally {
      setActionLoading(false)
    }
  }

  async function handleManualCheck() {
    if (!routeName) return
    setActionLoading(true)
    try {
      const res = await manualCheck(routeName)
      const outcome = res.data || {}
      if (outcome.is_first_baseline) {
        showSuccess('检查完成，已建立初始基线')
      } else if ((outcome.count || 0) > 0) {
        showSuccess(`检查完成，发现 ${outcome.count} 条变化`)
      } else {
        showSuccess('检查完成，未发现新内容')
      }
      await loadData()
    } catch (e) {
      showError('检查失败: ' + (e instanceof Error ? e.message : ''))
    } finally {
      setActionLoading(false)
    }
  }

  return (
    <div className="monitor-detail">
      <PageHeader
        backTo="/"
        title={monitor ? monitor.name : '加载中...'}
        actions={
          monitor && (
            <>
              <button
                className={`circle-btn ${monitor.is_running ? 'btn-pause' : 'btn-play'}`}
                onClick={toggleRun}
                disabled={actionLoading}
                title={monitor.is_running ? '暂停' : '启动'}
              >
                {monitor.is_running ? (
                  <svg viewBox="0 0 24 24" fill="currentColor" width="18" height="18">
                    <rect x="6" y="4" width="4" height="16" rx="1" />
                    <rect x="14" y="4" width="4" height="16" rx="1" />
                  </svg>
                ) : (
                  <svg viewBox="0 0 24 24" fill="currentColor" width="18" height="18">
                    <path d="M8 5v14l11-7z" />
                  </svg>
                )}
              </button>
              <Link to={`/edit/${encodeURIComponent(monitor.name)}`} className="btn btn-ghost btn-sm">
                编辑
              </Link>
              <button className="btn btn-ghost btn-sm text-error" onClick={() => setShowDeleteConfirm(true)}>
                删除
              </button>
            </>
          )
        }
      />

      <Toasts success={successMsg} error={pageErrorMsg} />

      {loading ? (
        <LoadingState text="加载监控器详情..." />
      ) : error ? (
        <EmptyState
          icon="❌"
          title="加载失败"
          desc={error}
          action={
            <button className="btn btn-primary btn-sm" onClick={loadData}>
              重试
            </button>
          }
        />
      ) : (
        monitor && (
          <>
            {showDeleteConfirm && (
              <ConfirmModal
                open={showDeleteConfirm}
                title="确认删除"
                danger
                confirmText="确认删除"
                busy={actionLoading}
                onConfirm={handleDelete}
                onCancel={() => setShowDeleteConfirm(false)}
              >
                <p>确定要删除监控器「{monitor.name}」吗？</p>
                <p className="mt-2">删除后无法恢复，相关更新记录也会被清除。</p>
              </ConfirmModal>
            )}
            <div className="detail-panel settings-section">
              <div className="detail-left">
                <div className={`status-summary${monitor.last_error ? ' status-error' : ''}`}>
                  <div className="status-row">
                    <StatusBadge status={monitor.is_running ? 'running' : monitor.last_error ? 'error' : 'stopped'} />
                    <span className="interval-badge">{formatInterval(monitor.check_interval)}</span>
                  </div>
                  <div className="status-description">
                    {monitor.last_error ? (
                      <>
                        <span className="desc-error">检查失败</span>
                        <span className="desc-detail">{monitor.last_error}</span>
                      </>
                    ) : monitor.is_running ? (
                      <>
                        <span className="desc-ok">运行正常</span>
                        {monitor.last_check && <span className="desc-detail">{formatTime(monitor.last_check)}完成检查</span>}
                      </>
                    ) : (
                      <span className="desc-paused">已暂停</span>
                    )}
                  </div>
                </div>
                <div className="status-grid">
                  <div className="status-item">
                    <span className="status-label">网址</span>
                    <span className="status-value">{monitor.url}</span>
                  </div>
                  {monitor.group && (
                    <div className="status-item">
                      <span className="status-label">分组</span>
                      <span className="status-value">{monitor.group}</span>
                    </div>
                  )}
                  <div className="status-item">
                    <span className="status-label">上次检查</span>
                    <span className="status-value">{monitor.last_check ? formatTime(monitor.last_check) : '—'}</span>
                  </div>
                  <div className="status-item">
                    <span className="status-label">更新次数</span>
                    <span className="status-value">{monitor.updates_count || 0}</span>
                  </div>
                  <div className="status-item">
                    <span className="status-label">下次检查</span>
                    <span className="status-value">{monitor.next_check ? formatTime(monitor.next_check) : '—'}</span>
                  </div>
                  <div className="status-item">
                    <span className="status-label">检查耗时</span>
                    <span className="status-value">{monitor.last_duration ? formatDuration(monitor.last_duration) : '—'}</span>
                  </div>
                  {monitor.baseline_status && (
                    <div className="status-item">
                      <span className="status-label">基线状态</span>
                      <span className="status-value">{monitor.baseline_status === 'ready' ? '已建立' : '待建立'}</span>
                    </div>
                  )}
                </div>
                <div className="status-actions">
                  <button className="btn btn-sm btn-ghost" onClick={handleManualCheck} disabled={actionLoading}>
                    立即检查
                  </button>
                </div>
              </div>

              {allAccounts.length > 0 && (
                <div className="detail-right">
                  <div className="detail-divider"></div>
                  <div className="accounts-header">
                    <h3>推送账户</h3>
                    <Link to="/notifications" className="link-sm">
                      管理账户
                    </Link>
                  </div>
                  <div className="accounts-list">
                    {allAccounts.map((acc) => (
                      <label key={acc.id} className="account-checkbox">
                        <input
                          type="checkbox"
                          value={acc.id}
                          checked={selectedAccountIDs.includes(acc.id)}
                          onChange={() => toggleAccount(acc.id)}
                        />
                        <span className="acc-name">{acc.name}</span>
                        <span className={`service-tag badge-${acc.service}`}>{serviceLabel(acc.service)}</span>
                      </label>
                    ))}
                  </div>
                  {selectedAccountIDs.length === 0 && (
                    <p className="hint">未启用任何推送账户，发现更新时不会推送通知</p>
                  )}
                  <div className="accounts-actions">
                    <button className="btn btn-primary btn-sm" disabled={actionLoading} onClick={saveAccounts}>
                      {actionLoading ? '保存中...' : '保存'}
                    </button>
                  </div>
                </div>
              )}
            </div>

            <div className="settings-section">
              <div className="section-header row">
                <h2>更新历史</h2>
                {records.length > 0 && (
                  <button className="btn btn-sm btn-ghost" disabled={markLoading} onClick={handleMarkAll}>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
                      <polyline points="9 11 12 14 22 4" />
                      <path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" />
                    </svg>
                    全部标为已推送
                  </button>
                )}
              </div>
              <UpdateTable records={records} loading={updatesLoading} />
              {updatesTotal > UPDATES_PAGE_SIZE && (
                <div className="pagination">
                  <button
                    className="btn btn-sm btn-ghost"
                    disabled={updatesPage <= 1 || updatesLoading}
                    onClick={() => loadUpdates(updatesPage - 1)}
                  >
                    上一页
                  </button>
                  <span>
                    第 {updatesPage} / {totalUpdatePages} 页
                  </span>
                  <button
                    className="btn btn-sm btn-ghost"
                    disabled={updatesPage >= totalUpdatePages || updatesLoading}
                    onClick={() => loadUpdates(updatesPage + 1)}
                  >
                    下一页
                  </button>
                </div>
              )}
            </div>
          </>
        )
      )}
    </div>
  )
}
