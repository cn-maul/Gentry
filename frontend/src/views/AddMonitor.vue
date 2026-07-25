<template>
  <div class="add-monitor">
    <div class="page-header">
      <div>
        <router-link to="/" class="back-btn">← 返回</router-link>
        <h1>{{ pageTitle }}</h1>
      </div>
    </div>

    <!-- ===== Loading ===== -->
    <div class="loading" v-if="isEdit && loading">
      <div class="spinner" />
      <p>加载配置...</p>
    </div>

    <!-- ===== Form ===== -->
    <template v-else>
      <MonitorForm
        :form="form"
        :showTypeSelector="true"
        :accounts="accounts"
        :error="submitError"
        :validationResult="validationResult"
        :validationLoading="validationLoading"
        :showBaselineWarning="baselineWarning"
        @validate="runValidation"
        @update:form="onFormUpdate"
        @change:type="changeMonitorType"
      >
        <template #actions>
          <button v-if="!isEdit" class="btn btn-ghost btn-sm" @click="handleSaveAsRule" :disabled="submitting" style="margin-right: auto;">{{ form.monitorType === 'field_transition' ? '另存为价格规则' : '另存为网页规则' }}</button>
          <button class="btn btn-primary" :disabled="submitting" @click="handleSubmit">
            {{ submitting ? '提交中...' : (isEdit ? '保存修改' : '创建并启动') }}
          </button>
        </template>
      </MonitorForm>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, reactive, onMounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { createMonitor, updateMonitor, fetchMonitorConfig, fetchAccounts, quickCreateScanRule, validateMonitorConfig } from '../api/monitors'
import MonitorForm from '../components/monitor/form/MonitorForm.vue'
import { createEmptyForm, toMonitorRequest, fromMonitorResponse, hasSemanticChange, validateForm, getDetectionFingerprint } from '../composables/useMonitorForm'
import { useToastMessages } from '../composables/useToastMessages'

const router = useRouter()
const route = useRoute()
const { showSuccess, showError } = useToastMessages()

const isEdit = computed(() => !!route.params.name)
const loading = ref(false)
const submitting = ref(false)
const submitError = ref(null)

const form = reactive(createEmptyForm())
const originalFormSnapshot = ref(null)
const workflowDrafts = {
  presence: null,
  field_transition: null,
}

const pageTitle = computed(() => {
  const typeName = form.monitorType === 'field_transition' ? '商品价格监控' : '网页监控'
  return `${isEdit.value ? '编辑' : '新增'}${typeName}`
})

const accounts = ref([])

// Validation
const validationResult = ref(null)
const validationLoading = ref(false)
const validatedFingerprint = ref('')
const validationAttemptFingerprint = ref('')

// Baseline warning
const baselineWarning = ref(false)

function onFormUpdate(newForm) {
  Object.assign(form, newForm)
}

function workflowState(source) {
  return JSON.parse(JSON.stringify({ extraction: source.extraction, rule: source.rule }))
}

function changeMonitorType(nextType) {
  if (nextType === form.monitorType) return
  workflowDrafts[form.monitorType] = workflowState(form)
  const stored = workflowDrafts[nextType]
  const next = stored || workflowState(createEmptyForm())
  form.extraction = next.extraction
  form.rule = next.rule
  if (nextType === 'field_transition' && !form.extraction.containerSelector.trim()) {
    form.extraction.containerSelector = 'body'
  }
  if (nextType === 'field_transition') {
    const titleField = form.extraction.fields.find(field => field.name === 'title')
    if (titleField?.selector === 'a') titleField.selector = 'h1'
  }
  form.monitorType = nextType
  submitError.value = null
  validationResult.value = null
  validatedFingerprint.value = ''
  validationAttemptFingerprint.value = ''
}

// Watch for semantic changes in edit mode
watch(() => form.monitorType, (monitorType) => {
  if (monitorType === 'field_transition') {
    form.rule.pageMode = form.extraction.itemSelector.trim() ? 'list' : 'single'
    if (form.rule.pageMode === 'list' && form.rule.identity.mode === 'source_url') {
      form.rule.identity = { mode: 'field', field: '' }
    }
    ensurePriceField()
  }
})

watch(
  () => [form.monitorType, form.extraction.sourceMode, form.extraction.itemSelector, form.extraction.fields.map(field => field.name).join('|')],
  () => {
    if (form.monitorType !== 'field_transition') return
    const isList = form.extraction.sourceMode === 'api_json' || Boolean(form.extraction.itemSelector.trim())
    form.rule.pageMode = isList ? 'list' : 'single'
    if (isList && form.rule.identity.mode === 'source_url') {
      const identityField = form.extraction.fields.find(field => ['sku', 'products_no', 'id', 'url'].includes(field.name))
      if (identityField) form.rule.identity = { mode: 'field', field: identityField.name }
    }
  },
)

function ensurePriceField() {
  if (form.monitorType !== 'field_transition') return
  const currentTarget = form.rule.target.field || 'price'
  if (!form.extraction.fields.some(field => field.name === currentTarget)) {
    form.extraction.fields.push({ name: currentTarget, selector: '.price', type: 'text', attr: '', transform: '' })
  }
  form.rule.target.field = currentTarget
  form.rule.target.valueType = 'money'
  if (!['decreased', 'at_or_below'].includes(form.rule.transition.operator)) {
    form.rule.transition.operator = 'decreased'
  }
}

