import test from 'node:test'
import assert from 'node:assert/strict'

import {
  createEmptyForm,
  fromMonitorResponse,
  getDetectionFingerprint,
  hasSemanticChange,
  suggestMonitorName,
  suggestedScanRuleScope,
  toMonitorRequest,
  validateForm,
} from '../src/composables/useMonitorForm.js'

test('suggests a readable name from the URL when the user leaves it blank', () => {
  assert.equal(suggestMonitorName('https://www.example.com/products/1', 'field_transition'), 'example.com 价格')
  assert.equal(suggestMonitorName('https://notice.example.cn/list', 'presence'), 'notice.example.cn 网页')
  assert.equal(suggestMonitorName('', 'presence'), '未命名网页监控')
})

test('suggests a reusable directory scope for product price rules', () => {
  assert.equal(suggestedScanRuleScope('https://lizhi.shop/products/adguard', 'field_transition'), 'section')
  assert.equal(suggestedScanRuleScope('https://example.com/product', 'field_transition'), 'exact')
  assert.equal(suggestedScanRuleScope('https://lizhi.shop/products/adguard', 'presence'), 'exact')
})

function validPriceForm() {
  const form = createEmptyForm()
  form.basic.name = '商品价格'
  form.basic.url = 'https://example.com/product/1'
  form.extraction.containerSelector = 'body'
  form.extraction.fields = [
    { name: 'title', selector: 'h1', type: 'text', attr: '', transform: '' },
    { name: 'price', selector: '.price', type: 'text', attr: '', transform: '' },
  ]
  form.monitorType = 'field_transition'
  return form
}

test('price monitor DTO round-trips through backend-shaped response', () => {
  const form = validPriceForm()
  form.rule.transition.minAmount = '10.50'
  form.rule.transition.minPercent = '5'

  const payload = toMonitorRequest(form)
  assert.equal(payload.strategy_config.conditions[0].value_type, 'money')
  assert.equal(payload.strategy_config.conditions[0].operator, 'decreased')
  assert.deepEqual(payload.strategy_config.conditions[0].threshold, { amount: '10.50', percent: 5 })

  const restored = fromMonitorResponse({
    ...payload,
    strategy_config: JSON.stringify(payload.strategy_config),
    field_data_types: JSON.stringify(payload.field_data_types),
  })
  assert.equal(restored.monitorType, 'field_transition')
  assert.equal(restored.rule.identity.mode, 'source_url')
  assert.equal(restored.rule.target.field, 'price')
  assert.equal(restored.rule.target.valueType, 'money')
  assert.equal(restored.rule.transition.minAmount, '10.50')
  assert.equal(restored.rule.transition.minPercent, 5)
})

test('semantic changes include extraction selectors and thresholds', () => {
  const original = validPriceForm()
  const selectorChanged = structuredClone(original)
  selectorChanged.extraction.fields[1].selector = '.sale-price'
  assert.equal(hasSemanticChange(original, selectorChanged), true)

  const thresholdChanged = structuredClone(original)
  thresholdChanged.rule.transition.minAmount = '20'
  assert.equal(hasSemanticChange(original, thresholdChanged), true)

  const displayOnlyChanged = structuredClone(original)
  displayOnlyChanged.basic.name = '新名称'
  displayOnlyChanged.notification.accountIds = [1, 2]
  assert.equal(getDetectionFingerprint(original), getDetectionFingerprint(displayOnlyChanged))
})

test('list price monitors require an item selector and field identity', () => {
  const form = validPriceForm()
  form.rule.pageMode = 'list'
  assert.match(validateForm(form), /列表项选择器/)

  form.extraction.itemSelector = '.product'
  assert.match(validateForm(form), /字段作为商品身份/)

  form.rule.identity = { mode: 'field', field: 'sku' }
  assert.match(validateForm(form), /商品身份字段必须存在/)

  form.extraction.fields.push({ name: 'sku', selector: '[data-sku]', type: 'attr', attr: 'data-sku', transform: '' })
  assert.equal(validateForm(form), null)

  form.rule.identity.field = 'price'
  assert.match(validateForm(form), /不能使用会发生变化的价格字段/)
})

test('unsupported numeric types are rejected before submission', () => {
  const form = validPriceForm()
  form.rule.target.valueType = 'decimal'
  assert.match(validateForm(form), /仅支持金额类型/)
})

