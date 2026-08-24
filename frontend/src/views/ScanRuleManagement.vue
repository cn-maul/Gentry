<template>
  <div class="page scan-rules-page">
    <div v-if="successMsg" class="toast toast-success">{{ successMsg }}</div>
    <div v-if="pageErrorMsg" class="toast toast-error">{{ pageErrorMsg }}</div>

    <header class="page-header">
      <div>
        <h1>高级规则</h1>
        <p>添加和管理可复用的网页与价格提取规则</p>
      </div>
      <div class="header-actions">
        <input ref="fileInput" class="file-input" type="file" accept="application/json,.json" @change="handleImportFile" />
        <button class="btn btn-ghost btn-sm" :disabled="importing" @click="fileInput?.click()">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M12 3v12"/><path d="m7 10 5 5 5-5"/><path d="M5 21h14"/></svg>
          {{ importing ? '导入中...' : '导入' }}
        </button>
        <button class="btn btn-ghost btn-sm" :disabled="exporting || rules.length === 0" @click="handleExport">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M12 21V9"/><path d="m17 14-5-5-5 5"/><path d="M5 3h14"/></svg>
          {{ exporting ? '导出中...' : '导出' }}
        </button>
      </div>
    </header>

    <section class="builder-section" aria-labelledby="quick-rule-title">
      <div class="section-title-row">
        <h2 id="quick-rule-title">添加规则</h2>
        <span v-if="builderMode === 'web' && scanResult" class="result-count">{{ candidates.length }} 个候选</span>
      </div>

      <div class="builder-mode" role="tablist" aria-label="规则类型">
        <button type="button" role="tab" :aria-selected="builderMode === 'web'" :class="{ active: builderMode === 'web' }" @click="builderMode = 'web'">网页规则</button>
        <button type="button" role="tab" :aria-selected="builderMode === 'price_api'" :class="{ active: builderMode === 'price_api' }" @click="builderMode = 'price_api'">价格 API 规则</button>
      </div>

      <template v-if="builderMode === 'web'">
      <div class="scan-form">
        <div class="form-group url-field">
          <label for="rule-url">URL</label>
          <input id="rule-url" v-model="url" class="form-input" placeholder="https://example.com/announcements/" @keyup.enter="handleScan" />
        </div>
        <div class="form-group keyword-field">
          <label for="rule-keywords">关键词</label>
          <input id="rule-keywords" v-model="keywords" class="form-input" placeholder="公告, 招聘, 公示" @keyup.enter="handleScan" />
        </div>
        <button class="btn btn-primary scan-button" :disabled="!url.trim() || scanning" @click="handleScan">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/></svg>
          {{ scanning ? '扫描中...' : '预扫描' }}
        </button>
      </div>

      <div v-if="scanError" class="inline-error">{{ scanError }}</div>
      <div v-if="scanning" class="scan-loading"><div class="spinner" /><span>正在扫描网页</span></div>

      <div v-else-if="candidates.length" class="candidate-list">
        <button
          v-for="(candidate, index) in candidates"
          :key="candidateKey(candidate, index)"
          type="button"
          class="candidate-row"
          :class="{ selected: selectedIndex === index }"
          @click="selectedIndex = index"
        >
          <span class="choice-mark" aria-hidden="true">
            <svg v-if="selectedIndex === index" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><path d="m5 12 4 4L19 6"/></svg>
          </span>
          <span class="candidate-content">
            <span class="candidate-heading">
              <strong>候选 {{ index + 1 }}</strong>
              <span>{{ candidate.item_count }} 条</span>
              <span v-if="candidate.keyword_hits">关键词命中 {{ candidate.keyword_hits }}</span>
              <span class="candidate-strategy" :class="strategyClass(candidate.strategy)" v-if="candidate.strategy">{{ strategyLabel(candidate.strategy) }}</span>
            </span>
            <span class="sample-list">
              <span v-for="(item, itemIndex) in (candidate.sample_items || []).slice(0, 4)" :key="itemIndex" class="sample-line">
                <span class="sample-title">{{ item.title || item.url || '未命名内容' }}</span>
                <span v-if="item.date" class="sample-date">{{ item.date }}</span>
              </span>
            </span>
            <span class="selector-line">
              <code>{{ candidate.config?.container }}</code>
              <span>/</span>
              <code>{{ candidate.config?.item }}</code>
            </span>
          </span>
        </button>
      </div>

      <div v-else-if="scanned" class="empty-result">没有找到可保存的内容区域</div>

      <div v-if="selectedCandidate" class="save-panel">
        <div class="form-group name-field">
          <label for="rule-name">规则名称</label>
          <input id="rule-name" v-model="ruleName" class="form-input" placeholder="例如：殷都区招聘公告列表" @keyup.enter="handleSave" />
        </div>
        <div class="scope-field">
          <span class="field-label">适用范围</span>
          <div class="scope-control">
            <button type="button" :class="{ active: scopeType === 'exact' }" @click="scopeType = 'exact'">当前页面</button>
            <button type="button" :disabled="!routeScopeAvailable" :class="{ active: scopeType === 'route' }" title="匹配当前路径及其子路径" @click="scopeType = 'route'">当前路由</button>
            <button type="button" :class="{ active: scopeType === 'global' }" title="跨网站按相同页面结构匹配" @click="scopeType = 'global'">通用结构</button>
          </div>
        </div>
        <div class="scope-summary">{{ scopeSummary }}</div>
        <button class="btn btn-primary save-button" :disabled="saving || !ruleName.trim()" @click="handleSave">
          {{ saving ? '保存中...' : '保存规则' }}
        </button>
      </div>
      </template>

      <template v-else>
        <div class="price-builder">
          <div class="price-builder-hint">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>
            <span>此页面适合熟悉 CSS 选择器或 JSON API 的用户。普通监控请使用「新增监控」页面的自动识别功能。</span>
          </div>
          <div class="price-grid primary-fields">
            <div class="form-group">
              <label for="price-rule-name">规则名称</label>
              <input id="price-rule-name" v-model="ruleName" class="form-input" placeholder="例如：商城商品 SKU" />
            </div>
            <div class="form-group page-url-field">
              <label for="price-page-url">页面 URL</label>
              <input id="price-page-url" v-model="url" class="form-input" placeholder="https://shop.example/products/item" />
            </div>
          </div>

          <div class="price-grid source-fields">
            <div class="form-group api-url-field">
              <label for="price-api-url">JSON API URL</label>
              <input id="price-api-url" v-model="priceDraft.apiUrl" class="form-input" placeholder="https://shop.example/api/skus?id=31" />
            </div>
            <div class="form-group">
              <label for="price-items-path">列表路径</label>
              <input id="price-items-path" v-model="priceDraft.itemsPath" class="form-input" placeholder="data" />
            </div>
            <div class="form-group">
              <label for="price-filter-path">过滤字段</label>
              <input id="price-filter-path" v-model="priceDraft.filterPath" class="form-input" placeholder="is_selling" />
            </div>
            <div class="form-group">
              <label for="price-filter-value">过滤值</label>
              <input id="price-filter-value" v-model="priceDraft.filterEquals" class="form-input" placeholder="true" />
            </div>
          </div>

          <div class="variable-section">
            <div class="variable-heading">
              <div>
                <strong>动态参数</strong>
                <span>先从商品页面提取，再替换 API URL 中的 &#123;&#123;参数名&#125;&#125;</span>
              </div>
              <button type="button" class="btn btn-ghost btn-sm" @click="addPriceVariable">添加参数</button>
            </div>
            <div v-for="(variable, index) in priceDraft.variables" :key="index" class="variable-row">
              <div class="form-group">
                <label :for="`price-variable-name-${index}`">参数名</label>
                <input :id="`price-variable-name-${index}`" v-model="variable.name" class="form-input" placeholder="goods_id" />
              </div>
              <div class="form-group">
                <label :for="`price-variable-selector-${index}`">页面选择器</label>
                <input :id="`price-variable-selector-${index}`" v-model="variable.selector" class="form-input" placeholder="#goods_id" />
              </div>
              <div class="form-group">
                <label :for="`price-variable-attr-${index}`">取值属性</label>
                <input :id="`price-variable-attr-${index}`" v-model="variable.attr" class="form-input" placeholder="value（留空则取文本）" />
              </div>
              <button type="button" class="icon-button danger variable-remove" title="删除动态参数" @click="removePriceVariable(index)">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M3 6h18"/><path d="m19 6-1 14H6L5 6"/></svg>
              </button>
            </div>
          </div>

          <div class="price-grid mapping-fields">
            <div class="form-group">
              <label for="price-title-path">标题路径</label>
              <input id="price-title-path" v-model="priceDraft.titlePath" class="form-input" placeholder="spec_array.*.value" />
            </div>
            <div class="form-group">
              <label for="price-identity-path">商品身份路径</label>
              <input id="price-identity-path" v-model="priceDraft.identityPath" class="form-input" placeholder="products_no" />
            </div>
            <div class="form-group">
              <label for="price-value-path">价格路径</label>
              <input id="price-value-path" v-model="priceDraft.pricePath" class="form-input" placeholder="sell_price" />
            </div>
            <div class="form-group">
              <label for="price-original-path">原价路径</label>
              <input id="price-original-path" v-model="priceDraft.originalPricePath" class="form-input" placeholder="original_price" />
            </div>
          </div>

          <div class="price-grid header-fields">
            <div class="form-group">
              <label for="price-accept">Accept</label>
              <input id="price-accept" v-model="priceDraft.headers.Accept" class="form-input" placeholder="application/json" />
            </div>
            <div class="form-group">
              <label for="price-language">Accept-Language</label>
              <input id="price-language" v-model="priceDraft.headers['Accept-Language']" class="form-input" placeholder="zh-CN,zh;q=0.9" />
            </div>
            <div class="form-group">
              <label for="price-referer">Referer</label>
              <input id="price-referer" v-model="priceDraft.headers.Referer" class="form-input" :placeholder="url || 'https://shop.example/products/item'" />
            </div>
            <div class="form-group">
              <label for="price-requested-with">X-Requested-With</label>
              <input id="price-requested-with" v-model="priceDraft.headers['X-Requested-With']" class="form-input" placeholder="XMLHttpRequest" />
            </div>
          </div>

          <div v-if="priceError" class="inline-error">{{ priceError }}</div>
          <div v-if="priceValidation" class="price-preview">
            <div class="preview-heading">
              <strong>提取结果</strong>
              <span>{{ priceValidation.extracted_items }} 条</span>
            </div>
            <div class="sample-list">
              <div v-for="(item, index) in priceSamples" :key="index" class="sample-line">
                <span class="sample-title">{{ item.title || item.sku || item.item_key || '未命名商品' }}</span>
                <span class="sample-price">{{ item.price || item.normalized || item.raw }}</span>
              </div>
            </div>
          </div>

          <div class="price-actions">
            <div class="scope-field">
              <span class="field-label">适用范围</span>
              <div class="scope-control">
                <button type="button" :class="{ active: scopeType === 'exact' }" @click="scopeType = 'exact'">当前页面</button>
                <button type="button" :disabled="!sectionScopeAvailable" :class="{ active: scopeType === 'section' }" title="匹配同一网站、同一目录下的商品页" @click="scopeType = 'section'">同站目录</button>
                <button type="button" :class="{ active: scopeType === 'global' }" @click="scopeType = 'global'">通用结构</button>
              </div>
            </div>
            <div class="scope-summary">{{ scopeSummary }}</div>
            <button class="btn btn-ghost" :disabled="priceTesting" @click="handleTestPriceRule">
              {{ priceTesting ? '测试中...' : '测试提取' }}
            </button>
            <button v-if="editingRule" class="btn btn-ghost" :disabled="saving" @click="resetBuilder">取消编辑</button>
            <button class="btn btn-primary" :disabled="saving || priceTesting || !ruleName.trim()" @click="handleSavePriceRule">
              {{ saving ? '保存中...' : (editingRule ? '更新规则' : '保存规则') }}
            </button>
          </div>
        </div>
      </template>
    </section>

    <section class="library-section" aria-labelledby="rule-library-title">
      <div class="section-title-row library-title">
        <h2 id="rule-library-title">已保存规则</h2>
        <span>{{ rules.length }}</span>
      </div>

      <div v-if="loading" class="list-state">正在加载规则</div>
      <div v-else-if="rules.length === 0" class="list-state">暂无已保存规则</div>
      <div v-else class="rule-list">
        <article v-for="rule in rules" :key="rule.id" class="rule-row">
          <div class="rule-main">
            <div class="rule-heading">
              <strong>{{ rule.name }}</strong>
              <span class="source-badge" :class="{ api: isAPIRule(rule) }">{{ isAPIRule(rule) ? '价格 API' : '网页' }}</span>
              <span class="scope-badge" :class="`scope-${rule.scope_type || 'legacy'}`">{{ scopeName(rule) }}</span>
              <span v-if="!rule.enabled" class="disabled-badge">已禁用</span>
            </div>
            <div class="rule-target">{{ scopeTarget(rule) }}</div>
            <div class="rule-structure">
              <code>{{ rule.container }}</code>
              <span>/</span>
              <code>{{ rule.item }}</code>
              <span class="field-count">{{ (rule.fields || []).length }} 个字段</span>
            </div>
          </div>
          <div class="rule-actions">
            <button v-if="isAPIRule(rule)" class="icon-button" title="编辑价格规则" @click="startEditPriceRule(rule)">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L8 18l-4 1 1-4Z"/></svg>
            </button>
            <button class="icon-button danger" title="删除规则" @click="handleDelete(rule)">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M3 6h18"/><path d="M8 6V4h8v2"/><path d="m19 6-1 14H6L5 6"/><path d="M10 11v5M14 11v5"/></svg>
            </button>
          </div>
        </article>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { deleteScanRule, exportScanRules, fetchScanRules, importScanRules, previewScan, quickCreateScanRule, updateScanRule, validateMonitorConfig } from '../api/monitors'
