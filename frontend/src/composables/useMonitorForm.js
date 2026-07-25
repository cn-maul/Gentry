export function createEmptyForm() {
  return {
    basic: {
      name: '',
      url: '',
      group: '',
      interval: 3600,
      isActive: true,
    },

    monitorType: 'presence',

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

    rule: {
      pageMode: 'single',
      identity: {
        mode: 'source_url',
        field: '',
      },
      target: {
        field: 'price',
        valueType: 'money',
      },
      transition: {
        operator: 'decreased',
        minAmount: '',
        minPercent: '',
        targetPrice: '',
      },
    },

    notification: {
      filter: 'all',
      keywords: '',
      accountIds: [],
    },
  }
}

export function toMonitorRequest(form) {
  const payload = {
    name: form.basic.name.trim(),
    url: form.basic.url.trim(),
    container: form.extraction.containerSelector.trim(),
    item: form.extraction.itemSelector.trim(),
    group: form.basic.group.trim(),
    check_interval: form.basic.interval || 3600,
    is_active: form.basic.isActive,
    strategy_type: form.monitorType,
    notify_filter: form.notification.filter,
    notify_keywords: form.notification.filter === 'keyword' ? (form.notification.keywords || '') : '',
    notify_account_ids: form.notification.accountIds || [],
    fields: form.extraction.fields.filter(f => f.name && f.name.trim()).map(f => ({
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
      ...(Object.keys(form.extraction.sourceVariables || {}).length ? { variables: form.extraction.sourceVariables } : {}),
    }
  }

  if (form.monitorType === 'field_transition') {
    const identity = form.rule.identity.mode === 'source_url'
      ? { source: 'source_url' }
      : { field: form.rule.identity.field }

    const operator = form.rule.transition.operator || 'decreased'
    const threshold = {}
    if (operator === 'at_or_below') {
      if (form.rule.transition.targetPrice !== '' && form.rule.transition.targetPrice !== null) {
        threshold.value = String(form.rule.transition.targetPrice)
      }
    } else {
      if (form.rule.transition.minAmount !== '' && form.rule.transition.minAmount !== null && Number(form.rule.transition.minAmount) > 0) {
        threshold.amount = String(form.rule.transition.minAmount)
      }
      if (form.rule.transition.minPercent !== '' && form.rule.transition.minPercent !== null && Number(form.rule.transition.minPercent) > 0) {
        threshold.percent = Number(form.rule.transition.minPercent)
      }
    }

    const condition = {
      field: form.rule.target.field || 'price',
      value_type: 'money',
      operator,
    }
    if (Object.keys(threshold).length > 0) {
      condition.threshold = threshold
    }

    payload.strategy_config = {
      type: 'field_transition',
      identity: identity,
      conditions: [condition],
      on_first_baseline: 'silent',
    }

    if (form.rule.target.field) {
      payload.field_data_types = {
        [form.rule.target.field]: 'money',
      }
    }
  }

  return payload
}

export function fromMonitorResponse(data) {
  const form = createEmptyForm()

  form.basic.name = data.name || ''
  form.basic.url = data.url || ''
  form.basic.group = data.group || ''
  form.basic.interval = data.check_interval || 3600
  form.basic.isActive = data.is_active ?? true

  form.monitorType = data.strategy_type || 'presence'

  form.extraction.containerSelector = data.container || ''
  form.extraction.itemSelector = data.item || ''
  form.rule.pageMode = data.item ? 'list' : 'single'
  const fetchConfig = parseJSONValue(data.fetch_config)
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
    form.extraction.fields = data.fields.map(f => ({
      name: f.name || '',
      selector: f.selector || '',
      type: f.type || 'text',
      attr: f.attr || '',
      transform: f.transform || '',
    }))
  }

  form.notification.filter = data.notify_filter || 'all'
  form.notification.keywords = data.notify_keywords || ''
  form.notification.accountIds = data.notify_account_ids || []

  if (data.strategy_config) {
    const sc = parseJSONValue(data.strategy_config)
    if (sc && sc.identity) {
      if (sc.identity.source === 'source_url') {
        form.rule.identity.mode = 'source_url'
        form.rule.identity.field = ''
      } else if (sc.identity.field) {
        form.rule.identity.mode = 'field'
        form.rule.identity.field = sc.identity.field
      }
    }
    if (sc && sc.conditions && sc.conditions.length > 0) {
      const cond = sc.conditions[0]
      form.rule.target.field = cond.field || 'price'
      form.rule.target.valueType = cond.value_type || 'money'
      form.rule.transition.operator = cond.operator || 'decreased'
      if (cond.threshold) {
        form.rule.transition.minAmount = cond.threshold.amount || ''
        form.rule.transition.minPercent = cond.threshold.percent || ''
        form.rule.transition.targetPrice = cond.threshold.value || ''
      }
    }
  }

  const fieldDataTypes = parseJSONValue(data.field_data_types)
  if (fieldDataTypes && typeof fieldDataTypes === 'object') {
    const keys = Object.keys(fieldDataTypes)
    if (keys.length > 0 && !form.rule.target.field) {
      form.rule.target.field = keys[0]
    }
    if (keys.length > 0 && fieldDataTypes[keys[0]]) {
      form.rule.target.valueType = fieldDataTypes[keys[0]]
    }
  }

  return form
}

function parseJSONValue(value) {
  if (!value || typeof value !== 'string') return value
  try {
    return JSON.parse(value)
  } catch {
    return null
  }
}

function detectionDefinition(form) {
  const payload = toMonitorRequest(form)
  const fields = [...payload.fields]
    .map(field => ({
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
    strategy_type: payload.strategy_type,
    strategy_config: payload.strategy_config || null,
    field_data_types: payload.field_data_types || {},
    fetch_config: payload.fetch_config || null,
  }
}

export function getDetectionFingerprint(form) {
  return JSON.stringify(detectionDefinition(form))
}

export function hasSemanticChange(original, current) {
  return getDetectionFingerprint(original) !== getDetectionFingerprint(current)
}

export function validateForm(form) {
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
  if (!form.extraction.containerSelector.trim()) return '容器选择器不能为空'
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

  const fieldNames = new Set()
  for (const f of form.extraction.fields) {
    if (!f.name.trim()) return '字段名称不能为空'
    const name = f.name.trim()
    if (fieldNames.has(name)) return `字段名称重复: ${name}`
    fieldNames.add(name)
  }

  if (form.monitorType === 'field_transition') {
    if (!form.rule.target.field.trim()) return '监控字段名称不能为空'
    if (!fieldNames.has(form.rule.target.field.trim())) return '监控字段必须存在于提取字段中'
    if (form.rule.target.valueType !== 'money') return '当前价格监控仅支持金额类型'
    if (!['decreased', 'at_or_below'].includes(form.rule.transition.operator)) return '不支持的价格触发条件'
    if (form.rule.pageMode === 'list' && !form.extraction.itemSelector.trim()) {
      return '商品列表页必须配置列表项选择器'
    }
    if ((form.rule.pageMode === 'list' || form.extraction.itemSelector.trim()) && form.rule.identity.mode !== 'field') {
      return '商品列表页必须使用稳定且唯一的字段作为商品身份'
    }
    if (form.rule.identity.mode === 'field' && !form.rule.identity.field.trim()) {
      return '商品列表页必须指定商品身份字段'
    }
    if (form.rule.identity.mode === 'field' && !fieldNames.has(form.rule.identity.field.trim())) {
      return '商品身份字段必须存在于提取字段中'
    }
    if (form.rule.identity.mode === 'field' && form.rule.identity.field.trim() === form.rule.target.field.trim()) {
      return '商品身份字段不能使用会发生变化的价格字段'
    }
    if (form.rule.transition.operator === 'at_or_below') {
      const target = Number(form.rule.transition.targetPrice)
      if (form.rule.transition.targetPrice === '' || !Number.isFinite(target) || target < 0) {
        return '目标价格必须为有效的非负数'
      }
    } else {
      const amt = Number(form.rule.transition.minAmount)
      const pct = Number(form.rule.transition.minPercent)
      if (form.rule.transition.minAmount !== '' && (isNaN(amt) || amt < 0)) {
        return '降价金额必须为非负数'
      }
      if (form.rule.transition.minPercent !== '' && (isNaN(pct) || pct < 0 || pct > 100)) {
        return '降价百分比必须在 0-100 之间'
      }
    }
  }

  if (form.notification.filter === 'keyword' && !form.notification.keywords.trim()) {
    return '选择关键词过滤时必须填写推送关键词'
  }

  return null
}
