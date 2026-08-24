<template>
  <div class="identity-editor">
    <div class="form-group">
      <label>商品身份模式</label>
      <div class="filter-mode-row">
        <label class="radio-label" :class="{ active: modelValue.mode === 'source_url', disabled: !allowSourceUrl }">
          <input type="radio" :disabled="!allowSourceUrl" :checked="modelValue.mode === 'source_url'" @change="updateMode('source_url')" />
          使用页面 URL
        </label>
        <label class="radio-label" :class="{ active: modelValue.mode === 'field' }">
          <input type="radio" :checked="modelValue.mode === 'field'" @change="updateMode('field')" />
          使用字段值
        </label>
      </div>
    </div>

    <div class="form-group" v-if="modelValue.mode === 'field'">
      <label>身份字段</label>
      <select :value="modelValue.field" @change="update('field', $event.target.value)" class="form-input">
        <option value="">选择身份字段</option>
        <option v-for="f in fields" :key="f.name" :value="f.name">{{ f.name }}</option>
      </select>
      <p class="hint">该字段必须能唯一标识每件商品，且在两次检查之间保持不变。不要使用列表位置。</p>
    </div>

    <div class="identity-hint" v-if="modelValue.mode === 'source_url'">
      <p>单商品页面默认使用当前 URL 作为商品身份，无需额外配置。</p>
    </div>
  </div>
</template>

<script setup>
const props = defineProps({
  modelValue: { type: Object, required: true },
  fields: { type: Array, default: () => [] },
  allowSourceUrl: { type: Boolean, default: true },
})
const emit = defineEmits(['update:modelValue'])

function update(key, value) {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
}

function updateMode(mode) {
  if (mode === 'source_url' && !props.allowSourceUrl) return
  emit('update:modelValue', { mode, field: mode === 'source_url' ? '' : props.modelValue.field })
}
</script>

<style scoped>
.identity-editor { margin-bottom: 0.5rem; }
.identity-hint {
  padding: 0.5rem 0.75rem;
  background: var(--bg-elevated);
  border-radius: var(--radius-lg);
  font-size: 0.8125rem;
  color: var(--text-secondary);
}
.filter-mode-row {
  display: flex; gap: 0.5rem; margin-top: 0.25rem;
}
.radio-label {
  display: flex; align-items: center; gap: 0.4rem;
  padding: 0.45rem 0.85rem; border-radius: var(--radius-pill);
  font-size: 0.8125rem; font-weight: 700; cursor: pointer;
  background: var(--bg-surface); color: var(--text-secondary);
  transition: var(--transition);
}
.radio-label:hover { background: var(--bg-elevated); color: var(--text); }
.radio-label.active { background: var(--green); color: #000; }
.radio-label.disabled { cursor: not-allowed; opacity: 0.45; }
.radio-label input { display: none; }
.hint { font-size: 0.75rem; color: var(--text-muted); margin-top: 0.25rem; }
</style>
