<template>
  <div class="monitor-form">
    <MonitorTypeSelector :modelValue="form.monitorType" @update:modelValue="$emit('change:type', $event)" v-if="showTypeSelector" />

    <BasicMonitorForm v-model="form.basic" :monitorType="form.monitorType" />

    <template v-if="form.monitorType === 'presence'">
      <ExtractionEditor v-model="form.extraction" :url="form.basic.url" />
    </template>

    <template v-else>
      <PriceExtractionEditor :form="form" @update:form="updateForm" />
      <NumericTransitionRuleEditor :form="form" @update:form="updateForm" />
    </template>

    <MonitorValidationPanel :result="validationResult" :loading="validationLoading" />

    <details class="advanced-settings">
      <summary>
        <span>
          <strong>高级设置</strong>
          <small>通知、检查频率和提取规则</small>
        </span>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><polyline points="9 18 15 12 9 6"/></svg>
      </summary>

      <div class="advanced-content">
        <BasicMonitorForm v-model="form.basic" :monitorType="form.monitorType" advanced />

        <ExtractionEditor
          v-if="form.monitorType === 'presence'"
          v-model="form.extraction"
          :url="form.basic.url"
          advanced
        />
        <PriceExtractionEditor v-else :form="form" advanced @update:form="updateForm" />
        <NumericTransitionRuleEditor
          v-if="form.monitorType === 'field_transition' && form.rule.transition.operator === 'decreased'"
          :form="form"
          advanced
          @update:form="updateForm"
        />

        <NotificationEditor
          v-model="form.notification"
          :accounts="accounts"
          :monitorType="form.monitorType"
        />

        <div class="validation-actions">
          <div>
            <strong>检查当前配置</strong>
            <p>抓取一次网页，确认系统能够读取到所需内容。</p>
          </div>
          <button class="btn btn-ghost" type="button" :disabled="validationLoading" @click="$emit('validate')">
            {{ validationLoading ? '检查中...' : '运行检查' }}
          </button>
        </div>

        <MonitorFormSummary :form="form" />
        <div class="advanced-actions"><slot name="advanced-actions" /></div>
      </div>
    </details>

    <div class="start-option">
      <label class="checkbox-label">
        <input type="checkbox" :checked="form.basic.isActive" @change="form.basic.isActive = $event.target.checked" />
        创建后立即开始监控
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

.advanced-settings {
  margin-bottom: 1rem;
  border-top: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
}
.advanced-settings > summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.9rem 0.25rem;
  color: var(--text-secondary);
  cursor: pointer;
  list-style: none;
}
.advanced-settings > summary::-webkit-details-marker { display: none; }
.advanced-settings > summary span { display: grid; gap: 0.1rem; }
.advanced-settings > summary strong { color: var(--text); font-size: 0.875rem; }
.advanced-settings > summary small { color: var(--text-muted); font-size: 0.75rem; }
.advanced-settings > summary svg { width: 16px; height: 16px; flex: 0 0 16px; transition: transform 160ms ease; }
.advanced-settings[open] > summary svg { transform: rotate(90deg); }
.advanced-content { padding: 0.5rem 0 0.25rem; }
.advanced-content :deep(.advanced-panel-section) { padding: 1rem 0; border-bottom: 1px solid var(--border-light); }
.advanced-content :deep(.settings-section) { margin: 0; padding: 1rem 0; border-bottom: 1px solid var(--border-light); border-radius: 0; background: transparent; }
.advanced-actions { display: flex; justify-content: flex-start; }

.start-option { margin: 0.25rem 0 1rem; }
.start-option .checkbox-label { display: inline-flex; align-items: center; gap: 0.5rem; color: var(--text-secondary); font-size: 0.8125rem; cursor: pointer; }
.start-option input { accent-color: var(--green); }

.validation-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.75rem 1rem;
  margin-bottom: 1rem;
  background: transparent;
  border-radius: 0;
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

@media (max-width: 640px) {
  .validation-actions { align-items: stretch; flex-direction: column; }
  .validation-actions .btn { width: 100%; }
  .form-actions { align-items: stretch; flex-direction: column-reverse; }
  .form-actions :deep(.btn) { width: 100%; justify-content: center; }
}
</style>
