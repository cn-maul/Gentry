const controlledHeaders = ['Accept', 'Accept-Language', 'Referer', 'X-Requested-With']

export function createEmptyPriceRuleDraft() {
  return {
    apiUrl: '',
    itemsPath: 'data',
    filterPath: 'is_selling',
    filterEquals: 'true',
    headers: {
      Accept: 'application/json, text/javascript, */*; q=0.01',
      'Accept-Language': '',
      Referer: '{{page_url}}',
      'X-Requested-With': 'XMLHttpRequest',
    },
    variables: [],
    titlePath: 'spec_array.*.value',
    identityPath: 'products_no',
    pricePath: 'sell_price',
    originalPricePath: 'original_price',
  }
}

function normalizedVariables(variables = []) {
  const result = {}
  for (const variable of variables) {
    const name = String(variable.name || '').trim()
    if (!name) continue
    result[name] = {
      source: 'html',
      selector: String(variable.selector || '').trim(),
      ...(String(variable.attr || '').trim() ? { attr: String(variable.attr).trim() } : {}),
    }
  }
  return result
}

function normalizedHeaders(headers = {}) {
  const result = {}
  for (const name of controlledHeaders) {
    const value = String(headers[name] || '').trim()
    if (value) result[name] = value
  }
  return result
}

export function buildPriceRuleConfig(draft) {
  const variables = normalizedVariables(draft.variables)
  const fields = [
    { name: 'title', selector: draft.titlePath.trim(), type: 'text' },
    { name: 'sku', selector: draft.identityPath.trim(), type: 'text' },
    { name: 'price', selector: draft.pricePath.trim(), type: 'text' },
  ]
  if (draft.originalPricePath.trim()) {
    fields.push({ name: 'original_price', selector: draft.originalPricePath.trim(), type: 'text' })
  }
  return {
    container: draft.itemsPath.trim(),
    item: '*',
    fetch_config: {
      mode: 'api_json',
      url: draft.apiUrl.trim(),
      items_path: draft.itemsPath.trim(),
      filter_path: draft.filterPath.trim(),
      filter_equals: String(draft.filterEquals ?? '').trim(),
      headers: normalizedHeaders(draft.headers),
      ...(Object.keys(variables).length ? { variables } : {}),
    },
    fields,
  }
}

export function validatePriceRuleDraft(pageUrl, draft) {
  for (const [label, value] of [['页面 URL', pageUrl]]) {
    try {
      const parsed = new URL(String(value || '').trim())
      if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return `${label} 仅支持 HTTP 或 HTTPS`
    } catch {
      return `${label} 格式无效`
    }
  }
  const variableNames = new Set()
  for (const variable of draft.variables || []) {
    const name = String(variable.name || '').trim()
    if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(name)) return '动态参数名只能包含字母、数字和下划线，且不能以数字开头'
    if (variableNames.has(name)) return `动态参数 ${name} 重复`
    if (!String(variable.selector || '').trim()) return `动态参数 ${name} 的页面选择器不能为空`
    variableNames.add(name)
  }
  const apiTemplate = String(draft.apiUrl || '').trim()
  const templatePattern = /\{\{\s*[A-Za-z_][A-Za-z0-9_]*\s*\}\}/g
  const firstTemplateIndex = apiTemplate.search(templatePattern)
  const authorityStart = apiTemplate.indexOf('://')
  const authorityTail = authorityStart >= 0 ? apiTemplate.slice(authorityStart + 3) : ''
  const pathOffset = authorityTail.search(/[/?#]/)
  const authorityEnd = pathOffset >= 0 ? authorityStart + 3 + pathOffset : apiTemplate.length
  if (firstTemplateIndex >= 0 && firstTemplateIndex <= authorityEnd) return '动态参数不能用于 JSON API URL 的协议或主机'
  const validationUrl = apiTemplate.replace(templatePattern, 'value')
  if (validationUrl.includes('{{') || validationUrl.includes('}}')) return 'JSON API URL 中的动态参数格式无效'
  try {
    const parsed = new URL(validationUrl)
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') return 'JSON API URL 仅支持 HTTP 或 HTTPS'
  } catch {
    return 'JSON API URL 格式无效'
  }
  const knownVariables = new Set(['page_url', ...variableNames])
  for (const template of [apiTemplate, ...Object.values(draft.headers || {})]) {
    for (const match of String(template || '').matchAll(/\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}/g)) {
      if (!knownVariables.has(match[1])) return `动态参数 ${match[1]} 未配置`
    }
  }
  if (!draft.itemsPath.trim()) return '列表路径不能为空'
  if (!draft.titlePath.trim()) return '标题路径不能为空'
  if (!draft.identityPath.trim()) return '商品身份路径不能为空'
  if (!draft.pricePath.trim()) return '价格路径不能为空'
  return ''
}

export function buildPriceRuleValidationRequest(pageUrl, draft) {
  const config = buildPriceRuleConfig(draft)
  return {
    name: 'price-rule-preview',
    url: pageUrl.trim(),
    container: config.container,
    item: config.item,
    check_interval: 3600,
    is_active: false,
    fields: config.fields,
    strategy_type: 'field_transition',
    strategy_config: {
      type: 'field_transition',
      identity: { field: 'sku' },
      conditions: [{ field: 'price', value_type: 'money', operator: 'decreased' }],
      on_first_baseline: 'silent',
    },
    field_data_types: { price: 'money' },
    fetch_config: config.fetch_config,
  }
}

export function priceRuleFingerprint(pageUrl, draft) {
  return JSON.stringify({ pageUrl: pageUrl.trim(), config: buildPriceRuleConfig(draft) })
}
