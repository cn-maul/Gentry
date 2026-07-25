<template>
  <div class="settings-section">
    <div class="section-header">
      <h2>价格监控规则</h2>
      <p class="section-desc">设置满足什么价格条件时产生通知</p>
    </div>

    <div class="subsection">
      <h3 class="subsection-title">触发条件</h3>
      <div class="filter-mode-row rule-mode-row">
        <label class="radio-label" :class="{ active: form.rule.transition.operator === 'decreased' }">
          <input type="radio" :checked="form.rule.transition.operator === 'decreased'" @change="updateOperator('decreased')" />
          价格发生下降
        </label>
        <label class="radio-label" :class="{ active: form.rule.transition.operator === 'at_or_below' }">
          <input type="radio" :checked="form.rule.transition.operator === 'at_or_below'" @change="updateOperator('at_or_below')" />
          降到目标价及以下
        </label>
      </div>
      <ThresholdEditor
        v-if="form.rule.transition.operator === 'decreased'"
        :modelValue="form.rule.transition"
        @update:modelValue="updateTransition"
      />
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
          <p class="hint">仅在价格从目标价以上降到目标价或以下时通知；持续低于目标价不会重复推送。</p>
        </div>
      </div>
    </div>

    <div class="baseline-notice">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>
      <span>首次检查仅建立价格基线，不会发送通知。后续检测到符合价格规则的变化后才会推送。</span>
    </div>
  </div>
</template>

<script setup>
import ThresholdEditor from './ThresholdEditor.vue'

const props = defineProps({
  form: { type: Object, required: true },
})
const emit = defineEmits(['update:form'])

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
