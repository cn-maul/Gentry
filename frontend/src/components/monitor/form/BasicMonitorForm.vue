<template>
  <div :class="advanced ? 'advanced-panel-section' : 'settings-section basic-monitor-form'">
    <template v-if="!advanced">
      <div class="section-header">
        <h2>{{ monitorType === 'field_transition' ? '要监控哪个商品？' : '要监控哪个网页？' }}</h2>
        <p class="section-desc">粘贴网址即可，名称可以留空。</p>
      </div>
      <div class="form-group">
        <label>{{ monitorType === 'field_transition' ? '商品网址' : '网页网址' }}</label>
        <input :value="modelValue.url" @input="update('url', $event.target.value)" class="form-input primary-url" inputmode="url" autofocus :placeholder="monitorType === 'field_transition' ? 'https://shop.example.com/product' : 'https://example.com/notice'" />
      </div>
      <div class="form-group optional-name">
        <label>监控名称 <span>可选</span></label>
        <input :value="modelValue.name" @input="update('name', $event.target.value)" class="form-input" :placeholder="monitorType === 'field_transition' ? '留空将自动使用网站名称' : '留空将自动使用网站名称'" />
      </div>
    </template>
    <template v-else>
      <h3>检查频率与分组</h3>
      <p class="advanced-section-desc">默认每小时检查一次，通常无需修改。</p>
    <div class="form-group">
      <label>分组</label>
      <input :value="modelValue.group" @input="update('group', $event.target.value)" class="form-input" placeholder="默认" />
    </div>
    <div class="form-group">
      <label>检查间隔（秒）</label>
      <input :value="modelValue.interval" @input="update('interval', Number($event.target.value))" class="form-input" type="number" min="10" placeholder="3600（默认1小时）" />
    </div>
    </template>
  </div>
</template>

<script setup>
const props = defineProps({
  modelValue: { type: Object, required: true },
  monitorType: { type: String, default: 'presence' },
  advanced: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue'])

function update(key, value) {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
}
</script>

<style scoped>
.basic-monitor-form .section-header { margin-bottom: 1rem; }
.basic-monitor-form .primary-url { min-height: 46px; font-size: 0.9375rem; }
.optional-name { margin-bottom: 0; }
.optional-name label span { color: var(--text-muted); font-size: 0.6875rem; font-weight: 500; text-transform: none; }
.advanced-panel-section h3 { margin-bottom: 0.2rem; color: var(--text); font-size: 0.875rem; }
.advanced-section-desc { margin-bottom: 1rem; color: var(--text-muted); font-size: 0.75rem; }
</style>
