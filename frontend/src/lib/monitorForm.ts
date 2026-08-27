export type SourceMode = 'html' | 'api_json'
export type NotifyFilter = 'all' | 'keyword'

export interface ExtractionField {
  name: string
  selector: string
  type: string
  attr: string
  transform: string
}

export interface SourceVariable {
  source: string
  selector: string
  attr?: string
}

export interface MonitorFormState {
  basic: {
    name: string
    url: string
    group: string
    /** 允许输入过程中的原始字符串，提交/校验时统一 Number() 转换 */
    interval: number | string
    isActive: boolean
  }
  extraction: {
    sourceMode: SourceMode
    sourceUrl: string
    itemsPath: string
    filterPath: string
    filterEquals: string
    sourceHeaders: Record<string, string>
    sourceVariables: Record<string, SourceVariable>
    containerSelector: string
    itemSelector: string
    fields: ExtractionField[]
  }
  notification: {
    filter: NotifyFilter
    keywords: string
    accountIds: Array<number | string>
  }
}

export interface FetchConfig {
  mode: 'api_json'
  url: string
  items_path: string
  filter_path: string
  filter_equals: string
  headers: Record<string, string>
  variables?: Record<string, SourceVariable>
}

export interface MonitorRequest {
  name: string
  url: string
  container: string
  item: string
  group: string
  check_interval: number
  is_active: boolean
  notify_filter: string
  notify_keywords: string
  notify_account_ids: Array<number | string>
  fields: ExtractionField[]
  fetch_config?: FetchConfig
}

export function createEmptyForm(): MonitorFormState {
  return {
    basic: {
      name: '',
      url: '',
      group: '',
      interval: 3600,
      isActive: true,
    },

    extraction: {
      sourceMode: 'html',
      sourceUrl: '',
      itemsPath: '',
      filterPath: '',
      filterEquals: '',
      sourceHeaders: {},
      sourceVariables: {},
      containerSelector: '',
      itemSelector: '',
      fields: [{ name: 'title', selector: 'a', type: 'text', attr: '', transform: '' }],
    },

    notification: {
      filter: 'all',
      keywords: '',
      accountIds: [],
    },
  }
}

export function suggestMonitorName(url: string) {
  try {
    const hostname = new URL((url || '').trim()).hostname.replace(/^www\./, '')
    return hostname ? `${hostname} 网页` : '未命名网页监控'
  } catch {
    return '未命名网页监控'
  }
}

export function toMonitorRequest(form: MonitorFormState): MonitorRequest {
  const payload: MonitorRequest = {
    name: form.basic.name.trim(),
    url: form.basic.url.trim(),
    container: form.extraction.containerSelector.trim(),
    item: form.extraction.itemSelector.trim(),
    group: form.basic.group.trim(),
    check_interval: Number(form.basic.interval) || 3600,
    is_active: form.basic.isActive,
    notify_filter: form.notification.filter,
    notify_keywords: form.notification.filter === 'keyword' ? form.notification.keywords || '' : '',
    notify_account_ids: form.notification.accountIds || [],
    fields: form.extraction.fields
      .filter((f) => f.name && f.name.trim())
      .map((f) => ({
        name: f.name.trim(),
        selector: (f.selector || '').trim(),
        type: f.type || 'text',
        attr: f.attr || '',
        transform: f.transform || '',
      })),
  }

  if (form.extraction.sourceMode === 'api_json') {
    payload.fetch_config = {
      mode: 'api_json',
      url: form.extraction.sourceUrl.trim(),
      items_path: form.extraction.itemsPath.trim(),
      filter_path: form.extraction.filterPath.trim(),
      filter_equals: String(form.extraction.filterEquals ?? '').trim(),
      headers: form.extraction.sourceHeaders || {},
      ...(Object.keys(form.extraction.sourceVariables || {}).length
        ? { variables: form.extraction.sourceVariables }
        : {}),
    }
  }

  return payload
}

/** 监控器配置响应（编辑模式 GET /monitors/:name/config） */
export interface MonitorConfigResponse {
  name?: string
  url?: string
  group?: string
  check_interval?: number
  is_active?: boolean
  container?: string
  item?: string
  fields?: ExtractionField[]
  notify_filter?: string
  notify_keywords?: string
  notify_account_ids?: Array<number | string>
  fetch_config?: string | FetchConfig
}

