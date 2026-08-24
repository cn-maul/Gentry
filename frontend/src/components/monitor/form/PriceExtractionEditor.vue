<template>
  <div :class="advanced ? 'advanced-panel-section price-source-editor' : 'settings-section price-source-editor'">
    <template v-if="!advanced">
    <div class="section-header">
      <div>
        <h2>识别商品价格</h2>
        <p>系统会从商品页中找到名称和价格。</p>
      </div>
    </div>

    <div class="rule-match-row">
      <div class="match-target">
        <span class="match-label">商品网址</span>
        <code>{{ form.basic.url || '请先在上方粘贴商品网址' }}</code>
      </div>
      <button class="btn btn-primary btn-sm" type="button" :disabled="!form.basic.url || scanning" @click="handleScan">
        {{ scanning ? '识别中...' : '自动识别价格' }}
      </button>
    </div>

    <div v-if="scanning" class="loading"><div class="spinner" /><p>正在识别商品价格...</p></div>
    <div v-else-if="candidates.length" class="candidate-list">
      <div v-for="(candidate, index) in candidates" :key="candidateKey(candidate, index)" class="candidate-row">
        <div class="candidate-main">
          <div class="candidate-heading">
            <strong>识别结果 {{ index + 1 }}</strong>
            <span>发现 {{ candidate.item_count }} 个商品</span>
          </div>
          <div class="sample-list">
            <div v-for="(item, sampleIndex) in (candidate.sample_items || []).slice(0, 5)" :key="sampleIndex" class="sample-line">
              <span>{{ item.title || item.sku || '未命名商品' }}</span>
              <strong>{{ item.price }}</strong>
            </div>
          </div>
        </div>
        <button class="btn btn-primary btn-sm" type="button" @click="applyCandidate(candidate)">选择此结果</button>
      </div>
    </div>
    <div v-else-if="scanned" class="empty-match">暂时没有识别到价格。可以重试，或在高级设置中手动配置。</div>
    <div v-if="scanError" class="form-error">{{ scanError }}</div>
    <div v-if="recognitionComplete && !scanning && !candidates.length" class="recognition-result">
      <div class="recognition-heading">
        <div>
          <strong>已识别 {{ recognizedCount }} 个价格</strong>
          <span>创建后将分别跟踪以下商品规格</span>
        </div>
        <button class="btn btn-ghost btn-sm" type="button" @click="handleScan">重新识别</button>
      </div>
      <div class="recognized-price-list">
        <div v-for="(item, index) in recognizedItems" :key="item.sku || index" class="recognized-price-row">
          <div class="recognized-product">
            <strong>{{ item.title || item.sku || `商品 ${index + 1}` }}</strong>
            <code v-if="item.sku">{{ item.sku }}</code>
          </div>
          <div class="recognized-prices">
            <strong>{{ formatPrice(item.price) }}</strong>
            <span v-if="showOriginalPrice(item)">{{ formatPrice(item.original_price) }}</span>
          </div>
        </div>
      </div>
      <p v-if="recognizedItems.length < recognizedCount" class="recognized-overflow">
        另有 {{ recognizedCount - recognizedItems.length }} 个价格已识别
      </p>
    </div>
    </template>

    <template v-else>
    <div class="advanced-heading">
      <div>
        <h3>价格提取规则</h3>
        <p>仅在自动识别不准确时修改这些技术选项。</p>
      </div>
      <router-link to="/scan-rules" class="btn btn-ghost btn-sm">管理已保存规则</router-link>
    </div>
    <div class="source-mode" role="tablist" aria-label="价格来源类型">
      <button type="button" role="tab" :aria-selected="priceSourceMode === 'html_single'" :class="{ active: priceSourceMode === 'html_single' }" @click="selectSourceMode('html_single')">单商品网页</button>
      <button type="button" role="tab" :aria-selected="priceSourceMode === 'html_list'" :class="{ active: priceSourceMode === 'html_list' }" @click="selectSourceMode('html_list')">商品列表网页</button>
      <button type="button" role="tab" :aria-selected="priceSourceMode === 'api_json'" :class="{ active: priceSourceMode === 'api_json' }" @click="selectSourceMode('api_json')">JSON API</button>
    </div>

    <template v-if="priceSourceMode === 'api_json'">
      <div class="config-grid api-config">
        <div class="form-group api-url">
          <label>JSON API URL</label>
          <input :value="extraction.sourceUrl" @input="updateExtraction('sourceUrl', $event.target.value)" class="form-input" placeholder="https://shop.example/api/skus" />
        </div>
        <div class="form-group">
          <label>列表路径</label>
          <input :value="extraction.itemsPath" @input="updateItemsPath($event.target.value)" class="form-input" placeholder="data" />
        </div>
        <div class="form-group">
          <label>过滤字段</label>
          <input :value="extraction.filterPath" @input="updateExtraction('filterPath', $event.target.value)" class="form-input" placeholder="is_selling" />
        </div>
        <div class="form-group">
          <label>过滤值</label>
          <input :value="extraction.filterEquals" @input="updateExtraction('filterEquals', $event.target.value)" class="form-input" placeholder="true" />
        </div>
      </div>

      <div class="mapping-grid">
        <div class="form-group">
          <label>商品标题路径</label>
          <input :value="fieldSelector('title')" @input="updateMappedField('title', $event.target.value)" class="form-input" placeholder="spec_array.*.value" />
        </div>
        <div class="form-group">
          <label>SKU 路径</label>
          <input :value="fieldSelector(identityFieldName)" @input="updateMappedField(identityFieldName, $event.target.value)" class="form-input" placeholder="products_no" />
        </div>
        <div class="form-group">
          <label>价格路径</label>
          <input :value="fieldSelector(priceFieldName)" @input="updateMappedField(priceFieldName, $event.target.value)" class="form-input" placeholder="sell_price" />
        </div>
        <div class="form-group">
          <label>原价路径</label>
          <input :value="fieldSelector('original_price')" @input="updateOptionalField('original_price', $event.target.value)" class="form-input" placeholder="original_price" />
        </div>
      </div>

      <div class="api-variables">
        <div class="variable-heading">
          <strong>页面动态参数</strong>
          <button type="button" class="btn btn-ghost btn-sm" @click="addSourceVariable">添加参数</button>
        </div>
        <div v-for="([name, variable], index) in sourceVariableEntries" :key="`${name}-${index}`" class="variable-row">
          <div class="form-group">
            <label>参数名</label>
            <input :value="name" @input="updateSourceVariable(index, 'name', $event.target.value)" class="form-input" placeholder="goods_id" />
          </div>
          <div class="form-group">
            <label>页面选择器</label>
            <input :value="variable.selector" @input="updateSourceVariable(index, 'selector', $event.target.value)" class="form-input" placeholder="#goods_id" />
          </div>
          <div class="form-group">
            <label>取值属性</label>
            <input :value="variable.attr" @input="updateSourceVariable(index, 'attr', $event.target.value)" class="form-input" placeholder="value（留空则取文本）" />
          </div>
          <button type="button" class="icon-button danger variable-remove" title="删除动态参数" @click="removeSourceVariable(index)">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M3 6h18"/><path d="m19 6-1 14H6L5 6"/></svg>
          </button>
        </div>
      </div>

      <details class="request-headers">
        <summary>请求头</summary>
        <div class="header-grid">
          <div class="form-group">
            <label>Accept</label>
            <input :value="sourceHeader('Accept')" @input="updateSourceHeader('Accept', $event.target.value)" class="form-input" placeholder="application/json" />
          </div>
          <div class="form-group">
            <label>Accept-Language</label>
            <input :value="sourceHeader('Accept-Language')" @input="updateSourceHeader('Accept-Language', $event.target.value)" class="form-input" placeholder="zh-CN,zh;q=0.9" />
          </div>
          <div class="form-group">
            <label>Referer</label>
            <input :value="sourceHeader('Referer')" @input="updateSourceHeader('Referer', $event.target.value)" class="form-input" :placeholder="form.basic.url || 'https://shop.example/product'" />
          </div>
          <div class="form-group">
            <label>X-Requested-With</label>
            <input :value="sourceHeader('X-Requested-With')" @input="updateSourceHeader('X-Requested-With', $event.target.value)" class="form-input" placeholder="XMLHttpRequest" />
          </div>
        </div>
      </details>
    </template>

    <template v-else>
      <div class="config-grid html-config">
        <div class="form-group">
          <label>商品容器</label>
          <input :value="extraction.containerSelector" @input="updateExtraction('containerSelector', $event.target.value)" class="form-input" :placeholder="priceSourceMode === 'html_single' ? 'body' : '.product-list'" />
        </div>
        <div v-if="priceSourceMode === 'html_list'" class="form-group">
          <label>商品条目</label>
          <input :value="extraction.itemSelector" @input="updateExtraction('itemSelector', $event.target.value)" class="form-input" placeholder=".product-item" />
        </div>
        <div class="form-group">
          <label>商品标题</label>
          <input :value="fieldSelector('title')" @input="updateMappedField('title', $event.target.value)" class="form-input" placeholder="h1" />
        </div>
        <div v-if="priceSourceMode === 'html_list'" class="form-group">
          <label>商品身份</label>
          <input :value="fieldSelector(identityFieldName)" @input="updateMappedField(identityFieldName, $event.target.value)" class="form-input" placeholder="[data-sku]" />
        </div>
        <div class="form-group">
          <label>商品价格</label>
          <input :value="fieldSelector(priceFieldName)" @input="updateMappedField(priceFieldName, $event.target.value)" class="form-input" placeholder=".price" />
        </div>
      </div>
    </template>
    </template>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue'
