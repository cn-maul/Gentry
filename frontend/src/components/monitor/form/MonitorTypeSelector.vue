<template>
  <div class="type-selector" role="tablist" aria-label="监控类型">
    <button
      type="button"
      role="tab"
      :aria-selected="modelValue === 'presence'"
      class="type-tab"
      :class="{ selected: modelValue === 'presence' }"
      @click="$emit('update:modelValue', 'presence')"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 9h18"/><path d="M8 14h8"/></svg>
      <span><strong>网页监控</strong><small>检测公告、新闻等新增内容</small></span>
    </button>

    <button
      type="button"
      role="tab"
      :aria-selected="modelValue === 'field_transition'"
      class="type-tab"
      :class="{ selected: modelValue === 'field_transition' }"
      @click="$emit('update:modelValue', 'field_transition')"
    >
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M3 17l5-5 4 4 8-9"/><path d="M15 7h5v5"/></svg>
      <span><strong>商品价格监控</strong><small>检测降价或到达目标价格</small></span>
    </button>
  </div>
</template>

<script setup>
defineProps({
  modelValue: { type: String, default: 'presence' },
})
defineEmits(['update:modelValue'])
</script>

<style scoped>
.type-selector {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 6px;
  margin-bottom: 1.5rem;
}

.type-tab {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  min-height: 64px;
  padding: 0.75rem 1rem;
  border: 0;
  border-right: 1px solid var(--border);
  background: var(--bg-elevated);
  color: var(--text-secondary);
  cursor: pointer;
  transition: var(--transition);
  text-align: left;
}
.type-tab:last-child { border-right: 0; }
.type-tab:hover {
  background: var(--bg-hover);
  color: var(--text);
}
.type-tab.selected { background: var(--green); color: #000; }
.type-tab svg { width: 21px; height: 21px; flex: 0 0 21px; }
.type-tab span { display: grid; gap: 0.1rem; min-width: 0; }
.type-tab strong { font-size: 0.875rem; }
.type-tab small { overflow: hidden; font-size: 0.6875rem; text-overflow: ellipsis; white-space: nowrap; }

@media (max-width: 600px) {
  .type-selector {
    grid-template-columns: 1fr;
  }
  .type-tab { border-right: 0; border-bottom: 1px solid var(--border); }
  .type-tab:last-child { border-bottom: 0; }
}
</style>
