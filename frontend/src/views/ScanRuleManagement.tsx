import { useEffect, useMemo, useRef, useState } from 'react'
import {
  aiExtractScanRule,
  deleteScanRule,
  exportScanRules,
  fetchScanRules,
  importScanRules,
  quickCreateScanRule,
  validateMonitorConfig,
} from '../api/monitors'
import type { AIExtractResult, ScanRule, ValidationResult } from '../api/types'
import type { ExtractionField, FetchConfig, SourceVariable } from '../lib/monitorForm'
import { useToastMessages } from '../hooks/useToastMessages'
import './ScanRuleManagement.css'

type ScopeType = 'exact' | 'route' | 'section' | 'global'
type RuleSourceMode = 'html' | 'api_json'

interface RuleDraft {
  name: string
  url: string
  scopeType: ScopeType
  sourceMode: RuleSourceMode
  apiUrl: string
  itemsPath: string
  filterPath: string
  filterEquals: string
  headers: Record<string, string>
  variables: Array<{ name: string; selector: string; attr: string }>
  container: string
  item: string
  fields: ExtractionField[]
}

function createEmptyDraft(): RuleDraft {
  return {
    name: '',
    url: '',
    scopeType: 'exact',
    sourceMode: 'html',
    apiUrl: '',
    itemsPath: '',
    filterPath: '',
    filterEquals: '',
    headers: {},
    variables: [],
    container: '',
    item: '',
    fields: [{ name: 'title', selector: 'a', type: 'text', attr: '', transform: '' }],
  }
}

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

function draftFetchConfig(draft: RuleDraft): FetchConfig | undefined {
  if (draft.sourceMode !== 'api_json') return undefined
  const variables: Record<string, SourceVariable> = {}
  for (const variable of draft.variables) {
    if (variable.name.trim() && variable.selector.trim()) {
      variables[variable.name.trim()] = { source: 'html', selector: variable.selector.trim(), attr: variable.attr.trim() || undefined }
    }
  }
  return {
    mode: 'api_json',
    url: draft.apiUrl.trim(),
    items_path: draft.itemsPath.trim(),
    filter_path: draft.filterPath.trim(),
    filter_equals: draft.filterEquals.trim(),
    headers: draft.headers,
    ...(Object.keys(variables).length ? { variables } : {}),
  }
}

function validateDraft(draft: RuleDraft): string | null {
  if (!draft.name.trim()) return '规则名称不能为空'
  if (!draft.url.trim()) return '页面 URL 不能为空'
  if (draft.sourceMode === 'html') {
    if (!draft.container.trim()) return '容器选择器不能为空'
  } else {
    if (!draft.apiUrl.trim()) return 'JSON API URL 不能为空'
    if (!draft.itemsPath.trim()) return 'JSON API 列表路径不能为空'
  }
  const fields = draft.fields.filter((f) => f.name.trim())
  if (fields.length === 0) return '至少配置一个提取字段（title 用于展示）'
  if (!fields.some((f) => f.name.trim() === 'title')) return '规则必须包含 title 字段'
  return null
}