import { previewScan } from '../../../api/monitors'

const props = defineProps({
  form: { type: Object, required: true },
  advanced: { type: Boolean, default: false },
})
const emit = defineEmits(['update:form'])

const scanning = ref(false)
const scanned = ref(false)
const scanError = ref('')
const scanResult = ref(null)
const recognitionComplete = ref(false)
const recognizedCandidate = ref(null)

const extraction = computed(() => props.form.extraction)
const candidates = computed(() => scanResult.value?.containers || [])
const recognizedItems = computed(() => recognizedCandidate.value?.sample_items || [])
const recognizedCount = computed(() => recognizedCandidate.value?.item_count || recognizedItems.value.length)
const priceSourceMode = computed(() => {
  if (extraction.value.sourceMode === 'api_json') return 'api_json'
  return props.form.rule.pageMode === 'list' ? 'html_list' : 'html_single'
})
const priceFieldName = computed(() => props.form.rule.target.field || 'price')
const identityFieldName = computed(() => props.form.rule.identity.field || 'sku')
const sourceVariableEntries = computed(() => Object.entries(extraction.value.sourceVariables || {}))

async function handleScan() {
  let autoApplied = false
  scanning.value = true
  scanned.value = false
  scanError.value = ''
  recognitionComplete.value = false
  recognizedCandidate.value = null
  try {
    const response = await previewScan({
      url: props.form.basic.url.trim(),
      keywords: '价格,售价,优惠',
      strategy_type: 'field_transition',
    })
    if (response.code === 0) {
      scanResult.value = response.data
      if (candidates.value.length === 1) {
        applyCandidate(candidates.value[0])
        autoApplied = true
      }
    }
    else scanError.value = response.message || '规则匹配失败'
  } catch (error) {
    scanError.value = error.response?.data?.message || error.message || '规则匹配失败'
  } finally {
    scanning.value = false
    scanned.value = !autoApplied
  }
}

