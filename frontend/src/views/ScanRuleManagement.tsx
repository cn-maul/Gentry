import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router'
import { deleteScanRule, exportScanRules, fetchScanRules, importScanRules } from '../api/monitors'
import type { ScanRule } from '../api/types'
import { useToastMessages } from '../hooks/useToastMessages'
import { useResource } from '../hooks/useResource'
import PageHeader from '../components/ui/PageHeader'
import EmptyState from '../components/ui/EmptyState'
import LoadingState from '../components/ui/LoadingState'
import Toasts from '../components/ui/Toasts'

function errorMessage(error: unknown) {
  if (error instanceof Error) return error.message || '操作失败'
  return '操作失败'
}

function scopeName(rule: ScanRule) {
  if (rule.scope_type === 'exact') return '页面'
  if (rule.scope_type === 'route') return '路由'
  if (rule.scope_type === 'section') return '同站目录'
  if (rule.scope_type === 'global') return '通用'
  return '旧版'
}

function scopeTarget(rule: ScanRule) {
  if (rule.scope_type === 'global') return '所有网站中结构相同的页面'
  return rule.source_url || `URL 包含 ${rule.url_contains}`
}

function isAPIRule(rule: ScanRule) {
  if (!rule.fetch_config) return false
  if (typeof rule.fetch_config === 'object') return rule.fetch_config.mode === 'api_json'
  try {
    return JSON.parse(rule.fetch_config).mode === 'api_json'
  } catch {
    return false
  }
}

