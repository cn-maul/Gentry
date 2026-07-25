<template>
  <div class="settings-section">
    <div class="section-header"><h2>{{ monitorType === 'field_transition' ? '商品信息' : '网页信息' }}</h2></div>
    <div class="form-group">
      <label>名称</label>
      <input :value="modelValue.name" @input="update('name', $event.target.value)" class="form-input" :placeholder="monitorType === 'field_transition' ? '如 AdGuard 套餐价格' : '如 招录公告'" />
    </div>
    <div class="form-group">
      <label>{{ monitorType === 'field_transition' ? '商品页面 URL' : '网页 URL' }}</label>
      <input :value="modelValue.url" @input="update('url', $event.target.value)" class="form-input" :placeholder="monitorType === 'field_transition' ? 'https://shop.example/products/item' : 'https://example.com/announce/'" />
    </div>
    <div class="form-group">
      <label>分组</label>
      <input :value="modelValue.group" @input="update('group', $event.target.value)" class="form-input" placeholder="默认" />
    </div>
    <div class="form-group">
      <label>检查间隔（秒）</label>
      <input :value="modelValue.interval" @input="update('interval', Number($event.target.value))" class="form-input" type="number" min="10" placeholder="3600（默认1小时）" />
    </div>
  </div>
</template>

<script setup>
const props = defineProps({
  modelValue: { type: Object, required: true },
  monitorType: { type: String, default: 'presence' },
})
const emit = defineEmits(['update:modelValue'])

function update(key, value) {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
}
</script>
