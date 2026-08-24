<template>
  <div class="threshold-editor">
    <div class="threshold-options">
      <label class="radio-label block-radio" :class="{ active: mode === 'any' }">
        <input type="radio" value="any" v-model="mode" />
        任意降价即通知
      </label>
      <label class="radio-label block-radio" :class="{ active: mode === 'amount' }">
        <input type="radio" value="amount" v-model="mode" />
        降价金额至少达到
      </label>
      <label class="radio-label block-radio" :class="{ active: mode === 'percent' }">
        <input type="radio" value="percent" v-model="mode" />
        降价百分比至少达到
      </label>
      <label class="radio-label block-radio" :class="{ active: mode === 'both' }">
        <input type="radio" value="both" v-model="mode" />
        同时满足金额和百分比
      </label>
    </div>

    <div class="threshold-inputs" v-if="mode === 'amount' || mode === 'both'">
      <div class="form-group">
        <label>最低降价金额</label>
        <div class="input-with-suffix">
          <input :value="modelValue.minAmount" @input="emit('update:modelValue', { ...modelValue, minAmount: $event.target.value })" class="form-input" type="number" min="0" step="0.01" placeholder="0" />
          <span class="input-suffix">元</span>
        </div>
      </div>
    </div>

    <div class="threshold-inputs" v-if="mode === 'percent' || mode === 'both'">
      <div class="form-group">
        <label>最低降价百分比</label>
        <div class="input-with-suffix">
          <input :value="modelValue.minPercent" @input="emit('update:modelValue', { ...modelValue, minPercent: $event.target.value })" class="form-input" type="number" min="0" max="100" step="0.1" placeholder="0" />
          <span class="input-suffix">%</span>
        </div>
      </div>
    </div>

    <p class="threshold-hint" v-if="mode === 'both'">
      仅当降价金额和降价百分比同时满足时才会推送通知
    </p>
    <p class="threshold-hint" v-if="mode === 'any'">
      任何降价都会触发通知，不设最低门槛
    </p>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'

const props = defineProps({
  modelValue: { type: Object, required: true },
})
const emit = defineEmits(['update:modelValue'])

const mode = ref(resolveMode(props.modelValue))

function resolveMode(val) {
  const hasAmt = val.minAmount !== '' && val.minAmount !== null && Number(val.minAmount) > 0
  const hasPct = val.minPercent !== '' && val.minPercent !== null && Number(val.minPercent) > 0
  if (hasAmt && hasPct) return 'both'
  if (hasAmt) return 'amount'
  if (hasPct) return 'percent'
  return 'any'
}

watch(mode, (m) => {
  if (m === 'any') {
    emit('update:modelValue', { ...props.modelValue, minAmount: '', minPercent: '' })
  } else if (m === 'amount') {
    emit('update:modelValue', { ...props.modelValue, minAmount: props.modelValue.minAmount || '', minPercent: '' })
  } else if (m === 'percent') {
    emit('update:modelValue', { ...props.modelValue, minAmount: '', minPercent: props.modelValue.minPercent || '' })
  }
})
</script>

<style scoped>
.threshold-options {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  margin-bottom: 0.75rem;
}
.block-radio {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.55rem 0.85rem;
  border-radius: var(--radius-pill);
  font-size: 0.8125rem;
  font-weight: 700;
  cursor: pointer;
  background: var(--bg-surface);
  color: var(--text-secondary);
  transition: var(--transition);
  width: fit-content;
}
.block-radio:hover { background: var(--bg-elevated); color: var(--text); }
.block-radio.active { background: var(--green); color: #000; }
.block-radio input { display: none; }
.threshold-inputs { margin-top: 0.5rem; }
.input-with-suffix {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}
.input-with-suffix .form-input { flex: 1; max-width: 200px; }
.input-suffix {
  font-size: 0.8125rem;
  font-weight: 700;
  color: var(--text-muted);
}
.threshold-hint {
  font-size: 0.75rem;
  color: var(--text-muted);
  margin-top: 0.25rem;
  padding: 0.4rem 0.6rem;
  background: var(--bg-elevated);
  border-radius: var(--radius-lg);
}
</style>