export function fromMonitorResponse(data: MonitorConfigResponse): MonitorFormState {
  const form = createEmptyForm()

  form.basic.name = data.name || ''
  form.basic.url = data.url || ''
  form.basic.group = data.group || ''
  form.basic.interval = data.check_interval || 3600
  form.basic.isActive = data.is_active ?? true

  form.extraction.containerSelector = data.container || ''
  form.extraction.itemSelector = data.item || ''
  const fetchConfig = parseJSONValue<FetchConfig>(data.fetch_config)
  if (fetchConfig?.mode === 'api_json') {
    form.extraction.sourceMode = 'api_json'
    form.extraction.sourceUrl = fetchConfig.url || ''
    form.extraction.itemsPath = fetchConfig.items_path || ''
    form.extraction.filterPath = fetchConfig.filter_path || ''
    form.extraction.filterEquals = fetchConfig.filter_equals ?? ''
    form.extraction.sourceHeaders = fetchConfig.headers || {}
    form.extraction.sourceVariables = fetchConfig.variables || {}
  }

  if (data.fields && data.fields.length > 0) {
    form.extraction.fields = data.fields.map((f) => ({
      name: f.name || '',
      selector: f.selector || '',
      type: f.type || 'text',
      attr: f.attr || '',
      transform: f.transform || '',
    }))
  }

  form.notification.filter = (data.notify_filter as NotifyFilter) || 'all'
  form.notification.keywords = data.notify_keywords || ''
  form.notification.accountIds = data.notify_account_ids || []

  return form
}

function parseJSONValue<T>(value: string | T | null | undefined): T | null {
  if (!value) return null
  if (typeof value !== 'string') return value
  try {
    return JSON.parse(value) as T
  } catch {
    return null
  }
}

function detectionDefinition(form: MonitorFormState) {
  const payload = toMonitorRequest(form)
  const fields = [...payload.fields]
    .map((field) => ({
      name: field.name,
      selector: field.selector,
      type: field.type,
      attr: field.attr,
      transform: field.transform,
    }))
    .sort((a, b) => a.name.localeCompare(b.name))

  return {
    url: payload.url,
    container: payload.container,
    item: payload.item,
    fields,
    fetch_config: payload.fetch_config || null,
  }
}

export function getDetectionFingerprint(form: MonitorFormState) {
  return JSON.stringify(detectionDefinition(form))
}

export function hasSemanticChange(original: MonitorFormState, current: MonitorFormState) {
  return getDetectionFingerprint(original) !== getDetectionFingerprint(current)
}

export function validateForm(form: MonitorFormState): string | null {
  if (!form.basic.name.trim()) return '名称不能为空'
  if (!form.basic.url.trim()) return 'URL不能为空'
  try {
    const parsed = new URL(form.basic.url.trim())
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return 'URL仅支持 HTTP 或 HTTPS'
  } catch {
    return 'URL格式无效'
  }
  if (!Number.isFinite(Number(form.basic.interval)) || Number(form.basic.interval) < 10) {
    return '检查间隔不能小于 10 秒'
  }
  if (!form.extraction.containerSelector.trim()) {
    return '还没有匹配到规则：请点击上方「匹配规则」识别内容区域，或先去「规则库」添加规则'
  }
  if (form.extraction.sourceMode === 'api_json') {
    const sourceUrl = form.extraction.sourceUrl.trim().replace(/\{\{\s*[A-Za-z_][A-Za-z0-9_]*\s*\}\}/g, 'value')
    try {
      const parsed = new URL(sourceUrl)
      if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return 'JSON API URL 仅支持 HTTP 或 HTTPS'
    } catch {
      return 'JSON API URL 格式无效'
    }
    if (!form.extraction.itemsPath.trim()) return 'JSON API 列表路径不能为空'
  }
  if (!form.extraction.fields.length) return '至少需要配置一个提取字段'

  const fieldNames = new Set<string>()
  for (const f of form.extraction.fields) {
    if (!f.name.trim()) return '字段名称不能为空'
    const name = f.name.trim()
    if (fieldNames.has(name)) return `字段名称重复: ${name}`
    fieldNames.add(name)
  }

  if (form.notification.filter === 'keyword' && !form.notification.keywords.trim()) {
    return '选择关键词过滤时必须填写推送关键词'
  }

  return null
}
