import { useEffect, useMemo, useState } from 'react'
import { Link, useNavigate } from 'react-router'
import PageHeader from '../components/ui/PageHeader'
import Toasts from '../components/ui/Toasts'
import {
  captureScanRule,
  fetchLLMSettings,
  quickCreateScanRule,
  testDraftMonitor,
} from '../api/monitors'
import type { CaptureResult, ValidationResult } from '../api/types'
import type { ExtractionField, FetchConfig, SourceVariable } from '../lib/monitorForm'
import { useToastMessages } from '../hooks/useToastMessages'

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

function aiDefaultName(url: string) {
  try {
    const host = new URL(url).host.replace(/^www\./, '')
    return `${host} 内容列表`
  } catch {
    return 'AI 生成规则'
  }
}

function draftFetchConfig(draft: RuleDraft): FetchConfig | undefined {
  if (draft.sourceMode !== 'api_json') return undefined
  const variables: Record<string, SourceVariable> = {}
  for (const variable of draft.variables) {
    if (variable.name.trim() && variable.selector.trim()) {
      variables[variable.name.trim()] = {
        source: 'html',
        selector: variable.selector.trim(),
        attr: variable.attr.trim() || undefined,
      }
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

export default function RuleBuilder() {
  const navigate = useNavigate()
  const { successMsg, pageErrorMsg, showSuccess, showError } = useToastMessages()

  const [tab, setTab] = useState<'simple' | 'advanced'>('simple')

  // ———— AI 是否已配置 ————
  const [aiConfigured, setAIConfigured] = useState<boolean | null>(null)
  useEffect(() => {
    let cancelled = false
    fetchLLMSettings()
      .then((res) => {
        if (!cancelled) {
          const ok = Boolean(res.data && res.data.base_url && res.data.api_key && res.data.model)
          setAIConfigured(res.code === 0 && ok)
        }
      })
      .catch(() => {
        if (!cancelled) setAIConfigured(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  // ———— 智能创建（仅两个输入框：URL + 关键词） ————
  const [url, setUrl] = useState('')
  const [keywords, setKeywords] = useState('')
  const [aiRunning, setAIRunning] = useState(false)
  const [aiResult, setAIResult] = useState<CaptureResult | null>(null)
  const [aiError, setAIError] = useState('')
  const [ruleName, setRuleName] = useState('')
  const [saving, setSaving] = useState(false)

  async function handleAIParse() {
    if (aiRunning || !url.trim()) return
    setAIRunning(true)
    setAIError('')
    setAIResult(null)
    try {
      const response = await captureScanRule({ url: url.trim(), keywords: keywords.trim() })
      const result = response.data
      if (!result?.config?.container) {
        throw new Error(result?.message || '未能生成有效的选择器配置')
      }
      setAIResult(result)
      setRuleName((prev) => (prev.trim() ? prev : aiDefaultName(url.trim())))
    } catch (error) {
      setAIError(errorMessage(error))
    } finally {
      setAIRunning(false)
    }
  }

  async function handleSimpleSave() {
    if (!aiResult?.config || saving) return
    const config = aiResult.config
    const name = ruleName.trim() || aiDefaultName(url.trim())
    setSaving(true)
    try {
      await quickCreateScanRule({
        name,
        url: url.trim(),
        scope_type: 'exact',
        config: {
          container: config.container || '',
          item: config.item || '',
          fields:
            config.fields && config.fields.length > 0
              ? config.fields
              : [{ name: 'title', selector: '', type: 'text', attr: '', transform: '' }],
        },
      })
      showSuccess('规则已保存')
      navigate('/rules')
    } catch (error) {
      showError('保存规则失败: ' + errorMessage(error))
    } finally {
      setSaving(false)
    }
  }

  // ———— 手动高级创建 ————
  const [draft, setDraft] = useState<RuleDraft>(createEmptyDraft)
  const [testing, setTesting] = useState(false)
  const [testError, setTestError] = useState('')
  const [testResult, setTestResult] = useState<ValidationResult | null>(null)
  const [savingAdvanced, setSavingAdvanced] = useState(false)

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

  function update<K extends keyof RuleDraft>(key: K, value: RuleDraft[K]) {
    setDraft((prev) => ({ ...prev, [key]: value }))
    setTestResult(null)
    setTestError('')
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
    setDraft((prev) => ({
      ...prev,
      fields: [...prev.fields, { name: '', selector: '', type: 'text', attr: '', transform: '' }],
    }))
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
      const response = await testDraftMonitor({
        url: draft.url.trim(),
        container: draft.sourceMode === 'api_json' ? draft.itemsPath.trim() : draft.container.trim(),
        item: draft.sourceMode === 'api_json' ? '*' : draft.item.trim(),
        fields: draft.fields.filter((f) => f.name.trim()),
        ...(draftFetchConfig(draft) ? { fetch_config: draftFetchConfig(draft) } : {}),
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

  async function handleAdvancedSave() {
    const localError = validateDraft(draft)
    if (localError) {
      setTestError(localError)
      return
    }
    setSavingAdvanced(true)
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
      navigate('/rules')
    } catch (error) {
      showError('保存规则失败: ' + errorMessage(error))
    } finally {
      setSavingAdvanced(false)
    }
  }

  return (
    <div className="scan-rules-page rule-builder-page">
      <Toasts success={successMsg} error={pageErrorMsg} />

      <PageHeader backTo="/rules" title="添加规则" />

      {/* 标签页切换 */}
      <div className="tab-bar" role="tablist" aria-label="创建方式">
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'simple'}
          className={`tab-btn${tab === 'simple' ? ' active' : ''}`}
          onClick={() => setTab('simple')}
        >
          AI 智能创建
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'advanced'}
          className={`tab-btn${tab === 'advanced' ? ' active' : ''}`}
          onClick={() => setTab('advanced')}
        >
          手动高级创建
        </button>
      </div>

      {/* ═══════════════════════ 智能创建 ═══════════════════════ */}
      <section hidden={tab !== 'simple'} aria-label="AI 智能创建规则">
        <div className="settings-section">
          <div className="section-header">
            <h2>只需两步：填写网址，AI 自动识别规则</h2>
            <p className="section-desc">填写页面地址与关键词后点击「智能解析」，AI 会自动分析页面结构并生成提取规则。</p>
          </div>

          {aiConfigured === false && (
            <div className="ai-notice">
              <span>AI 模型尚未配置，智能创建暂不可用。请先在「设置」中完成配置。</span>
              <button className="btn btn-primary btn-sm" onClick={() => navigate('/settings')}>
                去配置
              </button>
            </div>
          )}

          <div className="scan-form">
            <div className="form-group url-field">
              <label htmlFor="ai-url">页面 URL</label>
              <input
                id="ai-url"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                className="form-input"
                placeholder="https://example.com/announcements/"
                inputMode="url"
              />
            </div>
            <div className="form-group">
              <label htmlFor="ai-keywords">
                关键词 <span className="optional-label">辅助 AI 定位</span>
              </label>
              <input
                id="ai-keywords"
                value={keywords}
                onChange={(e) => setKeywords(e.target.value)}
                className="form-input"
                placeholder="正确条目包含的词，如：公告,公示"
                onKeyUp={(e) => e.key === 'Enter' && handleAIParse()}
              />
            </div>
            <button className="btn btn-primary ai-extract-button" disabled={!url.trim() || aiRunning} onClick={handleAIParse}>
              {aiRunning ? '解析中...' : '智能解析'}
            </button>
          </div>

          {aiRunning && (
            <div className="scan-loading">
              <div className="spinner" />
              <span>正在抓取页面并分析结构（可能需要几十秒）</span>
            </div>
          )}

          {aiError && <div className="inline-error">{aiError}</div>}

          {aiResult && (
            <div className="simple-result">
              <div className={`ai-result-preview${aiResult.verified ? '' : ' unverified'}`}>
                <div className="preview-heading">
                  <strong>{aiResult.verified ? '解析成功' : '解析结果'}（识别 {aiResult.diagnostics?.item_count ?? 0} 条）</strong>
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
                  已自动生成规则草稿，请确认后保存；如需精细调整可切换到「手动高级创建」。
                </p>
              </div>

              <div className="simple-save">
                <div className="config-chips">
                  <span className="chip">
                    容器 <code>{aiResult.config.container || '—'}</code>
                  </span>
                  <span className="chip">
                    条目 <code>{aiResult.config.item || '—'}</code>
                  </span>
                  {(aiResult.config.fields || []).map((field) => (
                    <span className="chip" key={field.name}>
                      字段 <code>{field.name}</code>
                    </span>
                  ))}
                </div>
                <div className="simple-save-form">
                  <div className="form-group name-field">
                    <label htmlFor="ai-rule-name">规则名称</label>
                    <input
                      id="ai-rule-name"
                      value={ruleName}
                      onChange={(e) => setRuleName(e.target.value)}
                      className="form-input"
                      placeholder="规则名称"
                    />
                  </div>
                  <button className="btn btn-primary" disabled={saving} onClick={handleSimpleSave}>
                    {saving ? '保存中...' : '保存规则'}
                  </button>
                </div>
                <p className="ai-result-note">默认仅应用于当前页面；如需跨页面匹配，请在高级模式下保存。</p>
              </div>
            </div>
          )}
        </div>
      </section>

      {/* ═══════════════════════ 手动高级创建 ═══════════════════════ */}
      <section hidden={tab !== 'advanced'} aria-label="手动配置规则">
        <div className="settings-section">
          <div className="section-header">
            <h2>手动配置提取规则</h2>
            <p className="section-desc">
              填写 CSS 选择器或 JSON API 路径来定义提取规则。可先用浏览器开发者工具定位选择器，再用「测试提取」验证。
            </p>
          </div>

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
                  <label htmlFor="rule-json-path">列表路径</label>
                  <input
                    id="rule-json-path"
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

          <div className="builder-scope">
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
            <div className="scope-summary">{scopeSummary}</div>
          </div>

          <div className="builder-actions">
            <Link to="/rules" className="btn btn-ghost">
              取消
            </Link>
            <button className="btn btn-primary" disabled={testing || !draft.name.trim()} onClick={handleTest}>
              {testing ? '测试中...' : '测试提取'}
            </button>
            <button className="btn btn-primary" disabled={savingAdvanced || testing || !draft.name.trim()} onClick={handleAdvancedSave}>
              {savingAdvanced ? '保存中...' : '保存规则'}
            </button>
          </div>
        </div>
      </section>
    </div>
  )
}