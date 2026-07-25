<template>
  <div class="monitor-form">
    <MonitorTypeSelector :modelValue="form.monitorType" @update:modelValue="$emit('change:type', $event)" v-if="showTypeSelector" />

    <BasicMonitorForm v-model="form.basic" :monitorType="form.monitorType" />

    <template v-if="form.monitorType === 'presence'">
      <ExtractionEditor v-model="form.extraction" :url="form.basic.url" />
      <PresenceRuleEditor />
    </template>

    <template v-else>
      <PriceExtractionEditor :form="form" @update:form="updateForm" />
      <NumericTransitionRuleEditor :form="form" @update:form="updateForm" />
    </template>

    <NotificationEditor
      v-model="form.notification"
      :accounts="accounts"
      :monitorType="form.monitorType"
    />

    <div class="validation-actions">
      <div>
        <strong>{{ form.monitorType === 'field_transition' ? '验证商品与价格' : '验证网页提取' }}</strong>
        <p>{{ form.monitorType === 'field_transition' ? '检查商品身份、价格解析和重复 SKU。' : '检查页面区域和内容字段。' }}</p>
      </div>
      <button class="btn btn-ghost" type="button" :disabled="validationLoading" @click="$emit('validate')">
        {{ validationLoading ? '验证中...' : (form.monitorType === 'field_transition' ? '验证价格' : '验证网页') }}
      </button>
    </div>

    <MonitorValidationPanel :result="validationResult" :loading="validationLoading" />

    <MonitorFormSummary :form="form" />

    <div class="form-group">
      <label class="checkbox-label">
        <input type="checkbox" :checked="form.basic.isActive" @change="form.basic.isActive = $event.target.checked" />
        保存后立即启动监控
      </label>
    </div>

    <div class="form-error" v-if="error">{{ error }}</div>

    <div class="baseline-warning" v-if="showBaselineWarning">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
      <span>此修改会清除当前比较基线。保存后首次检查只建立新基线，不会发送降价通知。</span>
    </div>

    <div class="form-actions">
      <router-link to="/" class="btn btn-ghost">取消</router-link>
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup>
import MonitorTypeSelector from './MonitorTypeSelector.vue'
import BasicMonitorForm from './BasicMonitorForm.vue'
import ExtractionEditor from './ExtractionEditor.vue'
import PriceExtractionEditor from './PriceExtractionEditor.vue'
import NumericTransitionRuleEditor from './NumericTransitionRuleEditor.vue'
import PresenceRuleEditor from './PresenceRuleEditor.vue'
import NotificationEditor from './NotificationEditor.vue'
import MonitorValidationPanel from './MonitorValidationPanel.vue'
import MonitorFormSummary from './MonitorFormSummary.vue'

const props = defineProps({
  form: { type: Object, required: true },
  showTypeSelector: { type: Boolean, default: true },
  accounts: { type: Array, default: () => [] },
  error: { type: String, default: null },
  validationResult: { type: Object, default: null },
  validationLoading: { type: Boolean, default: false },
  showBaselineWarning: { type: Boolean, default: false },
})

const emit = defineEmits(['update:form', 'validate', 'change:type'])

function updateForm(newForm) {
  emit('update:form', newForm)
}
</script>

<style scoped>
.monitor-form {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.validation-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.75rem 1rem;
  margin-bottom: 1rem;
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
}
.validation-actions strong { display: block; color: var(--text); font-size: 0.875rem; }
.validation-actions p { color: var(--text-muted); font-size: 0.75rem; margin-top: 0.15rem; }

.form-error {
  background: var(--error-bg);
  color: var(--error);
  padding: 0.5rem 0.75rem;
  border-radius: var(--radius-lg);
  font-size: 0.8125rem;
  margin-bottom: 1rem;
}

.baseline-warning {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  padding: 0.6rem 0.75rem;
  background: var(--warning-bg);
  color: var(--warning);
  border-radius: var(--radius-lg);
  font-size: 0.8125rem;
  margin-bottom: 1rem;
  line-height: 1.4;
}
.baseline-warning svg { flex-shrink: 0; margin-top: 1px; }

.form-actions {
  display: flex;
  gap: 0.5rem;
  justify-content: flex-end;
  align-items: center;
  margin-top: 0.5rem;
}
</style>
