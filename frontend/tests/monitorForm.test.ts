import { describe, expect, it } from 'vitest'
import {
  createEmptyForm,
  fromMonitorResponse,
  getDetectionFingerprint,
  hasSemanticChange,
  suggestMonitorName,
  toMonitorRequest,
  validateForm,
} from '../src/lib/monitorForm'

describe('suggestMonitorName', () => {
  it('derives a name from the hostname', () => {
    expect(suggestMonitorName('https://www.example.com/news')).toBe('example.com 网页')
  })

  it('falls back for invalid URLs', () => {
    expect(suggestMonitorName('not a url')).toBe('未命名网页监控')
  })
})

describe('toMonitorRequest / fromMonitorResponse round trip', () => {
  it('serializes html extraction config', () => {
    const form = createEmptyForm()
    form.basic.name = '公告列表'
    form.basic.url = 'https://example.com/news'
    form.extraction.containerSelector = 'ul.news-list'
    form.extraction.itemSelector = 'li'
    form.extraction.fields = [
      { name: 'title', selector: 'a', type: 'text', attr: '', transform: '' },
      { name: 'url', selector: 'a', type: 'attr', attr: 'href', transform: '' },
    ]

    const payload = toMonitorRequest(form)
    expect(payload.name).toBe('公告列表')
    expect(payload.container).toBe('ul.news-list')
    expect(payload.item).toBe('li')
    expect(payload.fields).toHaveLength(2)
    expect(payload.fetch_config).toBeUndefined()
  })

  it('round trips a JSON API source through fromMonitorResponse', () => {
    const form = createEmptyForm()
    form.extraction.sourceMode = 'api_json'
    form.extraction.sourceUrl = 'https://shop.example/api/list'
    form.extraction.itemsPath = 'data.items'
    form.extraction.filterPath = 'is_selling'
    form.extraction.filterEquals = 'true'
    form.extraction.sourceHeaders = { Accept: 'application/json' }
    form.extraction.sourceVariables = { goods_id: { source: 'html', selector: '#goods_id', attr: 'value' } }
    form.extraction.containerSelector = 'data.items'
    form.extraction.itemSelector = '*'

    const restored = fromMonitorResponse({
      name: 'shop-list',
      url: 'https://shop.example/list',
      container: 'data.items',
      item: '*',
      fields: [{ name: 'title', selector: 'name', type: 'text', attr: '', transform: '' }],
      fetch_config: toMonitorRequest(form).fetch_config,
    })

    expect(restored.extraction.sourceMode).toBe('api_json')
    expect(restored.extraction.itemsPath).toBe('data.items')
    expect(restored.extraction.sourceVariables.goods_id.selector).toBe('#goods_id')
    expect(restored.extraction.fields[0].name).toBe('title')
  })
})

describe('detection fingerprint', () => {
  it('changes when extraction semantics change', () => {
    const base = createEmptyForm()
    base.basic.url = 'https://example.com'
    base.extraction.containerSelector = 'ul.list'

    const changed = { ...base, extraction: { ...base.extraction, containerSelector: 'ul.other' } }
    expect(hasSemanticChange(base, changed)).toBe(true)
    expect(hasSemanticChange(base, { ...base, basic: { ...base.basic, name: '改名' } })).toBe(false)
  })

  it('is insensitive to field order', () => {
    const a = createEmptyForm()
    a.extraction.fields = [
      { name: 'title', selector: 'a', type: 'text', attr: '', transform: '' },
      { name: 'url', selector: 'a', type: 'attr', attr: 'href', transform: '' },
    ]
    const b = createEmptyForm()
    b.extraction.fields = [
      { name: 'url', selector: 'a', type: 'attr', attr: 'href', transform: '' },
      { name: 'title', selector: 'a', type: 'text', attr: '', transform: '' },
    ]
    expect(getDetectionFingerprint(a)).toBe(getDetectionFingerprint(b))
  })
})

describe('validateForm', () => {
  it('requires a container selector', () => {
    const form = createEmptyForm()
    form.basic.name = 'test'
    form.basic.url = 'https://example.com'
    expect(validateForm(form)).toBe('容器选择器不能为空')
  })

  it('requires json api list path', () => {
    const form = createEmptyForm()
    form.basic.name = 'test'
    form.basic.url = 'https://example.com'
    form.extraction.containerSelector = 'data'
    form.extraction.sourceMode = 'api_json'
    form.extraction.sourceUrl = 'https://example.com/api'
    expect(validateForm(form)).toBe('JSON API 列表路径不能为空')
  })

  it('rejects duplicate field names', () => {
    const form = createEmptyForm()
    form.basic.name = 'test'
    form.basic.url = 'https://example.com'
    form.extraction.containerSelector = 'ul.list'
    form.extraction.fields = [
      { name: 'title', selector: 'a', type: 'text', attr: '', transform: '' },
      { name: 'title', selector: 'b', type: 'text', attr: '', transform: '' },
    ]
    expect(validateForm(form)).toBe('字段名称重复: title')
  })

  it('accepts a minimal valid form', () => {
    const form = createEmptyForm()
    form.basic.name = 'test'
    form.basic.url = 'https://example.com'
    form.extraction.containerSelector = 'ul.list'
    expect(validateForm(form)).toBeNull()
  })
})
