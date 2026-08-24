import { useRef, useState } from 'react'
import { previewScan } from '@/api/monitors'
import type { ScanContainer } from '@/api/types'
import type { ExtractionField, SourceMode, MonitorFormState } from '@/lib/monitorForm'
import './ExtractionEditor.css'

interface ExtractionEditorProps {
  value: MonitorFormState['extraction']
  url?: string
  advanced?: boolean
  onChange: (value: MonitorFormState['extraction']) => void
}

export default function ExtractionEditor({ value, url = '', advanced = false, onChange }: ExtractionEditorProps) {
  const [scanning, setScanning] = useState(false)
  const [scanResult, setScanResult] = useState<{ containers?: ScanContainer[] } | null>(null)
  const [scanned, setScanned] = useState(false)
  const [scanError, setScanError] = useState<string | null>(null)
  const [selectedIndex, setSelectedIndex] = useState<number | null>(null)

  // 唯一候选会在 await 之后被自动套用，从 ref 取最新值，避免覆盖扫描期间的用户编辑
  const valueRef = useRef(value)
  valueRef.current = value

  async function handleScan() {
    if (scanning || !url) return
    setScanning(true)
    setScanned(false)
    setScanError(null)
    setSelectedIndex(null)
    try {
      const res = await previewScan({ url: url.trim() })
      if (res.code === 0 && res.data) {
        if (res.data.containers?.length === 1) {
          applyCandidate(res.data.containers[0])
          setScanning(false)
          return
        }
        setScanResult(res.data)
      } else {
        setScanError(res.message || '扫描失败')
      }
    } catch (e) {
      setScanError(e instanceof Error ? e.message : '扫描失败')
    }
    setScanned(true)
    setScanning(false)
  }

  function applyCandidate(container: ScanContainer) {
    const config = container.config || {}
    const fields: ExtractionField[] = (config.fields || []).map((f) => ({
      name: f.name || '',
      selector: f.selector || '',
      type: f.type || 'text',
      attr: f.attr || '',
      transform: f.transform || '',
    }))
    const extracted: MonitorFormState['extraction'] = {
      ...valueRef.current,
      containerSelector: config.container || container.container_css || '',
      itemSelector: config.item || container.item_css || '',
    }
    if (config.fetch_config) {
      extracted.sourceMode = (config.fetch_config.mode as SourceMode) || 'html'
      extracted.sourceUrl = config.fetch_config.url || ''
      extracted.itemsPath = config.fetch_config.items_path || ''
      extracted.filterPath = config.fetch_config.filter_path || ''
      extracted.filterEquals = config.fetch_config.filter_equals ?? ''
      extracted.sourceHeaders = config.fetch_config.headers || {}
    } else {
      extracted.sourceMode = 'html'
      extracted.sourceUrl = ''
      extracted.itemsPath = ''
      extracted.filterPath = ''
      extracted.filterEquals = ''
      extracted.sourceHeaders = {}
    }
    if (fields.length > 0) extracted.fields = fields
    onChange(extracted)
    setScanned(false)
    setScanResult(null)
  }

  function update<K extends keyof MonitorFormState['extraction']>(key: K, v: MonitorFormState['extraction'][K]) {
    onChange({ ...value, [key]: v })
  }

  function updateSourceMode(mode: string) {
    const next = { ...value, sourceMode: mode as SourceMode }
    if (mode === 'html') {
      next.sourceUrl = ''
      next.itemsPath = ''
      next.filterPath = ''
      next.filterEquals = ''
      next.sourceHeaders = {}
    }
    onChange(next)
  }

  function updateSourceHeader(name: string, v: string) {
    const headers = { ...(value.sourceHeaders || {}) }
    if (v.trim()) headers[name] = v
    else delete headers[name]
    onChange({ ...value, sourceHeaders: headers })
  }

  function updateField(index: number, key: keyof ExtractionField, v: string) {
    const fields = [...value.fields]
    fields[index] = { ...fields[index], [key]: v }
    onChange({ ...value, fields })
  }

  function addField() {
    const fields = [...value.fields, { name: '', selector: '', type: 'text', attr: '', transform: '' }]
    onChange({ ...value, fields })
  }

  function removeField(index: number) {
    const fields = [...value.fields]
    fields.splice(index, 1)
    onChange({ ...value, fields })
  }

  if (advanced) {
    return (
      <div className="advanced-panel-section extraction-editor">
        <h3>网页提取规则</h3>
        <p className="advanced-section-desc">仅在自动识别不准确时修改这些技术选项。</p>
        <div className="form-group">
          <label>数据源</label>
          <select value={value.sourceMode || 'html'} onChange={(e) => updateSourceMode(e.target.value)} className="form-input">
            <option value="html">网页 HTML</option>
            <option value="api_json">公开 JSON API</option>
          </select>
        </div>

        {value.sourceMode === 'api_json' && (
          <>
            <div className="source-grid">
              <div className="form-group source-url">
                <label>JSON API URL</label>
                <input
                  value={value.sourceUrl}
                  onChange={(e) => update('sourceUrl', e.target.value)}
                  className="form-input"
                  placeholder="https://example.com/api/products"
                />
              </div>
              <div className="form-group">
                <label>列表路径</label>
                <input value={value.itemsPath} onChange={(e) => update('itemsPath', e.target.value)} className="form-input" placeholder="data.items" />
              </div>
              <div className="form-group">
                <label>过滤字段（可选）</label>
                <input value={value.filterPath} onChange={(e) => update('filterPath', e.target.value)} className="form-input" placeholder="is_selling" />
              </div>
              <div className="form-group">
                <label>过滤值</label>
                <input value={value.filterEquals} onChange={(e) => update('filterEquals', e.target.value)} className="form-input" placeholder="true" />
              </div>
            </div>
            <div className="header-grid">
              <div className="form-group">
                <label>Accept</label>
                <input
                  value={value.sourceHeaders?.Accept || ''}
                  onChange={(e) => updateSourceHeader('Accept', e.target.value)}
                  className="form-input"
                  placeholder="application/json"
                />
              </div>
              <div className="form-group">
                <label>Accept-Language</label>
                <input
                  value={value.sourceHeaders?.['Accept-Language'] || ''}
                  onChange={(e) => updateSourceHeader('Accept-Language', e.target.value)}
                  className="form-input"
                  placeholder="zh-CN,zh;q=0.9"
                />
              </div>
              <div className="form-group">
                <label>Referer</label>
                <input
                  value={value.sourceHeaders?.Referer || ''}
                  onChange={(e) => updateSourceHeader('Referer', e.target.value)}
                  className="form-input"
                  placeholder={url || 'https://example.com/product'}
                />
              </div>
              <div className="form-group">
                <label>X-Requested-With</label>
                <input
                  value={value.sourceHeaders?.['X-Requested-With'] || ''}
                  onChange={(e) => updateSourceHeader('X-Requested-With', e.target.value)}
                  className="form-input"
                  placeholder="XMLHttpRequest"
                />
              </div>
            </div>
          </>
        )}

        <div className="form-group">
          <label>{value.sourceMode === 'api_json' ? '列表路径（兼容字段）' : '容器选择器'}</label>
          <input
            value={value.containerSelector}
            onChange={(e) => update('containerSelector', e.target.value)}
            className="form-input"
            placeholder="如 div.hap_infoBox"
          />
        </div>

        <div className="form-group">
          <label>{value.sourceMode === 'api_json' ? '条目表达式' : '列表项选择器（可选）'}</label>
          <input
            value={value.itemSelector}
            onChange={(e) => update('itemSelector', e.target.value)}
            className="form-input"
            placeholder="如 div.hap_infoOne"
          />
        </div>

        <div className="fields-section">
          <div className="fields-header">
            <span className="fields-title">提取字段</span>
            <button className="btn btn-sm btn-ghost" onClick={addField}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
                <line x1="12" y1="5" x2="12" y2="19" />
                <line x1="5" y1="12" x2="19" y2="12" />
              </svg>
              添加字段
            </button>
          </div>

          <div className="fields-list">
            {value.fields.map((field, index) => (
              <div key={index} className="field-card">
                <div className="field-grid">
                  <div className="form-group">
                    <label>名称</label>
                    <input
                      value={field.name}
                      onChange={(e) => updateField(index, 'name', e.target.value)}
                      className="form-input"
                      placeholder="如 title"
                    />
                  </div>
                  <div className="form-group">
                    <label>{value.sourceMode === 'api_json' ? 'JSON 路径' : '选择器'}</label>
                    <input
                      value={field.selector}
                      onChange={(e) => updateField(index, 'selector', e.target.value)}
                      className="form-input"
                      placeholder="如 a.title"
                    />
                  </div>
                  <div className="form-group">
                    <label>类型</label>
                    <select value={field.type} onChange={(e) => updateField(index, 'type', e.target.value)} className="form-input">
                      <option value="text">文本 (text)</option>
                      <option value="attr">属性 (attr)</option>
                    </select>
                  </div>
                  {field.type === 'attr' && (
                    <div className="form-group">
                      <label>属性名</label>
                      <input
                        value={field.attr}
                        onChange={(e) => updateField(index, 'attr', e.target.value)}
                        className="form-input"
                        placeholder="默认 href"
                      />
                    </div>
                  )}
                  <div className="form-group">
                    <label>转换</label>
                    <input
                      value={field.transform}
                      onChange={(e) => updateField(index, 'transform', e.target.value)}
                      className="form-input"
                      placeholder="如 trim([])"
                    />
                  </div>
                </div>
                <button className="btn-icon btn-icon-danger field-remove-btn" title="删除字段" onClick={() => removeField(index)}>
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
                    <line x1="18" y1="6" x2="6" y2="18" />
                    <line x1="6" y1="6" x2="18" y2="18" />
                  </svg>
                </button>
              </div>
            ))}
          </div>

          {value.fields.length === 0 && (
            <div className="empty">
              <p>暂无字段，点击上方按钮添加</p>
            </div>
          )}
        </div>
      </div>
    )
  }

  return (
    <div className="settings-section extraction-editor">
      <div className="ee-section-header">
        <h2>想关注什么内容？</h2>
      </div>

      <p className="section-desc">
        系统按「高级规则」页保存的扫描规则识别内容区域；未命中规则时，可在高级设置中手动配置并另存为规则。
      </p>

      <div className="scan-row">
        <div className="scan-action">
          <button className="btn btn-primary btn-sm" disabled={!url || scanning} onClick={handleScan}>
            {scanning ? '识别中...' : '按规则识别内容'}
          </button>
        </div>
      </div>

      {scanning && (
        <div className="loading">
          <div className="spinner" />
          <p>正在识别网页内容...</p>
        </div>
      )}

      {!scanning && scanResult && scanResult.containers && scanResult.containers.length > 0 && (
        <div className="scan-results">
          <p className="results-label">
            命中 {scanResult.containers.length} 条扫描规则，找到以下内容区域，请选择最符合的一项：
          </p>
          {scanResult.containers.map((container, ci) => (
            <div
              key={ci}
              className={`candidate-card${selectedIndex === ci ? ' selected' : ''}`}
              onClick={() => setSelectedIndex(ci)}
            >
              <div className="candidate-header">
                <div className="candidate-info">
                  <span className="candidate-badge">{container.strategy || `内容区域 ${ci + 1}`}</span>
                  <span className="candidate-count">发现 {container.item_count} 条内容</span>
                </div>
                <button
                  className="btn btn-sm btn-primary apply-btn"
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation()
                    applyCandidate(container)
                  }}
                >
                  选择此区域
                </button>
              </div>
              <div className="candidate-samples">
                {(container.sample_items || []).slice(0, 5).map((item, ii) => (
                  <div key={ii} className="sample-item">
                    <span className="sample-title">{item.title || '未命名条目'}</span>
                    {item.date ? <span className="sample-meta">{item.date}</span> : null}
                  </div>
                ))}
                {container.item_count > 5 && <div className="sample-more">...还有 {container.item_count - 5} 条</div>}
              </div>
            </div>
          ))}
        </div>
      )}

      {!scanning && !scanResult?.containers?.length && scanned && (
        <div className="empty-scan">
          <p>该网址未命中任何已保存的扫描规则。请先在「高级规则」页创建规则，或在高级设置中手动配置。</p>
        </div>
      )}

      {scanError && <div className="form-error">{scanError}</div>}

      {value.containerSelector && (
        <div className="selection-confirmed">
          <div className="confirmed-content">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
              <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
              <polyline points="22 4 12 14.01 9 11.01" />
            </svg>
            <span>已识别内容区域，系统将监控新出现的内容</span>
          </div>
        </div>
      )}
    </div>
  )
}
