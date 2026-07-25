<template>
  <div class="field-editor">
    <div class="fields-header">
      <span class="fields-title">提取字段</span>
      <button class="btn btn-sm btn-ghost" @click="addField">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/></svg>
        添加字段
      </button>
    </div>

    <div class="fields-list">
      <div v-for="(field, index) in modelValue" :key="index" class="field-card">
        <div class="field-grid">
          <div class="form-group">
            <label>名称</label>
            <input v-model="field.name" class="form-input" placeholder="如 title" />
          </div>
          <div class="form-group">
            <label>选择器</label>
            <input v-model="field.selector" class="form-input" placeholder="如 a.title" />
          </div>
          <div class="form-group">
            <label>类型</label>
            <select v-model="field.type" class="form-input">
              <option value="text">文本 (text)</option>
              <option value="attr">属性 (attr)</option>
            </select>
          </div>
          <div class="form-group" v-if="field.type === 'attr'">
            <label>属性名</label>
            <input v-model="field.attr" class="form-input" placeholder="默认 href" />
          </div>
          <div class="form-group">
            <label>转换</label>
            <input v-model="field.transform" class="form-input" placeholder="如 trim([])" />
          </div>
        </div>
        <button class="icon-btn icon-btn-danger" title="删除字段" @click="removeField(index)">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
        </button>
      </div>
    </div>

    <div class="empty" v-if="modelValue.length === 0">
      <p>暂无字段，点击上方按钮添加</p>
    </div>
  </div>
</template>

<script setup>
const props = defineProps({
  modelValue: { type: Array, default: () => [] },
})
const emit = defineEmits(['update:modelValue'])

function addField() {
  emit('update:modelValue', [
    ...props.modelValue,
    { name: '', selector: '', type: 'text', attr: '', transform: '' },
  ])
}

function removeField(index) {
  const copy = [...props.modelValue]
  copy.splice(index, 1)
  emit('update:modelValue', copy)
}
</script>

<style scoped>
.field-editor {
  margin-bottom: 1rem;
}

.fields-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.75rem;
}

.fields-title {
  font-size: 0.75rem;
  font-weight: 700;
  color: var(--text-secondary);
}

.fields-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.field-card {
  display: flex;
  gap: 0.5rem;
  align-items: flex-start;
  padding: 0.75rem;
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
}

.field-grid {
  flex: 1;
  display: grid;
  grid-template-columns: 1fr 1fr 1fr 1fr 1fr;
  gap: 0.5rem;
}

.icon-btn {
  background: none;
  border: none;
  cursor: pointer;
  padding: 0.4rem;
  border-radius: var(--radius-circle);
  transition: var(--transition);
  color: var(--text-muted);
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
  margin-top: 1.5rem;
}

.icon-btn:hover { background: var(--bg-hover); }

.icon-btn-danger:hover {
  background: var(--error-bg);
  color: var(--error);
}

@media (max-width: 768px) {
  .field-grid { grid-template-columns: 1fr 1fr; }
}
</style>