import {
  buildPriceRuleConfig,
  buildPriceRuleValidationRequest,
  createEmptyPriceRuleDraft,
  priceRuleFingerprint,
  validatePriceRuleDraft,
} from '../composables/usePriceScanRuleBuilder'
import { useToastMessages } from '../composables/useToastMessages'

const { successMsg, pageErrorMsg, showSuccess, showError } = useToastMessages()

const loading = ref(true)
const rules = ref([])
const builderMode = ref('web')
const url = ref('')
const keywords = ref('')
const scanning = ref(false)
const scanned = ref(false)
const scanError = ref('')
const scanResult = ref(null)
const selectedIndex = ref(null)
const ruleName = ref('')
const scopeType = ref('exact')
const saving = ref(false)
const importing = ref(false)
const exporting = ref(false)
const fileInput = ref(null)
const priceDraft = reactive(createEmptyPriceRuleDraft())
const priceTesting = ref(false)
const priceError = ref('')
const priceValidation = ref(null)
const priceValidatedFingerprint = ref('')
const editingRule = ref(null)

const candidates = computed(() => scanResult.value?.containers || [])
const selectedCandidate = computed(() => selectedIndex.value === null ? null : candidates.value[selectedIndex.value])
const priceSamples = computed(() => priceValidation.value?.items?.[0]?.samples || [])
const parsedURL = computed(() => {
  try { return new URL(url.value.trim()) } catch { return null }
})
const routeScopeAvailable = computed(() => Boolean(parsedURL.value && (parsedURL.value.pathname !== '/' || parsedURL.value.search)))
const sectionScopeAvailable = computed(() => Boolean(parsedURL.value && parsedURL.value.pathname.split('/').filter(Boolean).length > 1))
const scopeSummary = computed(() => {
  if (scopeType.value === 'global') return '所有网站中结构相同的页面'
  if (!parsedURL.value) return ''
  if (scopeType.value === 'section') {
    const parts = parsedURL.value.pathname.split('/').filter(Boolean)
    return `${parsedURL.value.host}/${parts.slice(0, -1).join('/')}/*`
  }
  if (scopeType.value === 'route') return `${parsedURL.value.host}${parsedURL.value.pathname}${parsedURL.value.search}`
  return parsedURL.value.href
})

