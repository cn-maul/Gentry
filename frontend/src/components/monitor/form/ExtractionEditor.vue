<template>
  <div class="settings-section">
    <div class="section-header">
      <h2>网页内容提取</h2>
    </div>

    <p class="section-desc">输入关键词，系统自动扫描网页中的列表区域和内容字段。</p>

    <div class="scan-row">
      <div class="form-group scan-keyword">
        <label>关键词（多个用逗号隔开）</label>
        <input v-model="keyword" class="form-input" :placeholder="keywordPlaceholder" @keyup.enter="handleScan" />
      </div>
      <div class="scan-action">
        <button class="btn btn-primary btn-sm" :disabled="!url || scanning" @click="handleScan">
          {{ scanning ? '扫描中...' : '预扫描' }}
        </button>
      </div>
    </div>

    <div class="loading" v-if="scanning"><div class="spinner" /><p>扫描网页内容...</p></div>

    <div class="scan-results" v-else-if="scanResult && scanResult.containers && scanResult.containers.length > 0">
      <p class="results-label">发现 {{ scanResult.containers.length }} 个数据区域，选择一个应用：</p>
      <div
        v-for="(container, ci) in scanResult.containers"
        :key="ci"
        class="candidate-card"
        :class="{ selected: selectedIndex === ci }"
        @click="selectedIndex = ci"
      >
        <div class="candidate-header">
          <div class="candidate-info">
            <span class="candidate-badge">{{ container.container_tag.toUpperCase() }}</span>
            <span class="candidate-count">{{ container.item_count }} 条</span>
            <span class="candidate-hit" v-if="container.keyword_hits">命中 {{ container.keyword_hits }} 个关键词</span>
            <span class="candidate-strategy" :class="strategyClass(container.strategy)" v-if="container.strategy">{{ strategyLabel(container.strategy) }}</span>
          </div>
          <button class="btn btn-sm btn-primary apply-btn" type="button" @click.stop="applyCandidate(container)">应用</button>
        </div>
        <div class="candidate-selector">
          <code>{{ container.config?.container || container.container_css }}</code>
          <span class="selector-sep"> / </span>
          <code>{{ container.config?.item || container.item_css || '单项' }}</code>
        </div>
        <div class="candidate-samples">
          <div v-for="(item, ii) in (container.sample_items || []).slice(0, 5)" :key="ii" class="sample-item">
            <span class="sample-title">{{ item.title || item.sku || '未命名条目' }}</span>
            <span class="sample-meta" v-if="item.price">{{ item.price }}</span>
            <span class="sample-meta" v-else-if="item.date">{{ item.date }}</span>
          </div>
          <div class="sample-more" v-if="container.item_count > 5">...还有 {{ container.item_count - 5 }} 条</div>
        </div>
      </div>
    </div>

    <div class="empty-scan" v-else-if="scanned">
      <p>未找到匹配内容，试试不同关键词或展开高级设置手动配置选择器</p>
    </div>

    <div class="form-error" v-if="scanError">{{ scanError }}</div>

    <div class="advanced-toggle" @click="showAdvanced = !showAdvanced">
      <svg :class="{ rotated: showAdvanced }" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><polyline points="9 18 15 12 9 6"/></svg>
      <span>高级设置</span>
    </div>

    <div class="manual-section" v-if="showAdvanced">
      <div class="form-group">
        <label>数据源</label>
        <select :value="modelValue.sourceMode || 'html'" @change="updateSourceMode($event.target.value)" class="form-input">
          <option value="html">网页 HTML</option>
          <option value="api_json">公开 JSON API</option>
        </select>
      </div>

      <template v-if="modelValue.sourceMode === 'api_json'">
        <div class="source-grid">
          <div class="form-group source-url">
            <label>JSON API URL</label>
            <input :value="modelValue.sourceUrl" @input="update('sourceUrl', $event.target.value)" class="form-input" placeholder="https://example.com/api/products" />
          </div>
          <div class="form-group">
            <label>列表路径</label>
            <input :value="modelValue.itemsPath" @input="update('itemsPath', $event.target.value)" class="form-input" placeholder="data.items" />
          </div>
          <div class="form-group">
            <label>过滤字段（可选）</label>
            <input :value="modelValue.filterPath" @input="update('filterPath', $event.target.value)" class="form-input" placeholder="is_selling" />
          </div>
          <div class="form-group">
            <label>过滤值</label>
            <input :value="modelValue.filterEquals" @input="update('filterEquals', $event.target.value)" class="form-input" placeholder="true" />
          </div>
        </div>
        <div class="header-grid">
          <div class="form-group">
            <label>Accept</label>
            <input :value="modelValue.sourceHeaders?.Accept || ''" @input="updateSourceHeader('Accept', $event.target.value)" class="form-input" placeholder="application/json" />
          </div>
          <div class="form-group">
            <label>Accept-Language</label>
            <input :value="modelValue.sourceHeaders?.['Accept-Language'] || ''" @input="updateSourceHeader('Accept-Language', $event.target.value)" class="form-input" placeholder="zh-CN,zh;q=0.9" />
          </div>
          <div class="form-group">
            <label>Referer</label>
            <input :value="modelValue.sourceHeaders?.Referer || ''" @input="updateSourceHeader('Referer', $event.target.value)" class="form-input" :placeholder="url || 'https://example.com/product'" />
          </div>
          <div class="form-group">
            <label>X-Requested-With</label>
            <input :value="modelValue.sourceHeaders?.['X-Requested-With'] || ''" @input="updateSourceHeader('X-Requested-With', $event.target.value)" class="form-input" placeholder="XMLHttpRequest" />
          </div>
        </div>
      </template>

      <div class="form-group">
        <label>{{ modelValue.sourceMode === 'api_json' ? '列表路径（兼容字段）' : '容器选择器' }}</label>
        <input :value="modelValue.containerSelector" @input="update('containerSelector', $event.target.value)" class="form-input" placeholder="如 div.hap_infoBox" />
      </div>

      <div class="form-group">
        <label>{{ modelValue.sourceMode === 'api_json' ? '条目表达式' : '列表项选择器（可选）' }}</label>
        <input :value="modelValue.itemSelector" @input="update('itemSelector', $event.target.value)" class="form-input" placeholder="如 div.hap_infoOne" />
      </div>

      <div class="fields-section">
        <div class="fields-header">
          <span class="fields-title">提取字段</span>
          <button class="btn btn-sm btn-ghost" @click="addField">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
            添加字段
          </button>
        </div>

        <div class="fields-list">
          <div v-for="(field, index) in modelValue.fields" :key="index" class="field-card">
            <div class="field-grid">
              <div class="form-group">
                <label>名称</label>
                <input :value="field.name" @input="updateField(index, 'name', $event.target.value)" class="form-input" placeholder="如 title" />
              </div>
              <div class="form-group">
                <label>{{ modelValue.sourceMode === 'api_json' ? 'JSON 路径' : '选择器' }}</label>
                <input :value="field.selector" @input="updateField(index, 'selector', $event.target.value)" class="form-input" placeholder="如 a.title" />
              </div>
              <div class="form-group">
                <label>类型</label>
                <select :value="field.type" @change="updateField(index, 'type', $event.target.value)" class="form-input">
                  <option value="text">文本 (text)</option>
                  <option value="attr">属性 (attr)</option>
                </select>
              </div>
              <div class="form-group" v-if="field.type === 'attr'">
                <label>属性名</label>
                <input :value="field.attr" @input="updateField(index, 'attr', $event.target.value)" class="form-input" placeholder="默认 href" />
              </div>
              <div class="form-group">
                <label>转换</label>
                <input :value="field.transform" @input="updateField(index, 'transform', $event.target.value)" class="form-input" placeholder="如 trim([])" />
              </div>
            </div>
            <button class="field-remove-btn" title="删除字段" @click="removeField(index)">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            </button>
          </div>
        </div>

        <div class="empty" v-if="modelValue.fields.length === 0">
          <p>暂无字段，点击上方按钮添加</p>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { previewScan } from '../../../api/monitors'