export default function ScanRuleManagement() {
  const { successMsg, pageErrorMsg, showSuccess, showError } = useToastMessages()

  const [loading, setLoading] = useState(true)
  const [rules, setRules] = useState<ScanRule[]>([])
  const [draft, setDraft] = useState<RuleDraft>(createEmptyDraft)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testError, setTestError] = useState('')
  const [testResult, setTestResult] = useState<ValidationResult | null>(null)
  const [importing, setImporting] = useState(false)
  const [exporting, setExporting] = useState(false)
  const fileInput = useRef<HTMLInputElement | null>(null)

  // AI 提取
  const [aiKeywords, setAIKeywords] = useState('')
  const [aiRunning, setAIRunning] = useState(false)
  const [aiResult, setAIResult] = useState<AIExtractResult | null>(null)
  const [aiError, setAIError] = useState('')

  const parsedURL = useMemo(() => {
    try {
      return new URL(draft.url.trim())
    } catch {
      return null
    }
  }, [draft.url])
  const routeScopeAvailable = Boolean(parsedURL && (parsedURL.pathname !== '/' || parsedURL.search))
  const sectionScopeAvailable = Boolean(parsedURL && parsedURL.pathname.split('/').filter(Boolean).length > 1)

  let scopeSummary = ''
  if (draft.scopeType === 'global') scopeSummary = '所有网站中结构相同的页面'
  else if (!parsedURL) scopeSummary = ''
  else if (draft.scopeType === 'section') {
    const parts = parsedURL.pathname.split('/').filter(Boolean)
    scopeSummary = `${parsedURL.host}/${parts.slice(0, -1).join('/')}/*`
  } else if (draft.scopeType === 'route') scopeSummary = `${parsedURL.host}${parsedURL.pathname}${parsedURL.search}`
  else scopeSummary = parsedURL.href

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

  function update<K extends keyof RuleDraft>(key: K, value: RuleDraft[K]) {
    setDraft((prev) => ({ ...prev, [key]: value }))
    setTestResult(null)
    setTestError('')
    if (key !== 'name') setAIResult(null)
  }

  async function handleAIExtract() {
    if (aiRunning || !draft.url.trim()) return
    setAIRunning(true)
    setAIError('')
    setAIResult(null)
    try {
      const response = await aiExtractScanRule({ url: draft.url.trim(), keywords: aiKeywords.trim() })
      const result = response.data
      if (!result?.config?.container) {
        throw new Error(result?.message || 'AI 未返回有效的选择器配置')
      }
      // AI 识别的是 HTML 结构，直接回填表单供人工核对与调整
      setDraft((prev) => ({
        ...prev,
        sourceMode: 'html',
        container: result.config.container || '',
        item: result.config.item || '',
        fields:
          result.config.fields && result.config.fields.length > 0
            ? result.config.fields
            : [{ name: 'title', selector: '', type: 'text', attr: '', transform: '' }],
      }))
      setTestResult(null)
      setTestError('')
      setAIResult(result)
    } catch (error) {
      setAIError(errorMessage(error))
    } finally {
      setAIRunning(false)
    }
  }

  function updateField(index: number, key: keyof ExtractionField, value: string) {
    setDraft((prev) => {
      const fields = prev.fields.map((field, i) => (i === index ? { ...field, [key]: value } : field))
      return { ...prev, fields }
    })
    setTestResult(null)
    setTestError('')
  }

  function addField() {
    setDraft((prev) => ({ ...prev, fields: [...prev.fields, { name: '', selector: '', type: 'text', attr: '', transform: '' }] }))
  }

  function removeField(index: number) {
    setDraft((prev) => ({ ...prev, fields: prev.fields.filter((_, i) => i !== index) }))
  }

  function addVariable() {
    setDraft((prev) => ({ ...prev, variables: [...prev.variables, { name: '', selector: '', attr: '' }] }))
  }

  function removeVariable(index: number) {
    setDraft((prev) => ({ ...prev, variables: prev.variables.filter((_, i) => i !== index) }))
  }

  function updateVariable(index: number, key: 'name' | 'selector' | 'attr', value: string) {
    setDraft((prev) => ({
      ...prev,
      variables: prev.variables.map((v, i) => (i === index ? { ...v, [key]: value } : v)),
    }))
  }

  function updateHeader(name: string, value: string) {
    setDraft((prev) => {
      const headers = { ...prev.headers }
      if (value.trim()) headers[name] = value
      else delete headers[name]
      return { ...prev, headers }
    })
  }

  async function handleTest() {
    const localError = validateDraft(draft)
    if (localError) {
      setTestError(localError)
      return
    }
    setTesting(true)
    setTestError('')
    setTestResult(null)
    try {
      const response = await validateMonitorConfig({
        name: draft.name.trim() || 'rule-test',
        url: draft.url.trim(),
        container: draft.sourceMode === 'api_json' ? draft.itemsPath.trim() : draft.container.trim(),
        item: draft.sourceMode === 'api_json' ? '*' : draft.item.trim(),
        fields: draft.fields.filter((f) => f.name.trim()),
        ...(draftFetchConfig(draft) ? { fetch_config: draftFetchConfig(draft) } : {}),
        check_interval: 3600,
        is_active: false,
        notify_filter: 'all',
        notify_keywords: '',
        notify_account_ids: [],
      })
      if (response.code !== 0 || !response.data?.valid) {
        throw new Error(response.message || response.data?.summary || '提取测试失败')
      }
      setTestResult(response.data)
    } catch (error) {
      setTestError(errorMessage(error))
    } finally {
      setTesting(false)
    }
  }

  async function handleSave() {
    const localError = validateDraft(draft)
    if (localError) {
      setTestError(localError)
      return
    }
    setSaving(true)
    try {
      await quickCreateScanRule({
        name: draft.name.trim(),
        url: draft.url.trim(),
        scope_type: draft.scopeType,
        config: {
          container: draft.sourceMode === 'api_json' ? draft.itemsPath.trim() : draft.container.trim(),
          item: draft.sourceMode === 'api_json' ? '*' : draft.item.trim(),
          ...(draftFetchConfig(draft) ? { fetch_config: draftFetchConfig(draft) } : {}),
          fields: draft.fields
            .filter((f) => f.name.trim())
            .map((f) => ({
              name: f.name.trim(),
              selector: f.selector.trim(),
              type: f.type || 'text',
              attr: f.attr || '',
              transform: f.transform || '',
            })),
        },
      })
      showSuccess('规则已保存')
      setDraft(createEmptyDraft())
      setTestResult(null)
      setTestError('')
      await loadRules()
    } catch (error) {
      showError('保存规则失败: ' + errorMessage(error))
    } finally {
      setSaving(false)
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
          <h1>高级规则</h1>
          <p>添加和管理可复用的内容提取规则；监控识别完全依赖这里的规则</p>
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
        </div>
      </header>

      <section className="builder-section" aria-labelledby="quick-rule-title">
        <div className="section-title-row">
          <h2 id="quick-rule-title">添加规则</h2>
        </div>

        <div className="price-builder-hint">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
            <circle cx="12" cy="12" r="10" />
            <line x1="12" y1="16" x2="12" y2="12" />
            <line x1="12" y1="8" x2="12.01" y2="8" />
          </svg>
          <span>
            手动填写 CSS 选择器或 JSON API 路径来定义提取规则。可以先用浏览器开发者工具定位选择器，再用「测试提取」验证；也可以填写关键词后用「AI 提取」自动识别。
          </span>
        </div>

        <div className="ai-extract-row">
          <div className="form-group ai-keyword-field">
            <label htmlFor="ai-keywords">
              关键词 <span className="optional-label">辅助 AI 定位</span>
            </label>
            <input
              id="ai-keywords"
              value={aiKeywords}
              onChange={(e) => setAIKeywords(e.target.value)}
              className="form-input"
              placeholder="正确条目包含的词，如：公告,公示"
              onKeyUp={(e) => e.key === 'Enter' && handleAIExtract()}
            />
          </div>
          <button
            className="btn btn-primary ai-extract-button"
            disabled={aiRunning || !draft.url.trim()}
            onClick={handleAIExtract}
            title="抓取页面并让 AI 识别内容列表结构，自动填入下方表单"
          >
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14" aria-hidden="true">
              <path d="M12 2a7 7 0 0 1 7 7c0 2.4-1.2 4.1-2.5 5.4-.7.7-1.1 1.3-1.3 2L15 18H9l-.2-1.6c-.2-.7-.6-1.3-1.3-2C6.2 13.1 5 11.4 5 9a7 7 0 0 1 7-7z" />
              <path d="M9.5 21h5" />
            </svg>
            {aiRunning ? 'AI 识别中...' : 'AI 提取'}
          </button>
        </div>
        <p className="ai-extract-hint">
          先在上方填写页面 URL。AI 提取需要在「设置」页配置 AI 模型；未提供关键词时 AI 会自行判断主要内容列表。
        </p>

        {aiRunning && (
          <div className="scan-loading">
            <div className="spinner" />
            <span>AI 正在分析页面结构（可能需要几十秒）</span>
          </div>
        )}

        {aiError && <div className="inline-error">{aiError}</div>}

        {aiResult && (
          <div className={`ai-result-preview${aiResult.verified ? '' : ' unverified'}`}>
            <div className="preview-heading">
              <strong>AI 提取结果</strong>
              <span>{aiResult.message}</span>
            </div>
            {(aiResult.samples || []).length > 0 && (
              <div className="sample-list">
                {aiResult.samples!.slice(0, 6).map((item, index) => (
                  <div key={index} className="sample-line">
                    <span className="sample-title">{item.title || item.url || '未命名条目'}</span>
                    {item.date && <span className="sample-date">{item.date}</span>}
                  </div>
                ))}
              </div>
            )}
            <p className="ai-result-note">
              已填入下方表单，请核对样本后「测试提取」并保存。{aiResult.verified ? '' : '注意：样本未命中关键词，请确认区域是否正确。'}
            </p>
          </div>
        )}

        <div className="scan-form">
          <div className="form-group name-field">
            <label htmlFor="rule-name">规则名称</label>
            <input
              id="rule-name"
              value={draft.name}
              onChange={(e) => update('name', e.target.value)}
              className="form-input"
              placeholder="例如：招聘公告列表"
            />
          </div>
          <div className="form-group url-field">
            <label htmlFor="rule-url">页面 URL</label>
            <input
              id="rule-url"
              value={draft.url}
              onChange={(e) => update('url', e.target.value)}
              className="form-input"
              placeholder="https://example.com/announcements/"
            />
          </div>
          <div className="form-group">
            <label htmlFor="rule-source">数据源</label>
            <select
              id="rule-source"
              value={draft.sourceMode}
              onChange={(e) => update('sourceMode', e.target.value as RuleSourceMode)}
              className="form-input"
            >
              <option value="html">网页 HTML</option>
              <option value="api_json">公开 JSON API</option>
            </select>
          </div>
        </div>

        {draft.sourceMode === 'html' ? (
          <div className="price-grid primary-fields">
            <div className="form-group">
              <label htmlFor="rule-container">容器选择器</label>
              <input
                id="rule-container"
                value={draft.container}
                onChange={(e) => update('container', e.target.value)}
                className="form-input"
                placeholder="如 ul.news-list"
              />
            </div>
            <div className="form-group">
              <label htmlFor="rule-item">条目选择器</label>
              <input
                id="rule-item"
                value={draft.item}
                onChange={(e) => update('item', e.target.value)}
                className="form-input"
                placeholder="如 li"
              />
            </div>
          </div>
        ) : (
          <>
            <div className="price-grid source-fields">
              <div className="form-group api-url-field">
                <label htmlFor="rule-api-url">JSON API URL</label>
                <input
                  id="rule-api-url"
                  value={draft.apiUrl}
                  onChange={(e) => update('apiUrl', e.target.value)}
                  className="form-input"
                  placeholder="https://example.com/api/list"
                />
              </div>
              <div className="form-group">
                <label htmlFor="rule-items-path">列表路径</label>
                <input
                  id="rule-items-path"
                  value={draft.itemsPath}
                  onChange={(e) => update('itemsPath', e.target.value)}
                  className="form-input"
                  placeholder="data"
                />
              </div>
              <div className="form-group">
                <label htmlFor="rule-filter-path">过滤字段（可选）</label>
                <input
                  id="rule-filter-path"
                  value={draft.filterPath}
                  onChange={(e) => update('filterPath', e.target.value)}
                  className="form-input"
                  placeholder="is_selling"
                />
              </div>
              <div className="form-group">
                <label htmlFor="rule-filter-value">过滤值</label>
                <input
                  id="rule-filter-value"
                  value={draft.filterEquals}
                  onChange={(e) => update('filterEquals', e.target.value)}
                  className="form-input"
                  placeholder="true"
                />
              </div>
            </div>

            <div className="variable-section">
              <div className="variable-heading">
                <div>
                  <strong>动态参数</strong>
                  <span>先从页面提取，再替换 API URL 中的 {'{{参数名}}'}</span>
                </div>
                <button type="button" className="btn btn-ghost btn-sm" onClick={addVariable}>
                  添加参数
                </button>
              </div>
              {draft.variables.map((variable, index) => (
                <div key={index} className="variable-row">
                  <div className="form-group">
                    <label htmlFor={`rule-variable-name-${index}`}>参数名</label>
                    <input
                      id={`rule-variable-name-${index}`}
                      value={variable.name}
                      onChange={(e) => updateVariable(index, 'name', e.target.value)}
                      className="form-input"
                      placeholder="goods_id"
                    />
                  </div>
                  <div className="form-group">
                    <label htmlFor={`rule-variable-selector-${index}`}>页面选择器</label>
                    <input
                      id={`rule-variable-selector-${index}`}
                      value={variable.selector}
                      onChange={(e) => updateVariable(index, 'selector', e.target.value)}
                      className="form-input"
                      placeholder="#goods_id"
                    />
                  </div>
                  <div className="form-group">
                    <label htmlFor={`rule-variable-attr-${index}`}>取值属性</label>
                    <input
                      id={`rule-variable-attr-${index}`}
                      value={variable.attr}
                      onChange={(e) => updateVariable(index, 'attr', e.target.value)}
                      className="form-input"
                      placeholder="value（留空则取文本）"
                    />
                  </div>
                  <button
                    type="button"
                    className="btn-icon btn-icon-danger variable-remove"
                    title="删除动态参数"
                    onClick={() => removeVariable(index)}
                  >
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                      <path d="M3 6h18" />
                      <path d="m19 6-1 14H6L5 6" />
                    </svg>
                  </button>
                </div>
              ))}
            </div>

            <div className="price-grid header-fields">
              <div className="form-group">
                <label htmlFor="rule-accept">Accept</label>
                <input
                  id="rule-accept"
                  value={draft.headers.Accept || ''}
                  onChange={(e) => updateHeader('Accept', e.target.value)}
                  className="form-input"
                  placeholder="application/json"
                />
              </div>
              <div className="form-group">
                <label htmlFor="rule-language">Accept-Language</label>
                <input
                  id="rule-language"
                  value={draft.headers['Accept-Language'] || ''}
                  onChange={(e) => updateHeader('Accept-Language', e.target.value)}
                  className="form-input"
                  placeholder="zh-CN,zh;q=0.9"
                />
              </div>
              <div className="form-group">
                <label htmlFor="rule-referer">Referer</label>
                <input
                  id="rule-referer"
                  value={draft.headers.Referer || ''}
                  onChange={(e) => updateHeader('Referer', e.target.value)}
                  className="form-input"
                  placeholder={draft.url || 'https://example.com/list'}
                />
              </div>
              <div className="form-group">
                <label htmlFor="rule-requested-with">X-Requested-With</label>
                <input
                  id="rule-requested-with"
                  value={draft.headers['X-Requested-With'] || ''}
                  onChange={(e) => updateHeader('X-Requested-With', e.target.value)}
                  className="form-input"
                  placeholder="XMLHttpRequest"
                />
              </div>
            </div>
          </>
        )}

        <div className="variable-section">
          <div className="variable-heading">
            <div>
              <strong>提取字段</strong>
              <span>{draft.sourceMode === 'api_json' ? '选择器填写 JSON 路径' : '选择器留空时取条目自身文本'}</span>
            </div>
            <button type="button" className="btn btn-ghost btn-sm" onClick={addField}>
              添加字段
            </button>
          </div>
          {draft.fields.map((field, index) => (
            <div key={index} className="variable-row">
              <div className="form-group">
                <label htmlFor={`rule-field-name-${index}`}>名称</label>
                <input
                  id={`rule-field-name-${index}`}
                  value={field.name}
                  onChange={(e) => updateField(index, 'name', e.target.value)}
                  className="form-input"
                  placeholder="title"
                />
              </div>
              <div className="form-group">
                <label htmlFor={`rule-field-selector-${index}`}>{draft.sourceMode === 'api_json' ? 'JSON 路径' : '选择器'}</label>
                <input
                  id={`rule-field-selector-${index}`}
                  value={field.selector}
                  onChange={(e) => updateField(index, 'selector', e.target.value)}
                  className="form-input"
                  placeholder={draft.sourceMode === 'api_json' ? 'name' : 'a'}
                />
              </div>
              <div className="form-group">
                <label htmlFor={`rule-field-type-${index}`}>类型</label>
                <select
                  id={`rule-field-type-${index}`}
                  value={field.type}
                  onChange={(e) => updateField(index, 'type', e.target.value)}
                  className="form-input"
                >
                  <option value="text">文本 (text)</option>
                  <option value="attr">属性 (attr)</option>
                </select>
              </div>
              {field.type === 'attr' && (
                <div className="form-group">
                  <label htmlFor={`rule-field-attr-${index}`}>属性名</label>
                  <input
                    id={`rule-field-attr-${index}`}
                    value={field.attr}
                    onChange={(e) => updateField(index, 'attr', e.target.value)}
                    className="form-input"
                    placeholder="默认 href"
                  />
                </div>
              )}
              <div className="form-group">
                <label htmlFor={`rule-field-transform-${index}`}>转换</label>
                <input
                  id={`rule-field-transform-${index}`}
                  value={field.transform}
                  onChange={(e) => updateField(index, 'transform', e.target.value)}
                  className="form-input"
                  placeholder="如 trim([])"
                />
              </div>
              <button
                type="button"
                className="btn-icon btn-icon-danger variable-remove"
                title="删除字段"
                onClick={() => removeField(index)}
              >
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                  <path d="M3 6h18" />
                  <path d="m19 6-1 14H6L5 6" />
                </svg>
              </button>
            </div>
          ))}
        </div>

        {testError && <div className="inline-error">{testError}</div>}
        {testResult && (
          <div className="price-preview">
            <div className="preview-heading">
              <strong>提取结果</strong>
              <span>{testResult.extracted_items} 条</span>
            </div>
            <div className="sample-list">
              {(testResult.items?.[0]?.samples || []).map((item, index) => (
                <div key={index} className="sample-line">
                  <span className="sample-title">{item.raw || item.item_key}</span>
                </div>
              ))}
            </div>
          </div>
        )}

        <div className="price-actions">
          <div className="scope-field">
            <span className="field-label">适用范围</span>
            <div className="scope-control">
              <button type="button" className={draft.scopeType === 'exact' ? 'active' : ''} onClick={() => update('scopeType', 'exact')}>
                当前页面
              </button>
              <button
                type="button"
                disabled={!routeScopeAvailable}
                className={draft.scopeType === 'route' ? 'active' : ''}
                title="匹配当前路径及其子路径"
                onClick={() => update('scopeType', 'route')}
              >
                当前路由
              </button>
              <button
                type="button"
                disabled={!sectionScopeAvailable}
                className={draft.scopeType === 'section' ? 'active' : ''}
                title="匹配同一网站、同一目录下的页面"
                onClick={() => update('scopeType', 'section')}
              >
                同站目录
              </button>
              <button
                type="button"
                className={draft.scopeType === 'global' ? 'active' : ''}
                title="跨网站按相同页面结构匹配"
                onClick={() => update('scopeType', 'global')}
              >
                通用结构
              </button>
            </div>
          </div>
          <div className="scope-summary">{scopeSummary}</div>
          <button className="btn btn-ghost" disabled={testing} onClick={handleTest}>
            {testing ? '测试中...' : '测试提取'}
          </button>
          <button className="btn btn-primary" disabled={saving || testing || !draft.name.trim()} onClick={handleSave}>
            {saving ? '保存中...' : '保存规则'}
          </button>
        </div>
      </section>

      <section className="library-section" aria-labelledby="rule-library-title">
        <div className="section-title-row library-title">
          <h2 id="rule-library-title">已保存规则</h2>
          <span>{rules.length}</span>
        </div>

        {loading ? (
          <div className="list-state">正在加载规则</div>
        ) : rules.length === 0 ? (
          <div className="list-state">暂无已保存规则</div>
        ) : (
          <div className="rule-list">
            {rules.map((rule) => (
              <article key={rule.id} className="rule-row">
                <div className="rule-main">
                  <div className="rule-heading">
                    <strong>{rule.name}</strong>
                    <span className={`source-badge${isAPIRule(rule) ? ' api' : ''}`}>{isAPIRule(rule) ? 'JSON API' : '网页'}</span>
                    <span className={`scope-badge scope-${rule.scope_type || 'legacy'}`}>{scopeName(rule)}</span>
                    {!rule.enabled && <span className="disabled-badge">已禁用</span>}
                  </div>
                  <div className="rule-target">{scopeTarget(rule)}</div>
                  <div className="rule-structure">
                    <code>{rule.container}</code>
                    <span>/</span>
                    <code>{rule.item}</code>
                    <span className="field-count">{(rule.fields || []).length} 个字段</span>
                  </div>
                </div>
                <div className="rule-actions">
                  <button className="btn-icon btn-icon-danger" title="删除规则" onClick={() => handleDelete(rule)}>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                      <path d="M3 6h18" />
                      <path d="M8 6V4h8v2" />
                      <path d="m19 6-1 14H6L5 6" />
                      <path d="M10 11v5M14 11v5" />
                    </svg>
                  </button>
                </div>
              </article>
            ))}
          </div>
        )}
      </section>
    </div>
  )
}
