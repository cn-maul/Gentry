<template>
  <div :class="advanced ? 'advanced-panel-section' : 'settings-section'">
    <template v-if="!advanced">
    <div class="section-header">
      <h2>什么时候提醒？</h2>
    </div>

    <div class="subsection">
      <div class="filter-mode-row rule-mode-row">
        <label class="radio-label" :class="{ active: form.rule.transition.operator === 'decreased' }">
          <input type="radio" :checked="form.rule.transition.operator === 'decreased'" @change="updateOperator('decreased')" />
          有降价就提醒
        </label>
        <label class="radio-label" :class="{ active: form.rule.transition.operator === 'at_or_below' }">
          <input type="radio" :checked="form.rule.transition.operator === 'at_or_below'" @change="updateOperator('at_or_below')" />
          降到目标价提醒
        </label>
      </div>
      <p v-if="form.rule.transition.operator === 'decreased'" class="simple-rule-note">
        {{ hasThreshold ? '已设置最低降价门槛，可在高级设置中调整。' : '价格低于上一次记录时通知。' }}
      </p>
      <div class="target-price-editor" v-else>
        <div class="form-group">
          <label>目标价格</label>
          <div class="input-with-suffix">
            <input
              :value="form.rule.transition.targetPrice"
              @input="updateTargetPrice($event.target.value)"
              class="form-input"
              type="number"
              min="0"
              step="0.01"
              placeholder="例如 199.00"
            />
            <span class="input-suffix">元</span>
          </div>
          <p class="hint">价格首次降到目标价或以下时通知。</p>
        </div>
      </div>
    </div>

    <div class="baseline-notice">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>
      <span>首次检查仅建立价格基线，不会发送通知。后续检测到符合价格规则的变化后才会推送。</span>
    </div>
    </template>

    <template v-else-if="form.rule.transition.operator === 'decreased'">
      <h3>最低降价门槛</h3>
      <p class="advanced-section-desc">默认任何降价都会通知；需要减少小额波动提醒时再设置。</p>
      <ThresholdEditor :modelValue="form.rule.transition" @update:modelValue="updateTransition" />
    </template>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import ThresholdEditor from './ThresholdEditor.vue'

const props = defineProps({
  form: { type: Object, required: true },
  advanced: { type: Boolean, default: false },
})
const emit = defineEmits(['update:form'])
const hasThreshold = computed(() => Number(props.form.rule.transition.minAmount) > 0 || Number(props.form.rule.transition.minPercent) > 0)

function updateTransition(val) {
  emit('update:form', {
    ...props.form,
    rule: { ...props.form.rule, transition: val },
  })
}

function updateOperator(operator) {
  updateTransition({ ...props.form.rule.transition, operator })
}

function updateTargetPrice(targetPrice) {
  updateTransition({ ...props.form.rule.transition, targetPrice })
}
</script>

<style scoped>
.section-header { margin-bottom: 1.25rem; padding-bottom: 0.75rem; border-bottom: 1px solid var(--border-light); }
.section-header h2 { font-size: 1.125rem; font-weight: 700; color: var(--text); margin-bottom: 0.15rem; }
.section-desc { font-size: 0.8125rem; color: var(--text-secondary); }
.advanced-panel-section h3 { margin-bottom: 0.2rem; color: var(--text); font-size: 0.875rem; }
.advanced-section-desc { margin-bottom: 1rem; color: var(--text-muted); font-size: 0.75rem; }

.subsection {
  padding: 1rem;
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
  margin-bottom: 1rem;
}
.subsection-title {
  font-size: 0.875rem;
  font-weight: 700;
  color: var(--text);
  margin-bottom: 0.25rem;
}
.subsection-desc {
  font-size: 0.75rem;
  color: var(--text-secondary);
  margin-bottom: 0.75rem;
}
.form-row { display: flex; gap: 1rem; }
.form-row .form-group { flex: 1; }
.type-hint { font-size: 0.6875rem; color: var(--warning); margin-top: 0.2rem; }
.hint { font-size: 0.75rem; color: var(--text-muted); margin-top: 0.2rem; }
.filter-mode-row { display: flex; gap: 0.5rem; flex-wrap: wrap; }
.rule-mode-row { margin-bottom: 0.75rem; }
.simple-rule-note { color: var(--text-secondary); font-size: 0.8125rem; }
.radio-label {
  display: flex; align-items: center; gap: 0.4rem;
  padding: 0.45rem 0.85rem; border-radius: var(--radius-pill);
  font-size: 0.8125rem; font-weight: 700; cursor: pointer;
  background: var(--bg-elevated); color: var(--text-secondary);
}
.radio-label.active { background: var(--green); color: #000; }
.radio-label input { display: none; }
.target-price-editor { margin-top: 0.5rem; }
.input-with-suffix { display: flex; align-items: center; gap: 0.5rem; }
.input-with-suffix .form-input { max-width: 220px; }
.input-suffix { color: var(--text-muted); font-size: 0.8125rem; font-weight: 700; }

.baseline-notice {
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
.baseline-notice svg { flex-shrink: 0; margin-top: 1px; color: var(--text-muted); }
</style>