watch([url, keywords], () => {
  if (builderMode.value !== 'web') return
  scanResult.value = null
  selectedIndex.value = null
  scanned.value = false
  scanError.value = ''
  ruleName.value = ''
  scopeType.value = 'exact'
})

watch([url, () => JSON.stringify(priceDraft)], () => {
  if (builderMode.value !== 'price_api') return
  priceValidation.value = null
  priceValidatedFingerprint.value = ''
  priceError.value = ''
})

watch(builderMode, mode => {
  scanError.value = ''
  priceError.value = ''
  if (mode === 'web') {
    editingRule.value = null
    scopeType.value = 'exact'
  } else if (!editingRule.value) {
    scopeType.value = 'section'
  }
})

onMounted(loadRules)

async function loadRules() {
  loading.value = true
  try {
    const response = await fetchScanRules()
    rules.value = response.code === 0 ? (response.data || []) : []
  } catch (error) {
    showError('加载规则失败: ' + errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function handleScan() {
  if (!url.value.trim()) return
  scanning.value = true
  scanned.value = false
  scanError.value = ''
  scanResult.value = null
  selectedIndex.value = null
  try {
    const response = await previewScan({ url: url.value.trim(), keywords: keywords.value.trim() })
    if (response.code === 0) scanResult.value = response.data
    else scanError.value = response.message || '扫描失败'
  } catch (error) {
    scanError.value = errorMessage(error)
  } finally {
    scanning.value = false
    scanned.value = true
  }
}

async function handleSave() {
  if (!selectedCandidate.value || !ruleName.value.trim()) return
  saving.value = true
  try {
    await quickCreateScanRule({
      name: ruleName.value.trim(),
      url: url.value.trim(),
      keywords: keywords.value.trim(),
      scope_type: scopeType.value,
      config: selectedCandidate.value.config,
    })
    showSuccess('规则已保存')
    resetBuilder()
    await loadRules()
  } catch (error) {
    showError('保存规则失败: ' + errorMessage(error))
  } finally {
    saving.value = false
  }
}

async function handleTestPriceRule() {
  const localError = validatePriceRuleDraft(url.value, priceDraft)
  if (localError) {
    priceError.value = localError
    return false
  }
  priceTesting.value = true
  priceError.value = ''
  priceValidation.value = null
  try {
    const response = await validateMonitorConfig(buildPriceRuleValidationRequest(url.value, priceDraft))
    if (response.code !== 0 || !response.data?.valid) {
      throw new Error(response.message || response.data?.summary || '提取测试失败')
    }
    priceValidation.value = response.data
    priceValidatedFingerprint.value = priceRuleFingerprint(url.value, priceDraft)
    return true
  } catch (error) {
    priceError.value = errorMessage(error)
    return false
  } finally {
    priceTesting.value = false
  }
}

async function handleSavePriceRule() {
  if (!ruleName.value.trim()) return
  if (priceValidatedFingerprint.value !== priceRuleFingerprint(url.value, priceDraft)) {
    if (!await handleTestPriceRule()) return
  }
  saving.value = true
  try {
    const config = buildPriceRuleConfig(priceDraft)
    if (editingRule.value) {
      await updateScanRule(editingRule.value.id, {
        name: ruleName.value.trim(),
        url_contains: editingRule.value.url_contains || '',
        source_url: url.value.trim(),
        scope_type: scopeType.value,
        container: config.container,
        item: config.item,
        priority: editingRule.value.priority || 50,
        enabled: editingRule.value.enabled !== false,
        description: editingRule.value.description || '通过价格规则编辑器生成',
        fetch_config: config.fetch_config,
        fields: config.fields,
      })
      showSuccess('价格规则已更新')
    } else {
      await quickCreateScanRule({
        name: ruleName.value.trim(),
        url: url.value.trim(),
        scope_type: scopeType.value,
        config,
      })
      showSuccess('价格规则已保存')
    }
    resetBuilder()
    await loadRules()
  } catch (error) {
    showError('保存价格规则失败: ' + errorMessage(error))
  } finally {
    saving.value = false
  }
}

function startEditPriceRule(rule) {
  let fetchConfig = rule.fetch_config || {}
  if (typeof fetchConfig === 'string') {
    try { fetchConfig = JSON.parse(fetchConfig) } catch { fetchConfig = {} }
  }
  const fields = rule.fields || []
  const fieldPath = name => fields.find(field => field.name === name)?.selector || ''
  editingRule.value = rule
  builderMode.value = 'price_api'
  url.value = rule.source_url || ''
  ruleName.value = rule.name || ''
  scopeType.value = rule.scope_type || 'exact'
  Object.assign(priceDraft, createEmptyPriceRuleDraft(), {
    apiUrl: fetchConfig.url || '',
    itemsPath: fetchConfig.items_path || rule.container || 'data',
    filterPath: fetchConfig.filter_path || '',
    filterEquals: fetchConfig.filter_equals ?? '',
    headers: { ...createEmptyPriceRuleDraft().headers, ...(fetchConfig.headers || {}) },
    variables: Object.entries(fetchConfig.variables || {}).map(([name, variable]) => ({
      name, selector: variable.selector || '', attr: variable.attr || '',
    })),
    titlePath: fieldPath('title'),
    identityPath: fieldPath('sku') || fieldPath('products_no'),
    pricePath: fieldPath('price') || fieldPath('sell_price'),
    originalPricePath: fieldPath('original_price'),
  })
  priceValidation.value = null
  priceValidatedFingerprint.value = ''
  priceError.value = ''
  document.querySelector('.builder-section')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

async function handleDelete(rule) {
  if (!window.confirm(`确定删除规则「${rule.name}」吗？`)) return
  try {
    await deleteScanRule(rule.id)
    rules.value = rules.value.filter(item => item.id !== rule.id)
    showSuccess('规则已删除')
  } catch (error) {
    showError('删除规则失败: ' + errorMessage(error))
  }
}

async function handleExport() {
  exporting.value = true
  try {
    const response = await exportScanRules()
    if (response.code !== 0 || !response.data) throw new Error(response.message || '导出失败')
    const blob = new Blob([JSON.stringify(response.data, null, 2)], { type: 'application/json;charset=utf-8' })
    const link = document.createElement('a')
    link.href = URL.createObjectURL(blob)
    link.download = `gentry-scan-rules-${new Date().toISOString().slice(0, 10)}.json`
    link.click()
    URL.revokeObjectURL(link.href)
    showSuccess(`已导出 ${rules.value.length} 条规则`)
  } catch (error) {
    showError('导出规则失败: ' + errorMessage(error))
  } finally {
    exporting.value = false
  }
}

async function handleImportFile(event) {
  const file = event.target.files?.[0]
  if (!file) return
  importing.value = true
  try {
    const document = JSON.parse(await file.text())
    const response = await importScanRules(document)
    if (response.code !== 0) throw new Error(response.message || '导入失败')
    const imported = response.data?.imported || 0
    const skipped = response.data?.skipped || 0
    showSuccess(`已导入 ${imported} 条${skipped ? `，跳过 ${skipped} 条同名规则` : ''}`)
    await loadRules()
  } catch (error) {
    showError('导入规则失败: ' + errorMessage(error))
  } finally {
    importing.value = false
    event.target.value = ''
  }
}

function resetBuilder() {
  url.value = ''
  keywords.value = ''
  scanResult.value = null
  selectedIndex.value = null
  scanned.value = false
  ruleName.value = ''
  scopeType.value = builderMode.value === 'price_api' ? 'section' : 'exact'
  Object.assign(priceDraft, createEmptyPriceRuleDraft())
  priceValidation.value = null
  priceValidatedFingerprint.value = ''
  priceError.value = ''
  editingRule.value = null
}

function addPriceVariable() {
  priceDraft.variables.push({ name: '', selector: '', attr: '' })
}

function removePriceVariable(index) {
  priceDraft.variables.splice(index, 1)
}

function candidateKey(candidate, index) {
  return `${candidate.config?.container || ''}:${candidate.config?.item || ''}:${index}`
}

function scopeName(rule) {
  if (rule.scope_type === 'exact') return '页面'
  if (rule.scope_type === 'route') return '路由'
  if (rule.scope_type === 'section') return '同站目录'
  if (rule.scope_type === 'global') return '通用'
  return '旧版'
}

function scopeTarget(rule) {
  if (rule.scope_type === 'global') return '所有网站中结构相同的页面'
  return rule.source_url || `URL 包含 ${rule.url_contains}`
}

function isAPIRule(rule) {
  if (!rule.fetch_config) return false
  if (typeof rule.fetch_config === 'object') return rule.fetch_config.mode === 'api_json'
  try { return JSON.parse(rule.fetch_config).mode === 'api_json' } catch { return false }
}

function strategyLabel(strategy) {
  if (!strategy) return ''
  if (strategy.startsWith('template_')) return `规则「${strategy.slice(9)}」`
  const labels = {
    keyword_ancestor: '关键词定位',
    repeated_list: '重复列表',
    link_cluster: '链接簇',
    table_rows: '表格检测',
  }
  return labels[strategy] || strategy
}

function strategyClass(strategy) {
  return strategy?.startsWith('template_') || strategy?.startsWith('rule_') ? 'strategy-rule' : 'strategy-heuristic'
}

function errorMessage(error) {
  return error.response?.data?.message || error.message || '操作失败'
}
</script>

<style scoped>
.scan-rules-page { max-width: 1120px; }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; margin-bottom: 1.75rem; }
.page-header h1 { margin: 0; color: var(--text); font-size: 1.5rem; font-weight: 700; }
.page-header p { margin-top: 0.3rem; color: var(--text-secondary); font-size: 0.8125rem; }
.header-actions { display: flex; gap: 0.5rem; flex-shrink: 0; }
.header-actions svg, .scan-button svg { width: 16px; height: 16px; }
.file-input { display: none; }

.builder-section, .library-section { padding: 0 0 1.75rem; }
.builder-section { border-bottom: 1px solid var(--border); }
.library-section { padding-top: 1.75rem; }
.section-title-row { display: flex; align-items: center; justify-content: space-between; gap: 1rem; margin-bottom: 1rem; }
.section-title-row h2 { margin: 0; color: var(--text); font-size: 1rem; font-weight: 700; }
.result-count, .library-title > span { color: var(--text-muted); font-size: 0.75rem; }
.builder-mode { display: inline-grid; grid-template-columns: repeat(2, minmax(0, 1fr)); margin-bottom: 1rem; overflow: hidden; border: 1px solid var(--border); border-radius: 6px; }
.builder-mode button { min-height: 36px; padding: 0 1rem; border: 0; border-right: 1px solid var(--border); background: var(--bg-elevated); color: var(--text-secondary); cursor: pointer; font-size: 0.8125rem; }
.builder-mode button:last-child { border-right: 0; }
.builder-mode button.active { background: var(--green); color: #000; font-weight: 700; }

.scan-form { display: grid; grid-template-columns: minmax(280px, 2fr) minmax(200px, 1fr) auto; align-items: end; gap: 0.75rem; }
.scan-form .form-group { min-width: 0; margin: 0; }
.scan-button { height: 38px; padding-inline: 1.2rem; }
.inline-error { margin-top: 0.75rem; color: var(--error); font-size: 0.8125rem; }
.scan-loading { display: flex; align-items: center; justify-content: center; gap: 0.75rem; min-height: 120px; color: var(--text-secondary); font-size: 0.8125rem; }
.scan-loading .spinner { width: 22px; height: 22px; margin: 0; border-width: 2px; }
.empty-result { margin-top: 1rem; padding: 1.5rem 0; border-top: 1px solid var(--border-light); color: var(--text-secondary); text-align: center; font-size: 0.8125rem; }

.candidate-list { display: grid; gap: 0.6rem; margin-top: 1rem; }
.candidate-row { display: grid; grid-template-columns: 24px minmax(0, 1fr); gap: 0.75rem; width: 100%; padding: 0.85rem; border: 1px solid var(--border); border-radius: 6px; background: var(--bg-surface); color: var(--text); text-align: left; cursor: pointer; transition: var(--transition); }
.candidate-row:hover { border-color: var(--text-muted); background: var(--bg-hover); }
.candidate-row.selected { border-color: var(--green); box-shadow: 0 0 0 1px var(--green) inset; }
.choice-mark { display: inline-flex; align-items: center; justify-content: center; width: 20px; height: 20px; margin-top: 1px; border: 1px solid var(--border); border-radius: 50%; color: #000; background: var(--bg-elevated); }
.candidate-row.selected .choice-mark { border-color: var(--green); background: var(--green); }
.choice-mark svg { width: 13px; height: 13px; }
.candidate-content { min-width: 0; }
.candidate-heading { display: flex; align-items: center; gap: 0.65rem; margin-bottom: 0.55rem; font-size: 0.75rem; color: var(--text-secondary); }
.candidate-heading strong { color: var(--text); font-size: 0.875rem; }
.candidate-strategy { font-size: 0.6875rem; padding: 0.1rem 0.5rem; border-radius: var(--radius-pill); white-space: nowrap; }
.candidate-strategy.strategy-rule { color: #fff; background: #e74c3c; font-weight: 700; }
.candidate-strategy.strategy-heuristic { color: var(--accent); background: var(--bg-elevated); }
.sample-list { display: grid; gap: 0.2rem; }
.sample-line { display: flex; justify-content: space-between; gap: 1rem; min-width: 0; padding: 0.28rem 0.5rem; border-radius: 4px; background: var(--bg-elevated); font-size: 0.8125rem; }
.sample-title { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sample-date { flex-shrink: 0; color: var(--text-muted); font-size: 0.75rem; }
.selector-line { display: flex; gap: 0.4rem; min-width: 0; margin-top: 0.55rem; color: var(--text-muted); font-size: 0.6875rem; }
.selector-line code { overflow: hidden; color: var(--text-secondary); text-overflow: ellipsis; white-space: nowrap; }

.save-panel { display: grid; grid-template-columns: minmax(220px, 1fr) auto minmax(180px, 1fr) auto; align-items: end; gap: 0.75rem; margin-top: 1rem; padding-top: 1rem; border-top: 1px solid var(--border-light); }
.save-panel .form-group { margin: 0; }
.field-label { display: block; margin-bottom: 0.35rem; color: var(--text-secondary); font-size: 0.75rem; font-weight: 700; }
.scope-control { display: grid; grid-template-columns: repeat(3, auto); overflow: hidden; border: 1px solid var(--border); border-radius: 6px; }
.scope-control button { min-height: 38px; padding: 0 0.8rem; border: 0; border-right: 1px solid var(--border); background: var(--bg-elevated); color: var(--text-secondary); cursor: pointer; font-size: 0.75rem; white-space: nowrap; }
.scope-control button:last-child { border-right: 0; }
.scope-control button.active { background: var(--green); color: #000; font-weight: 700; }
.scope-control button:disabled { opacity: 0.4; cursor: not-allowed; }
.scope-summary { align-self: center; min-width: 0; overflow: hidden; color: var(--text-muted); font-size: 0.75rem; text-overflow: ellipsis; white-space: nowrap; }
.save-button { height: 38px; }

.price-builder { display: grid; gap: 0.85rem; }

.price-builder-hint {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  padding: 0.75rem;
  background: var(--bg-elevated);
  border-radius: var(--radius-lg);
  font-size: 0.8125rem;
  color: var(--text-secondary);
  line-height: 1.4;
}
.price-builder-hint svg { flex-shrink: 0; margin-top: 1px; color: var(--text-muted); }
.price-grid { display: grid; gap: 0.75rem; }
.price-grid .form-group { min-width: 0; margin: 0; }
.primary-fields { grid-template-columns: minmax(220px, 1fr) minmax(320px, 2fr); }
.source-fields { grid-template-columns: minmax(300px, 2fr) repeat(3, minmax(140px, 1fr)); }
.mapping-fields, .header-fields { grid-template-columns: repeat(4, minmax(0, 1fr)); }
.variable-section { padding: 0.85rem 0; border-top: 1px solid var(--border-light); border-bottom: 1px solid var(--border-light); }
.variable-heading { display: flex; align-items: center; justify-content: space-between; gap: 1rem; }
.variable-heading > div { display: grid; gap: 0.2rem; }
.variable-heading strong { font-size: 0.8125rem; }
.variable-heading span { color: var(--text-muted); font-size: 0.75rem; }
.variable-row { display: grid; grid-template-columns: minmax(140px, 0.7fr) minmax(220px, 1.5fr) minmax(180px, 1fr) auto; align-items: end; gap: 0.75rem; margin-top: 0.75rem; }
.variable-row .form-group { min-width: 0; margin: 0; }
.variable-remove { margin-bottom: 2px; }
.price-preview { padding: 0.85rem 0; border-top: 1px solid var(--border-light); border-bottom: 1px solid var(--border-light); }
.preview-heading { display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.55rem; font-size: 0.8125rem; }
.preview-heading span { color: var(--text-muted); font-size: 0.75rem; }
.sample-price { flex-shrink: 0; color: var(--green); font-variant-numeric: tabular-nums; }
.price-actions { display: grid; grid-template-columns: auto minmax(160px, 1fr) repeat(3, auto); align-items: end; gap: 0.75rem; padding-top: 0.15rem; }
.price-actions .btn { height: 38px; }

.rule-list { border-top: 1px solid var(--border); }
.rule-row { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: 1rem 0; border-bottom: 1px solid var(--border-light); }
.rule-main { min-width: 0; }
.rule-heading { display: flex; align-items: center; flex-wrap: wrap; gap: 0.5rem; }
.rule-heading strong { font-size: 0.875rem; }
.source-badge, .scope-badge, .disabled-badge { padding: 0.14rem 0.45rem; border-radius: 4px; font-size: 0.6875rem; font-weight: 700; }
.source-badge { color: var(--text-secondary); background: var(--bg-elevated); }
.source-badge.api { color: var(--green); }
.scope-badge { background: var(--bg-elevated); color: var(--text-secondary); }
.scope-global { color: var(--green); }
.disabled-badge { color: var(--warning); background: var(--warning-bg); }
.rule-target { margin-top: 0.35rem; overflow: hidden; color: var(--text-secondary); font-size: 0.8125rem; text-overflow: ellipsis; white-space: nowrap; }
.rule-structure { display: flex; gap: 0.4rem; min-width: 0; margin-top: 0.35rem; color: var(--text-muted); font-size: 0.6875rem; }
.rule-structure code { max-width: 280px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.field-count { flex-shrink: 0; margin-left: 0.35rem; }
.icon-button { display: inline-flex; align-items: center; justify-content: center; width: 34px; height: 34px; flex: 0 0 34px; border: 0; border-radius: 50%; background: transparent; color: var(--text-muted); cursor: pointer; }
.icon-button:hover { background: var(--error-bg); color: var(--error); }
.icon-button svg { width: 17px; height: 17px; }
.rule-actions { display: flex; align-items: center; flex-shrink: 0; }
.list-state { padding: 2.5rem 0; color: var(--text-secondary); text-align: center; font-size: 0.8125rem; }

@media (max-width: 900px) {
  .scan-form { grid-template-columns: 1fr 1fr; }
  .scan-button { grid-column: 1 / -1; justify-self: start; }
  .save-panel { grid-template-columns: 1fr 1fr; }
  .source-fields, .mapping-fields, .header-fields, .variable-row { grid-template-columns: 1fr 1fr; }
  .price-actions { grid-template-columns: 1fr 1fr; }
  .price-actions .scope-summary { order: initial; }
  .scope-summary { order: 3; }
  .save-button { order: 4; justify-self: end; }
}

@media (max-width: 640px) {
  .page-header { align-items: stretch; flex-direction: column; }
  .header-actions { align-self: flex-start; }
  .builder-mode { width: 100%; }
  .scan-form, .save-panel, .primary-fields, .source-fields, .mapping-fields, .header-fields, .variable-row, .price-actions { grid-template-columns: 1fr; }
  .variable-heading { align-items: stretch; flex-direction: column; }
  .variable-remove { justify-self: end; }
  .scan-button, .save-button { width: 100%; justify-self: stretch; }
  .price-actions .btn { width: 100%; }
  .scope-control { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .scope-control button { padding-inline: 0.35rem; white-space: normal; }
  .scope-summary, .save-button { order: initial; }
  .candidate-heading { align-items: flex-start; flex-wrap: wrap; }
  .sample-date { display: none; }
  .rule-structure code { max-width: 120px; }
}
</style>