function applyCandidate(candidate) {
  const config = candidate.config || {}
  const fields = (config.fields || []).map(field => ({
    name: field.name || '', selector: field.selector || '', type: field.type || 'text',
    attr: field.attr || '', transform: field.transform || '',
  }))
  const fetch = config.fetch_config
  const nextExtraction = {
    ...extraction.value,
    sourceMode: fetch?.mode === 'api_json' ? 'api_json' : 'html',
    sourceUrl: fetch?.url || '',
    itemsPath: fetch?.items_path || '',
    filterPath: fetch?.filter_path || '',
    filterEquals: fetch?.filter_equals ?? '',
    sourceHeaders: fetch?.headers || {},
    sourceVariables: fetch?.variables || {},
    containerSelector: config.container || candidate.container_css || '',
    itemSelector: config.item || candidate.item_css || '',
    fields: fields.length ? fields : extraction.value.fields,
  }
  const priceField = fields.find(field => ['price', 'sell_price', 'sale_price'].includes(field.name))?.name || priceFieldName.value
  const identityField = fields.find(field => ['sku', 'products_no', 'product_id', 'id', 'url'].includes(field.name) && field.name !== priceField)?.name
  const isList = Boolean(fetch) || Boolean(nextExtraction.itemSelector)
  emit('update:form', {
    ...props.form,
    extraction: nextExtraction,
    rule: {
      ...props.form.rule,
      pageMode: isList ? 'list' : 'single',
      identity: isList ? { mode: 'field', field: identityField || 'sku' } : { mode: 'source_url', field: '' },
      target: { ...props.form.rule.target, field: priceField, valueType: 'money' },
    },
  })
  recognizedCandidate.value = candidate
  recognitionComplete.value = true
  scanResult.value = null
  scanned.value = false
}

