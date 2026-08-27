import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router'
import { deleteScanRule, exportScanRules, fetchScanRules, importScanRules } from '../api/monitors'
import type { ScanRule } from '../api/types'
import { useToastMessages } from '../hooks/useToastMessages'

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

  const [loading, setLoading] = useState(true)
  const [rules, setRules] = useState<ScanRule[]>([])
  const [importing, setImporting] = useState(false)
  const [exporting, setExporting] = useState(false)
  const fileInput = useRef<HTMLInputElement | null>(null)

  useEffect(() => {
    loadRules()
  }, [])

  async function loadRules() {
    setLoading(true)
    try {
      const response = await fetchScanRules()
      setRules(response.code === 0 ? response.data || [] : [])
    } catch (error) {
      showError('加载规则失败: ' + errorMessage(error))
    } finally {
      setLoading(false)
    }
  }

  async function handleDelete(rule: ScanRule) {
    if (!window.confirm(`确定删除规则「${rule.name}」吗？`)) return
    try {
      await deleteScanRule(rule.id)
      setRules((prev) => prev.filter((item) => item.id !== rule.id))
      showSuccess('规则已删除')
    } catch (error) {
      showError('删除规则失败: ' + errorMessage(error))
    }
  }

  async function handleExport() {
    setExporting(true)
    try {
      const response = await exportScanRules()
      if (response.code !== 0 || !response.data) throw new Error(response.message || '导出失败')
      const blob = new Blob([JSON.stringify(response.data, null, 2)], { type: 'application/json;charset=utf-8' })
      const link = document.createElement('a')
      link.href = URL.createObjectURL(blob)
      link.download = `gentry-scan-rules-${new Date().toISOString().slice(0, 10)}.json`
      link.click()
      URL.revokeObjectURL(link.href)
      showSuccess(`已导出 ${rules.length} 条规则`)
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
      await loadRules()
    } catch (error) {
      showError('导入规则失败: ' + errorMessage(error))
    } finally {
      setImporting(false)
      event.target.value = ''
    }
  }

  return (
    <div className="scan-rules-page">
      {successMsg && <div className="toast toast-success">{successMsg}</div>}
      {pageErrorMsg && <div className="toast toast-error">{pageErrorMsg}</div>}

      <header className="page-header">
        <div>
          <h1>规则库</h1>
          <p className="page-desc">共 {rules.length} 条规则；监控器的内容识别完全依赖这里的规则</p>
        </div>
        <div className="header-actions">
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
          <button className="btn btn-ghost btn-sm" disabled={exporting || rules.length === 0} onClick={handleExport}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
              <path d="M12 21V9" />
              <path d="m17 14-5-5-5 5" />
              <path d="M5 3h14" />
            </svg>
            {exporting ? '导出中...' : '导出'}
          </button>
          <button className="btn btn-primary" onClick={() => navigate('/rules/add')}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16" aria-hidden="true">
              <path d="M12 5v14" />
              <path d="M5 12h14" />
            </svg>
            添加规则
          </button>
        </div>
      </header>

      {/* ═══ 已保存规则（表格） ═══ */}
      <section className="library-section" aria-labelledby="rule-library-title">
        <div className="section-title-row library-title">
          <h2 id="rule-library-title">已保存规则</h2>
          <span>{rules.length} 条</span>
        </div>

        {loading ? (
          <div className="loading">
            <div className="spinner" />
            <p>正在加载规则...</p>
          </div>
        ) : rules.length === 0 ? (
          <div className="empty">
            <div className="empty-icon">🗂️</div>
            <p className="empty-title">还没有已保存的规则</p>
            <p className="empty-desc">创建规则后，新增监控器时就能自动识别页面内容区域</p>
            <button className="btn btn-primary mt-5" onClick={() => navigate('/rules/add')}>
              添加第一条规则
            </button>
          </div>
        ) : (
          <div className="settings-section rule-table-card">
            <table className="data-table">
              <thead>
                <tr>
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