export default function ScanRuleManagement() {
  const navigate = useNavigate()
  const { successMsg, pageErrorMsg, showSuccess, showError } = useToastMessages()

  const { data: rulesData, loading, error, load, refresh } = useResource<ScanRule[]>(fetchScanRules, { initial: [] })
  const rules = rulesData ?? []
  const [importing, setImporting] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const fileInput = useRef<HTMLInputElement | null>(null)

  useEffect(() => {
    load()
  }, [load])

  const allSelected = rules.length > 0 && selected.size === rules.length
  const someSelected = selected.size > 0 && !allSelected

  function toggleAll() {
    if (allSelected) {
      setSelected(new Set())
    } else {
      setSelected(new Set(rules.map((r) => r.id)))
    }
  }

  function toggleOne(id: number) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  async function handleDelete(rule: ScanRule) {
    if (!window.confirm(`确定删除规则「${rule.name}」吗？`)) return
    try {
      await deleteScanRule(rule.id)
      showSuccess('规则已删除')
      setSelected((prev) => {
        const next = new Set(prev)
        next.delete(rule.id)
        return next
      })
      refresh()
    } catch (error) {
      showError('删除规则失败: ' + errorMessage(error))
    }
  }

  async function handleExport() {
    setExporting(true)
    try {
      const ids = selected.size > 0 ? [...selected] : undefined
      const response = await exportScanRules(ids)
      if (response.code !== 0 || !response.data) throw new Error(response.message || '导出失败')
      const blob = new Blob([JSON.stringify(response.data, null, 2)], { type: 'application/json;charset=utf-8' })
      const link = document.createElement('a')
      link.href = URL.createObjectURL(blob)
      link.download = `gentry-scan-rules-${new Date().toISOString().slice(0, 10)}.json`
      link.click()
      URL.revokeObjectURL(link.href)
      showSuccess(`已导出 ${ids ? ids.length : rules.length} 条规则`)
      if (ids) setSelected(new Set())
    } catch (error) {
      showError('导出规则失败: ' + errorMessage(error))
    } finally {
      setExporting(false)
    }
  }

  async function handleImportFile(event: React.ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    if (!file) return
    setImporting(true)
    try {
      const payload = JSON.parse(await file.text())
      const response = await importScanRules(payload)
      if (response.code !== 0) throw new Error(response.message || '导入失败')
      const imported = response.data?.imported || 0
      const skipped = response.data?.skipped || 0
      showSuccess(`已导入 ${imported} 条${skipped ? `，跳过 ${skipped} 条同名规则` : ''}`)
      setSelected(new Set())
      await refresh()
    } catch (error) {
      showError('导入规则失败: ' + errorMessage(error))
    } finally {
      setImporting(false)
      event.target.value = ''
    }
  }

  return (
    <div className="scan-rules-page">
      <Toasts success={successMsg} error={pageErrorMsg} />

      <PageHeader
        title="规则库"
        actions={
          <>
            <input
              ref={fileInput}
              className="file-input"
              type="file"
              accept="application/json,.json"
              onChange={handleImportFile}
            />
            <button className="btn btn-ghost btn-sm" disabled={importing} onClick={() => fileInput.current?.click()}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                <path d="M12 3v12" />
                <path d="m7 10 5 5 5-5" />
                <path d="M5 21h14" />
              </svg>
              {importing ? '导入中...' : '导入'}
            </button>
            <button
              className="btn btn-ghost btn-sm"
              disabled={exporting || rules.length === 0}
              onClick={handleExport}
            >
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                <path d="M12 21V9" />
                <path d="m17 14-5-5-5 5" />
                <path d="M5 3h14" />
              </svg>
              {exporting ? '导出中...' : selected.size > 0 ? `导出选中 (${selected.size})` : '导出'}
            </button>
            <button className="btn btn-primary" onClick={() => navigate('/rules/add')}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16" aria-hidden="true">
                <path d="M12 5v14" />
                <path d="M5 12h14" />
              </svg>
              添加规则
            </button>
          </>
        }
      />

      {/* ═══ 已保存规则（表格） ═══ */}
      <section className="library-section" aria-labelledby="rule-library-title">
        <div className="section-title-row library-title">
          <h2 id="rule-library-title">已保存规则</h2>
          <span>{selected.size > 0 ? `已选 ${selected.size} / ${rules.length} 条` : `${rules.length} 条`}</span>
        </div>

        {loading ? (
          <LoadingState text="正在加载规则..." />
        ) : error ? (
          <EmptyState
            icon="❌"
            title="加载失败"
            desc={error}
            action={
              <button className="btn btn-primary btn-sm" onClick={load}>
                重试
              </button>
            }
          />
        ) : rules.length === 0 ? (
          <EmptyState
            icon="🗂️"
            title="还没有已保存的规则"
            desc="创建规则后，新增监控器时就能自动识别页面内容区域"
            action={
              <button className="btn btn-primary" onClick={() => navigate('/rules/add')}>
                添加第一条规则
              </button>
            }
          />
        ) : (
          <div className="settings-section rule-table-card">
            <table className="data-table">
              <thead>
                <tr>
                  <th className="check-col">
                    <input
                      type="checkbox"
                      className="row-checkbox"
                      checked={allSelected}
                      ref={(el) => {
                        if (el) el.indeterminate = someSelected
                      }}
                      onChange={toggleAll}
                      title="全选/取消全选"
                    />
                  </th>
                  <th>规则名称</th>
                  <th>适用地址</th>
                  <th>数据源</th>
                  <th>适用范围</th>
                  <th>字段</th>
                  <th>状态</th>
                  <th className="text-right">操作</th>
                </tr>
              </thead>
              <tbody>
                {rules.map((rule) => (
                  <tr key={rule.id}>
                    <td className="check-col">
                      <input
                        type="checkbox"
                        className="row-checkbox"
                        checked={selected.has(rule.id)}
                        onChange={() => toggleOne(rule.id)}
                      />
                    </td>
                    <td>
                      <span className="font-bold text-fg">{rule.name}</span>
                    </td>
                    <td>
                      <span className="rule-target-cell" title={scopeTarget(rule)}>
                        {scopeTarget(rule)}
                      </span>
                    </td>
                    <td>
                      <span className={`source-badge${isAPIRule(rule) ? ' api' : ''}`}>{isAPIRule(rule) ? 'JSON API' : '网页'}</span>
                    </td>
                    <td>
                      <span className={`scope-badge${rule.scope_type === 'global' ? ' scope-global' : ''}`}>{scopeName(rule)}</span>
                    </td>
                    <td>
                      <span className="text-[0.75rem] text-fg-secondary">{(rule.fields || []).length} 个</span>
                    </td>
                    <td>
                      {rule.enabled === false ? (
                        <span className="status-chip warn">已禁用</span>
                      ) : (
                        <span className="status-chip ok">启用</span>
                      )}
                    </td>
                    <td className="text-right">
                      <button className="btn-icon btn-icon-danger" title="删除规则" onClick={() => handleDelete(rule)}>
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                          <path d="M3 6h18" />
                          <path d="M8 6V4h8v2" />
                          <path d="m19 6-1 14H6L5 6" />
                          <path d="M10 11v5M14 11v5" />
                        </svg>
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  )
}