function formatPrice(value) {
  const text = String(value ?? '').trim()
  if (!text) return '价格未读取'
  return /[¥￥$€£]/.test(text) ? text : `¥${text}`
}

function showOriginalPrice(item) {
  const original = String(item.original_price ?? '').trim()
  return original && original !== String(item.price ?? '').trim()
}

function selectSourceMode(mode) {
  const isAPI = mode === 'api_json'
  const isList = mode !== 'html_single'
  const wasAPI = extraction.value.sourceMode === 'api_json'
  let fields = extraction.value.fields.map(field => ({ ...field }))
  if (isAPI && !wasAPI) {
    fields = fields.map(field => {
      if (field.name === 'title' && field.selector === 'a') return { ...field, selector: '' }
      if (field.name === priceFieldName.value && field.selector === '.price') return { ...field, selector: '' }
      return field
    })
    if (!fields.some(field => field.name === identityFieldName.value)) {
      fields.push({ name: identityFieldName.value, selector: '', type: 'text', attr: '', transform: '' })
    }
  } else if (!isAPI && wasAPI) {
    fields = fields.map(field => {
      if (field.name === 'title') return { ...field, selector: 'h1' }
      if (field.name === priceFieldName.value) return { ...field, selector: '.price' }
      if (field.name === identityFieldName.value) return { ...field, selector: '' }
      return field
    })
  }
  const nextExtraction = {
    ...extraction.value,
    sourceMode: isAPI ? 'api_json' : 'html',
    sourceUrl: isAPI ? extraction.value.sourceUrl : '',
    itemsPath: isAPI ? (extraction.value.itemsPath || 'data') : '',
    filterPath: isAPI ? extraction.value.filterPath : '',
    filterEquals: isAPI ? extraction.value.filterEquals : '',
    sourceHeaders: isAPI ? (Object.keys(extraction.value.sourceHeaders || {}).length ? extraction.value.sourceHeaders : {
      Accept: 'application/json, text/javascript, */*; q=0.01',
      Referer: '{{page_url}}',
      'X-Requested-With': 'XMLHttpRequest',
    }) : {},
    sourceVariables: isAPI ? (extraction.value.sourceVariables || {}) : {},
    containerSelector: isAPI ? (extraction.value.itemsPath || 'data') : (extraction.value.containerSelector || 'body'),
    itemSelector: isAPI ? '*' : (isList ? extraction.value.itemSelector : ''),
    fields,
  }
  emit('update:form', {
    ...props.form,
    extraction: nextExtraction,
    rule: {
      ...props.form.rule,
      pageMode: isList ? 'list' : 'single',
      identity: isList ? { mode: 'field', field: identityFieldName.value || 'sku' } : { mode: 'source_url', field: '' },
    },
  })
}

function updateExtraction(key, value) {
  emit('update:form', { ...props.form, extraction: { ...extraction.value, [key]: value } })
}

function updateItemsPath(value) {
  emit('update:form', {
    ...props.form,
    extraction: { ...extraction.value, itemsPath: value, containerSelector: value, itemSelector: '*' },
  })
}

function fieldSelector(name) {
  return extraction.value.fields.find(field => field.name === name)?.selector || ''
}