const props = defineProps({
  modelValue: { type: Object, required: true },
  url: { type: String, default: '' },
})
const emit = defineEmits(['update:modelValue'])

const keyword = ref('')
const scanning = ref(false)
const scanResult = ref(null)
const scanned = ref(false)
const scanError = ref(null)
const selectedIndex = ref(null)
const showAdvanced = ref(false)

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

const keywordPlaceholder = computed(() => '公告,通知,公示')

async function handleScan() {
  if (!props.url) return
  scanning.value = true
  scanned.value = false
  scanError.value = null
  selectedIndex.value = null
  try {
    const res = await previewScan({
      url: props.url.trim(),
      keywords: keyword.value || keywordPlaceholder.value,
      strategy_type: 'presence',
    })
    if (res.code === 0 && res.data) {
      scanResult.value = res.data
    } else {
      scanError.value = res.message || '扫描失败'
    }
  } catch (e) {
    scanError.value = e.response?.data?.message || e.message || '扫描失败'
  }
  scanned.value = true
  scanning.value = false
}

function applyCandidate(container) {
  const config = container.config || {}
  const fields = (config.fields || []).map(f => ({
    name: f.name || '', selector: f.selector || '', type: f.type || 'text', attr: f.attr || '', transform: f.transform || '',
  }))
  const extracted = {
    ...props.modelValue,
    containerSelector: config.container || container.container_css || '',
    itemSelector: config.item || container.item_css || '',
  }
  if (config.fetch_config) {
    extracted.sourceMode = config.fetch_config.mode || 'html'
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
  emit('update:modelValue', extracted)
  scanned.value = false
  scanResult.value = null
}

function update(key, value) {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
}

function updateSourceMode(mode) {
  const next = { ...props.modelValue, sourceMode: mode }
  if (mode === 'html') {
    next.sourceUrl = ''
    next.itemsPath = ''
    next.filterPath = ''
    next.filterEquals = ''
    next.sourceHeaders = {}
  }
  emit('update:modelValue', next)
}

function updateSourceHeader(name, value) {
  const headers = { ...(props.modelValue.sourceHeaders || {}) }
  if (value.trim()) headers[name] = value
  else delete headers[name]
  emit('update:modelValue', { ...props.modelValue, sourceHeaders: headers })
}

function updateField(index, key, value) {
  const fields = [...props.modelValue.fields]
  fields[index] = { ...fields[index], [key]: value }
  emit('update:modelValue', { ...props.modelValue, fields })
}

function addField() {
  const fields = [...props.modelValue.fields, { name: '', selector: '', type: 'text', attr: '', transform: '' }]
  emit('update:modelValue', { ...props.modelValue, fields })
}

function removeField(index) {
  const fields = [...props.modelValue.fields]
  fields.splice(index, 1)
  emit('update:modelValue', { ...props.modelValue, fields })
}
</script>

<style scoped>
.section-header { margin-bottom: 0.5rem; }
.section-desc { font-size: 0.8125rem; color: var(--text-secondary); margin-bottom: 1rem; }

.scan-row {
  display: flex;
  align-items: flex-end;
  gap: 0.75rem;
  margin-bottom: 1rem;
}
.scan-keyword { flex: 1; margin-bottom: 0; }
.scan-action { flex-shrink: 0; padding-bottom: 2px; }

.scan-results { margin-bottom: 1rem; }
.results-label { font-size: 0.8125rem; color: var(--text-secondary); margin-bottom: 0.5rem; }

.candidate-card {
  background: var(--bg-card);
  border: 2px solid transparent;
  border-radius: var(--radius-lg);
  padding: 0.85rem;
  margin-bottom: 0.5rem;
  cursor: pointer;
  transition: var(--transition);
}
.candidate-card:hover { background: var(--bg-hover); }
.candidate-card.selected { border-color: var(--green); background: rgba(29, 185, 84, 0.04); }

.candidate-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.4rem; }
.candidate-info { display: flex; align-items: center; gap: 0.5rem; }
.candidate-badge { font-size: 0.6875rem; font-weight: 700; color: var(--text); background: var(--bg-elevated); padding: 0.15rem 0.5rem; border-radius: var(--radius-pill); }
.candidate-count { font-size: 0.75rem; color: var(--text-secondary); }
.candidate-hit { font-size: 0.75rem; color: var(--green); }
.candidate-strategy { font-size: 0.6875rem; padding: 0.1rem 0.5rem; border-radius: var(--radius-pill); white-space: nowrap; }
.candidate-strategy.strategy-rule { color: #fff; background: #e74c3c; font-weight: 700; }
.candidate-strategy.strategy-heuristic { color: var(--accent); background: var(--bg-elevated); }
.apply-btn { flex-shrink: 0; }

.candidate-selector { margin-bottom: 0.5rem; font-size: 0.75rem; color: var(--text-muted); }
.candidate-selector code { color: var(--green); }
.selector-sep { color: var(--text-muted); }

.candidate-samples { display: flex; flex-direction: column; gap: 0.15rem; }
.sample-item { display: flex; justify-content: space-between; padding: 0.25rem 0.5rem; border-radius: 4px; background: var(--bg-elevated); font-size: 0.8125rem; }
.sample-title { color: var(--text); flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sample-meta { color: var(--text-muted); font-size: 0.75rem; margin-left: 0.5rem; flex-shrink: 0; }
.sample-more { font-size: 0.75rem; color: var(--text-muted); text-align: center; padding: 0.2rem; }

.empty-scan { text-align: center; padding: 1.5rem; color: var(--text-secondary); font-size: 0.8125rem; }

.divider { height: 1px; background: var(--border-light); margin: 1.25rem 0; }

.advanced-toggle {
  display: inline-flex; align-items: center; gap: 0.35rem;
  cursor: pointer; font-size: 0.75rem; font-weight: 700; color: var(--text-secondary);
  text-transform: uppercase; letter-spacing: 1px; margin-bottom: 0.75rem;
  user-select: none; transition: var(--transition);
}
.advanced-toggle:hover { color: var(--text); }
.advanced-toggle svg { transition: transform 0.2s; }
.advanced-toggle svg.rotated { transform: rotate(90deg); }

.manual-section { margin-top: 0.75rem; }
.source-grid { display: grid; grid-template-columns: minmax(0, 2fr) repeat(3, minmax(0, 1fr)); gap: 0.5rem; }
.source-grid .form-group { min-width: 0; }
.header-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 0.5rem; }
.header-grid .form-group { min-width: 0; }

.fields-section { margin-top: 0.5rem; }
.fields-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.75rem; }
.fields-title { font-size: 0.75rem; font-weight: 700; color: var(--text-secondary); text-transform: uppercase; letter-spacing: 1px; }
.fields-list { display: flex; flex-direction: column; gap: 0.5rem; }
.field-card { display: flex; gap: 0.5rem; align-items: flex-start; padding: 0.75rem; background: var(--bg-surface); border-radius: var(--radius-lg); }
.field-grid { flex: 1; display: grid; grid-template-columns: 1fr 1fr 1fr 1fr 1fr; gap: 0.5rem; }
.field-remove-btn {
  background: none; border: none; cursor: pointer; padding: 0.4rem;
  border-radius: var(--radius-circle); transition: var(--transition); color: var(--text-muted);
  display: inline-flex; align-items: center; flex-shrink: 0; margin-top: 1.5rem;
}
.field-remove-btn:hover { background: var(--error-bg); color: var(--error); }

@media (max-width: 768px) {
  .scan-row { flex-direction: column; align-items: stretch; }
  .field-grid { grid-template-columns: 1fr 1fr; }
  .source-grid, .header-grid { grid-template-columns: 1fr; }
}
</style>
