import { useEffect, useState } from 'react'
import { fetchPushLogs } from '../api/monitors'
import type { PushLog } from '../api/types'
import PageHeader from '../components/ui/PageHeader'
import LoadingState from '../components/ui/LoadingState'
import EmptyState from '../components/ui/EmptyState'
import Modal from '../components/ui/Modal'

const PAGE_SIZE = 20

const FILTERS = [
  { key: '', label: '全部' },
  { key: 'success', label: '成功' },
  { key: 'partial', label: '部分失败' },
  { key: 'failed', label: '失败' },
  { key: 'skipped', label: '跳过' },
] as const

const STATUS_META: Record<string, { label: string; chip: string }> = {
  success: { label: '成功', chip: 'bg-success-bg text-success' },
  partial: { label: '部分失败', chip: 'bg-warning-bg text-warning' },
  failed: { label: '失败', chip: 'bg-error-bg text-error' },
  skipped: { label: '跳过', chip: 'bg-elevated text-fg-muted' },
}

function formatTime(t?: string) {
  if (!t) return '—'
  return new Date(t).toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

export default function PushHistory() {
  const [logs, setLogs] = useState<PushLog[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [status, setStatus] = useState('')
  const [detailLog, setDetailLog] = useState<PushLog | null>(null)

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  async function load(targetPage = page, targetStatus = status) {
    setLoading(true)
    setError(null)
    try {
      const res = await fetchPushLogs({ page: targetPage, size: PAGE_SIZE, status: targetStatus || undefined })
      if (res.code === 0 && res.data) {
        setLogs(res.data.records || [])
        setTotal(res.data.total || 0)
        const maxPage = Math.max(1, Math.ceil((res.data.total || 0) / PAGE_SIZE))
        if (targetPage > maxPage) {
          setPage(maxPage)
          if (maxPage !== targetPage) load(maxPage, targetStatus)
          return
        }
        setPage(targetPage)
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : '加载失败')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    load(1, '')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  function changeFilter(key: string) {
    if (key === status) return
    setStatus(key)
    load(1, key)
  }

  return (
    <div className="push-history-page">
      <PageHeader title="推送记录" />

      <div className="push-filter">
        {FILTERS.map((f) => (
          <button
            key={f.key}
            className={`btn btn-sm ${status === f.key ? 'btn-primary' : 'btn-ghost'}`}
            onClick={() => changeFilter(f.key)}
          >
            {f.label}
          </button>
        ))}
      </div>

      {loading ? (
        <LoadingState text="加载中..." />
      ) : error ? (
        <EmptyState
          icon="❌"
          title="加载失败"
          desc={<span className="text-[0.8125rem] text-fg-muted">{error}</span>}
          action={
            <button className="btn btn-primary btn-sm" onClick={() => load()}>
              重试
            </button>
          }
        />
      ) : logs.length === 0 ? (
        <EmptyState
          icon="✉️"
          title={status ? '该状态下暂无推送记录' : '暂无推送记录'}
          desc="推送记录会在每次推送尝试后自动生成，包含成功、失败与跳过的原因"
        />
      ) : (
        <>
          <div className="push-timeline">
            {logs.map((log) => {
              const meta = STATUS_META[log.status] || STATUS_META.skipped
              return (
                <div key={log.id} className={`push-item ${log.status}`}>
                  <div className="push-node" />
                  <div className="push-card">
                    <div className="push-head">
                      <span className="push-time">{formatTime(log.created_at)}</span>
                      <span className={`push-chip ${meta.chip}`}>{meta.label}</span>
                      <span className="push-site">{log.site_name}</span>
                      {log.account_names.length > 0 && (
                        <span className="push-accounts">推送至 {log.account_names.join('、')}</span>
                      )}
                      {log.item_count > 0 && <span className="push-count">{log.item_count} 条</span>}
                      <button className="btn btn-sm btn-ghost push-detail-btn" onClick={() => setDetailLog(log)}>
                        查看详情
                      </button>
                    </div>
                  </div>
                </div>
              )
            })}
          </div>

          {total > PAGE_SIZE && (
            <div className="pagination">
              <button
                className="btn btn-sm btn-ghost"
                disabled={page <= 1 || loading}
                onClick={() => load(page - 1)}
              >
                上一页
              </button>
              <span>
                第 {page} / {totalPages} 页
              </span>
              <button
                className="btn btn-sm btn-ghost"
                disabled={page >= totalPages || loading}
                onClick={() => load(page + 1)}
              >
                下一页
              </button>
            </div>
          )}
        </>
      )}

      {/* 推送详情弹窗 */}
      <Modal open={detailLog !== null} title="推送详情" onClose={() => setDetailLog(null)}>
        {detailLog && (
          <div className="push-detail-body">
            <div className="push-detail-grid">
              <div className="push-detail-row">
                <span className="push-detail-label">时间</span>
                <span>{formatTime(detailLog.created_at)}</span>
              </div>
              <div className="push-detail-row">
                <span className="push-detail-label">状态</span>
                <span className={`push-chip ${(STATUS_META[detailLog.status] || STATUS_META.skipped).chip}`}>
                  {(STATUS_META[detailLog.status] || STATUS_META.skipped).label}
                </span>
              </div>
              <div className="push-detail-row">
                <span className="push-detail-label">站点</span>
                <span>{detailLog.site_name}</span>
              </div>
              <div className="push-detail-row">
                <span className="push-detail-label">推送账户</span>
                <span>{detailLog.account_names.length ? detailLog.account_names.join('、') : '—'}</span>
              </div>
              <div className="push-detail-row">
                <span className="push-detail-label">更新条数</span>
                <span>{detailLog.item_count} 条</span>
              </div>
            </div>

            {detailLog.reason && <div className="push-reason">原因：{detailLog.reason}</div>}
            {detailLog.detail && detailLog.detail.trim() && (
              <div className={`push-detail${detailLog.status === 'failed' ? '' : ''}`}>{detailLog.detail}</div>
            )}
            {detailLog.titles.length > 0 && (
              <div className="push-detail-block">
                <h4>条目标题</h4>
                <ul className="push-titles">
                  {detailLog.titles.map((t, i) => (
                    <li key={i}>{t}</li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}
      </Modal>
    </div>
  )
}