function updateMappedField(name, selector) {
  const fields = extraction.value.fields.map(field => field.name === name ? { ...field, selector } : field)
  if (!fields.some(field => field.name === name)) {
    fields.push({ name, selector, type: 'text', attr: '', transform: '' })
  }
  emit('update:form', { ...props.form, extraction: { ...extraction.value, fields } })
}

function updateOptionalField(name, selector) {
  if (selector) updateMappedField(name, selector)
  else emit('update:form', {
    ...props.form,
    extraction: { ...extraction.value, fields: extraction.value.fields.filter(field => field.name !== name) },
  })
}

function sourceHeader(name) {
  return extraction.value.sourceHeaders?.[name] || ''
}

function updateSourceHeader(name, value) {
  const headers = { ...(extraction.value.sourceHeaders || {}) }
  if (value.trim()) headers[name] = value
  else delete headers[name]
  emit('update:form', { ...props.form, extraction: { ...extraction.value, sourceHeaders: headers } })
}

function addSourceVariable() {
  const variables = { ...(extraction.value.sourceVariables || {}) }
  let name = 'parameter'
  let suffix = 2
  while (Object.prototype.hasOwnProperty.call(variables, name)) name = `parameter_${suffix++}`
  variables[name] = { source: 'html', selector: '', attr: '' }
  emit('update:form', { ...props.form, extraction: { ...extraction.value, sourceVariables: variables } })
}

function updateSourceVariable(index, key, value) {
  const entries = sourceVariableEntries.value.map(([name, variable]) => [name, { ...variable }])
  if (!entries[index]) return
  if (key === 'name') entries[index][0] = value
  else entries[index][1][key] = value
  emit('update:form', {
    ...props.form,
    extraction: { ...extraction.value, sourceVariables: Object.fromEntries(entries) },
  })
}

function removeSourceVariable(index) {
  const entries = sourceVariableEntries.value.filter((_, entryIndex) => entryIndex !== index)
  emit('update:form', {
    ...props.form,
    extraction: { ...extraction.value, sourceVariables: Object.fromEntries(entries) },
  })
}

function strategyLabel(strategy) {
  return strategy?.startsWith('template_') ? `规则「${strategy.slice(9)}」` : '价格提取候选'
}

function candidateKey(candidate, index) {
  return `${candidate.strategy || ''}:${candidate.config?.container || ''}:${index}`
}
</script>