watch(
  () => getDetectionFingerprint(form),
  (fingerprint) => {
    if (validatedFingerprint.value && validatedFingerprint.value !== fingerprint) {
      validatedFingerprint.value = ''
      validationResult.value = null
    }
    baselineWarning.value = Boolean(
      isEdit.value && originalFormSnapshot.value && hasSemanticChange(originalFormSnapshot.value, form),
    )
  }
)

watch(
  () => JSON.stringify(toMonitorRequest(form)),
  (fingerprint) => {
    if (validationAttemptFingerprint.value && validationAttemptFingerprint.value !== fingerprint) {
      validationAttemptFingerprint.value = ''
      validationResult.value = null
    }
  }
)

// Load accounts and edit data
onMounted(async () => {
  try {
    const acctRes = await fetchAccounts()
    accounts.value = (acctRes.code === 0 ? acctRes.data : []) || []
  } catch { /* ignore */ }

  if (isEdit.value) {
    loading.value = true
    try {
      const res = await fetchMonitorConfig(route.params.name)
      if (res.code === 0 && res.data) {
        const loaded = fromMonitorResponse(res.data)
        Object.assign(form, loaded)
        originalFormSnapshot.value = JSON.parse(JSON.stringify(form))
      } else {
        submitError.value = res.message || '加载配置失败'
      }
    } catch (e) {
      submitError.value = '加载配置失败: ' + e.message
    } finally {
      loading.value = false
    }
  }
})

// Validate before submit
async function runValidation() {
  validationAttemptFingerprint.value = JSON.stringify(toMonitorRequest(form))
  const localError = validateForm(form)
  if (localError) {
    validationResult.value = { valid: false, errors: [localError], summary: '请先修正表单配置后再验证。' }
    submitError.value = localError
    return false
  }
  validationLoading.value = true
  submitError.value = null
  try {
    const payload = toMonitorRequest(form)
    const res = await validateMonitorConfig(payload)
    if (res.code === 0 && res.data) {
      validationResult.value = res.data
      validatedFingerprint.value = getDetectionFingerprint(form)
      return true
    }
    const message = res.message || '验证失败'
    validationResult.value = { valid: false, errors: [message], summary: '配置未通过验证。' }
    return false
  } catch (e) {
    const message = e.response?.data?.message || e.message || '验证失败'
    validationResult.value = { valid: false, errors: [message], summary: '配置未通过验证。' }
    return false
  } finally {
    validationLoading.value = false
  }
}

async function handleSubmit() {
  const err = validateForm(form)
  if (err) { submitError.value = err; return }
  const semanticChange = !isEdit.value || !originalFormSnapshot.value || hasSemanticChange(originalFormSnapshot.value, form)
  if (form.monitorType === 'field_transition' && semanticChange && validatedFingerprint.value !== getDetectionFingerprint(form)) {
    const valid = await runValidation()
    if (!valid) {
      submitError.value = '价格监控必须先通过配置验证才能保存'
      return
    }
  }
  submitError.value = null
  submitting.value = true
  try {
    const payload = toMonitorRequest(form)
    if (isEdit.value) {
      await updateMonitor(route.params.name, payload)
    } else {
      await createMonitor(payload)
    }
    router.push('/')
  } catch (e) {
    submitError.value = e.response?.data?.message || e.message
  } finally {
    submitting.value = false
  }
}

async function handleSaveAsRule() {
  if (!form.extraction.containerSelector) return
  const example = form.monitorType === 'field_transition' ? '商城商品 SKU' : '公告列表'
  const name = prompt(`规则名称（如 ${example}）`, form.basic.name.trim() + ' 规则')
  if (!name) return
  try {
    await quickCreateScanRule({
      name,
      url: form.basic.url,
      scope_type: 'exact',
      config: {
        container: form.extraction.containerSelector,
        item: form.extraction.itemSelector,
        fetch_config: form.extraction.sourceMode === 'api_json' ? {
          mode: 'api_json',
          url: form.extraction.sourceUrl,
          items_path: form.extraction.itemsPath,
          filter_path: form.extraction.filterPath,
          filter_equals: String(form.extraction.filterEquals ?? ''),
          headers: form.extraction.sourceHeaders || {},
        } : undefined,
        fields: form.extraction.fields.filter(f => f.name).map(f => ({
          name: f.name, selector: f.selector, type: f.type, attr: f.attr || '', transform: f.transform || '',
        })),
      },
    })
    showSuccess('规则已保存')
  } catch (e) {
    showError('保存规则失败: ' + (e.response?.data?.message || e.message))
  }
}
</script>

<style scoped>
.page-header { margin-bottom: 1.5rem; }
.page-header h1 { font-size: 1.5rem; font-weight: 700; color: var(--text); margin-top: 0.5rem; }

.back-btn {
  display: inline-flex; align-items: center; gap: 0.3rem;
  padding: 0.35rem 0.85rem; border-radius: var(--radius-pill);
  font-size: 0.8125rem; font-weight: 700; color: var(--text-secondary);
  background: var(--bg-elevated); text-decoration: none;
  transition: var(--transition); margin-bottom: 0.5rem;
}
.back-btn:hover { background: var(--bg-hover); color: var(--text); }
</style>