test('target price rule DTO round-trips through backend-shaped response', () => {
  const form = validPriceForm()
  form.rule.transition.operator = 'at_or_below'
  form.rule.transition.targetPrice = '199.00'

  const payload = toMonitorRequest(form)
  assert.deepEqual(payload.strategy_config.conditions[0], {
    field: 'price',
    value_type: 'money',
    operator: 'at_or_below',
    threshold: { value: '199.00' },
  })

  const restored = fromMonitorResponse({
    ...payload,
    strategy_config: JSON.stringify(payload.strategy_config),
    field_data_types: JSON.stringify(payload.field_data_types),
  })
  assert.equal(restored.rule.transition.operator, 'at_or_below')
  assert.equal(restored.rule.transition.targetPrice, '199.00')
})

test('target price rule validates its boundary value', () => {
  const form = validPriceForm()
  form.rule.transition.operator = 'at_or_below'

  assert.match(validateForm(form), /目标价格必须为有效的非负数/)
  form.rule.transition.targetPrice = '-1'
  assert.match(validateForm(form), /目标价格必须为有效的非负数/)
  form.rule.transition.targetPrice = '0'
  assert.equal(validateForm(form), null)
})

test('target price changes are semantic changes', () => {
  const original = validPriceForm()
  original.rule.transition.operator = 'at_or_below'
  original.rule.transition.targetPrice = '199'

  const changed = structuredClone(original)
  changed.rule.transition.targetPrice = '189'
  assert.equal(hasSemanticChange(original, changed), true)
})

test('JSON API price source round-trips and participates in validation', () => {
  const form = validPriceForm()
  form.extraction.sourceMode = 'api_json'
  form.extraction.sourceUrl = 'https://shop.example/api/skus?id=31'
  form.extraction.itemsPath = 'data'
  form.extraction.filterPath = 'is_selling'
  form.extraction.filterEquals = 'true'
  form.extraction.sourceHeaders = {
    Accept: 'application/json',
    Referer: 'https://shop.example/products/31',
    'X-Requested-With': 'XMLHttpRequest',
  }
  form.extraction.sourceUrl = 'https://shop.example/api/skus?id={{goods_id}}'
  form.extraction.sourceHeaders.Referer = '{{page_url}}'
  form.extraction.sourceVariables = {
    goods_id: { source: 'html', selector: '#goods_id', attr: 'value' },
  }
  form.extraction.containerSelector = 'data'
  form.extraction.itemSelector = '*'
  form.extraction.fields = [
    { name: 'title', selector: 'spec_array.*.value', type: 'text', attr: '', transform: '' },
    { name: 'sku', selector: 'products_no', type: 'text', attr: '', transform: '' },
    { name: 'price', selector: 'sell_price', type: 'text', attr: '', transform: '' },
  ]
  form.rule.pageMode = 'list'
  form.rule.identity = { mode: 'field', field: 'sku' }

  assert.equal(validateForm(form), null)
  const payload = toMonitorRequest(form)
  assert.deepEqual(payload.fetch_config, {
    mode: 'api_json',
    url: 'https://shop.example/api/skus?id={{goods_id}}',
    items_path: 'data',
    filter_path: 'is_selling',
    filter_equals: 'true',
    headers: {
      Accept: 'application/json',
      Referer: '{{page_url}}',
      'X-Requested-With': 'XMLHttpRequest',
    },
    variables: {
      goods_id: { source: 'html', selector: '#goods_id', attr: 'value' },
    },
  })

  const restored = fromMonitorResponse({
    ...payload,
    fetch_config: JSON.stringify(payload.fetch_config),
    strategy_config: JSON.stringify(payload.strategy_config),
    field_data_types: JSON.stringify(payload.field_data_types),
  })
  assert.equal(restored.extraction.sourceMode, 'api_json')
  assert.equal(restored.extraction.sourceUrl, payload.fetch_config.url)
  assert.equal(restored.extraction.itemsPath, 'data')
  assert.deepEqual(restored.extraction.sourceHeaders, payload.fetch_config.headers)
  assert.deepEqual(restored.extraction.sourceVariables, payload.fetch_config.variables)
  assert.equal(restored.rule.identity.field, 'sku')
})