<style scoped>
.section-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; margin-bottom: 1rem; }
.section-header h2 { font-size: 1.125rem; margin-bottom: 0.2rem; }
.section-header p { color: var(--text-secondary); font-size: 0.8125rem; }
.advanced-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; }
.advanced-heading h3 { margin-bottom: 0.2rem; color: var(--text); font-size: 0.875rem; }
.advanced-heading p { color: var(--text-muted); font-size: 0.75rem; }
.recognition-result { margin-top: 0.75rem; border-top: 1px solid var(--border-light); }
.recognition-heading { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 0.9rem 0; }
.recognition-heading > div { display: grid; gap: 0.15rem; }
.recognition-heading strong { color: var(--success); font-size: 0.875rem; }
.recognition-heading span { color: var(--text-muted); font-size: 0.75rem; }
.recognized-price-list { border-top: 1px solid var(--border-light); }
.recognized-price-row { display: flex; align-items: center; justify-content: space-between; gap: 1rem; min-height: 54px; padding: 0.65rem 0; border-bottom: 1px solid var(--border-light); }
.recognized-product { display: grid; gap: 0.15rem; min-width: 0; }
.recognized-product strong { overflow: hidden; color: var(--text); font-size: 0.8125rem; text-overflow: ellipsis; white-space: nowrap; }
.recognized-product code { overflow: hidden; color: var(--text-muted); font-size: 0.6875rem; text-overflow: ellipsis; white-space: nowrap; }
.recognized-prices { display: flex; align-items: baseline; gap: 0.45rem; flex-shrink: 0; font-variant-numeric: tabular-nums; }
.recognized-prices strong { color: var(--green); font-size: 0.9375rem; }
.recognized-prices span { color: var(--text-muted); font-size: 0.6875rem; text-decoration: line-through; }
.recognized-overflow { padding-top: 0.65rem; color: var(--text-muted); font-size: 0.75rem; }
.rule-match-row { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 0.75rem 0; border-top: 1px solid var(--border-light); border-bottom: 1px solid var(--border-light); }
.match-target { min-width: 0; display: grid; gap: 0.2rem; }
.match-label { color: var(--text-muted); font-size: 0.6875rem; font-weight: 700; }
.match-target code { overflow: hidden; color: var(--text-secondary); font-size: 0.75rem; text-overflow: ellipsis; white-space: nowrap; }
.candidate-list { border-bottom: 1px solid var(--border-light); }
.candidate-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; align-items: center; gap: 1rem; padding: 0.85rem 0; border-bottom: 1px solid var(--border-light); }
.candidate-row:last-child { border-bottom: 0; }
.candidate-heading { display: flex; align-items: center; gap: 0.6rem; margin-bottom: 0.45rem; font-size: 0.75rem; }
.candidate-heading span { color: var(--text-muted); }
.sample-list { display: grid; gap: 0.2rem; }
.sample-line { display: flex; justify-content: space-between; gap: 1rem; min-width: 0; font-size: 0.75rem; color: var(--text-secondary); }
.sample-line span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sample-line strong { flex-shrink: 0; color: var(--green); font-variant-numeric: tabular-nums; }
.empty-match { padding: 0.75rem 0; border-bottom: 1px solid var(--border-light); color: var(--text-muted); font-size: 0.8125rem; }
.source-mode { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); margin: 1rem 0; overflow: hidden; border: 1px solid var(--border); border-radius: 6px; }
.source-mode button { min-height: 38px; padding: 0 0.75rem; border: 0; border-right: 1px solid var(--border); background: var(--bg-elevated); color: var(--text-secondary); cursor: pointer; font-size: 0.8125rem; }
.source-mode button:last-child { border-right: 0; }
.source-mode button.active { color: #000; background: var(--green); font-weight: 700; }
.config-grid, .mapping-grid, .header-grid { display: grid; gap: 0.75rem; }
.config-grid .form-group, .mapping-grid .form-group, .header-grid .form-group { min-width: 0; margin: 0; }
.api-config { grid-template-columns: minmax(280px, 2fr) repeat(3, minmax(130px, 1fr)); }
.mapping-grid { grid-template-columns: repeat(4, minmax(0, 1fr)); margin-top: 0.75rem; }
.api-variables { margin-top: 0.75rem; padding-top: 0.75rem; border-top: 1px solid var(--border-light); }
.variable-heading { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
.variable-heading strong { font-size: 0.8125rem; }
.variable-row { display: grid; grid-template-columns: minmax(140px, 0.7fr) minmax(220px, 1.5fr) minmax(180px, 1fr) auto; align-items: end; gap: 0.75rem; margin-top: 0.75rem; }
.variable-row .form-group { min-width: 0; margin: 0; }
.variable-remove { margin-bottom: 2px; }
.html-config { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.request-headers { margin-top: 0.75rem; border-top: 1px solid var(--border-light); }
.request-headers summary { padding: 0.75rem 0; color: var(--text-secondary); cursor: pointer; font-size: 0.75rem; font-weight: 700; }
.header-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); padding-bottom: 0.25rem; }
.form-error { margin-top: 0.75rem; }
@media (max-width: 900px) {
  .api-config, .mapping-grid, .html-config, .header-grid, .variable-row { grid-template-columns: 1fr 1fr; }
}
@media (max-width: 640px) {
  .section-header, .rule-match-row { align-items: stretch; flex-direction: column; }
  .source-mode { grid-template-columns: 1fr; }
  .source-mode button { border-right: 0; border-bottom: 1px solid var(--border); }
  .source-mode button:last-child { border-bottom: 0; }
  .api-config, .mapping-grid, .html-config, .header-grid, .variable-row { grid-template-columns: 1fr; }
  .variable-heading { align-items: stretch; flex-direction: column; }
  .variable-remove { justify-self: end; }
  .candidate-row { grid-template-columns: 1fr; }
  .candidate-row .btn { width: 100%; }
  .recognition-heading { align-items: stretch; flex-direction: column; }
  .recognition-heading .btn { width: 100%; }
  .recognized-price-row { align-items: flex-start; }
  .recognized-product strong { white-space: normal; }
}
</style>
