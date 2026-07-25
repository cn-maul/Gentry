import test from 'node:test'
import assert from 'node:assert/strict'

import {
  buildPriceRuleConfig,
  buildPriceRuleValidationRequest,
  createEmptyPriceRuleDraft,
  validatePriceRuleDraft,
} from '../src/composables/usePriceScanRuleBuilder.js'

test('price API rule builder creates a shared scan-rule config', () => {
  const draft = createEmptyPriceRuleDraft()
  draft.apiUrl = 'https://shop.example/api/skus?id=31'
  draft.headers.Referer = 'https://shop.example/products/item'
  draft.headers['Accept-Language'] = '  '

  assert.equal(validatePriceRuleDraft('https://shop.example/products/item', draft), '')
  const config = buildPriceRuleConfig(draft)
  assert.deepEqual(config.fetch_config, {
    mode: 'api_json',
    url: draft.apiUrl,
    items_path: 'data',
    filter_path: 'is_selling',
    filter_equals: 'true',
    headers: {
      Accept: 'application/json, text/javascript, */*; q=0.01',
      Referer: 'https://shop.example/products/item',
      'X-Requested-With': 'XMLHttpRequest',
    },
  })
  assert.deepEqual(config.fields.map(field => field.name), ['title', 'sku', 'price', 'original_price'])
})

test('price API rule validation request uses SKU identity and price semantics', () => {
  const draft = createEmptyPriceRuleDraft()
  draft.apiUrl = 'https://shop.example/api/skus'
  const payload = buildPriceRuleValidationRequest('https://shop.example/products/item', draft)

  assert.equal(payload.strategy_type, 'field_transition')
  assert.deepEqual(payload.strategy_config.identity, { field: 'sku' })
  assert.equal(payload.strategy_config.conditions[0].field, 'price')
  assert.equal(payload.field_data_types.price, 'money')
})

test('price API rule serializes reusable page variables', () => {
  const draft = createEmptyPriceRuleDraft()
  draft.apiUrl = 'https://shop.example/api/skus?goods_id={{goods_id}}'
  draft.variables.push({ name: 'goods_id', selector: '#goods_id', attr: 'value' })

  assert.equal(validatePriceRuleDraft('https://shop.example/products/item', draft), '')
  const fetchConfig = buildPriceRuleConfig(draft).fetch_config
  assert.equal(fetchConfig.headers.Referer, '{{page_url}}')
  assert.deepEqual(fetchConfig.variables, {
    goods_id: { source: 'html', selector: '#goods_id', attr: 'value' },
  })
})

test('price API rule rejects unknown template variables', () => {
  const draft = createEmptyPriceRuleDraft()
  draft.apiUrl = 'https://shop.example/api/skus?id={{missing_id}}'
  assert.match(validatePriceRuleDraft('https://shop.example/products/item', draft), /未配置/)
})

test('price API rule rejects variables in the destination host', () => {
  const draft = createEmptyPriceRuleDraft()
  draft.apiUrl = 'https://{{host}}/api/skus'
  draft.variables.push({ name: 'host', selector: '#host', attr: 'value' })
  assert.match(validatePriceRuleDraft('https://shop.example/products/item', draft), /协议或主机/)
})
