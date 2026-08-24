<template>
  <div class="settings-section">
    <div class="section-header"><h2>通知配置</h2></div>

    <div class="form-group">
      <label>推送过滤</label>
      <div class="filter-mode-row">
        <label class="radio-label" :class="{ active: modelValue.filter === 'all' }">
          <input type="radio" :checked="modelValue.filter === 'all'" @change="update('filter', 'all')" />
          {{ monitorType === 'field_transition' ? '符合价格规则就推送' : '有新内容就推送' }}
        </label>
        <label class="radio-label" :class="{ active: modelValue.filter === 'keyword' }">
          <input type="radio" :checked="modelValue.filter === 'keyword'" @change="update('filter', 'keyword')" />
          仅命中关键词时推送
        </label>
      </div>
      <div class="form-group" v-if="modelValue.filter === 'keyword'" style="margin-top: 0.5rem;">
        <label>推送关键词（多个用逗号隔开）</label>
        <input :value="modelValue.keywords" @input="update('keywords', $event.target.value)" class="form-input" placeholder="面试,录用,公示" />
      </div>
    </div>

    <div class="form-group">
      <label>推送账户</label>
      <div class="accounts-grid" v-if="accounts.length > 0">
        <label v-for="acc in accounts" :key="acc.id" class="account-checkbox">
          <input
            type="checkbox"
            :value="acc.id"
            :checked="modelValue.accountIds.includes(acc.id)"
            @change="toggleAccount(acc.id)"
          />
          <span class="acc-name">{{ acc.name }}</span>
          <span class="acc-badge" :class="'badge-' + acc.service">{{ serviceLabel(acc.service) }}</span>
        </label>
      </div>
      <p class="hint" v-else>暂无推送账户，请先在「推送管理」中添加</p>
    </div>
  </div>
</template>

<script setup>
const props = defineProps({
  modelValue: { type: Object, required: true },
  accounts: { type: Array, default: () => [] },
  monitorType: { type: String, default: 'presence' },
})
const emit = defineEmits(['update:modelValue'])

function update(key, value) {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
}

function toggleAccount(id) {
  const ids = [...props.modelValue.accountIds]
  const idx = ids.indexOf(id)
  if (idx >= 0) {
    ids.splice(idx, 1)
  } else {
    ids.push(id)
  }
  emit('update:modelValue', { ...props.modelValue, accountIds: ids })
}

function serviceLabel(s) {
  if (s === 'pushplus') return 'PushPlus'
  if (s === 'webhook') return 'Webhook'
  if (s === 'serverchan') return 'Server酱'
  if (s === 'bark') return 'Bark'
  return s
}
</script>

<style scoped>
.section-header { margin-bottom: 1.25rem; padding-bottom: 0.75rem; border-bottom: 1px solid var(--border-light); }
.section-header h2 { font-size: 1.125rem; font-weight: 700; color: var(--text); margin-bottom: 0.15rem; }

.filter-mode-row { display: flex; gap: 0.5rem; margin-top: 0.25rem; flex-wrap: wrap; }
.radio-label {
  display: flex; align-items: center; gap: 0.4rem;
  padding: 0.45rem 0.85rem; border-radius: var(--radius-pill);
  font-size: 0.8125rem; font-weight: 700; cursor: pointer;
  background: var(--bg-surface); color: var(--text-secondary);
  transition: var(--transition);
}
.radio-label:hover { background: var(--bg-elevated); color: var(--text); }
.radio-label.active { background: var(--green); color: #000; }
.radio-label input { display: none; }

.accounts-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.4rem;
}
.account-checkbox {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  border-radius: var(--radius-lg);
  background: var(--bg-surface);
  cursor: pointer;
  transition: var(--transition);
}
.account-checkbox:hover { background: var(--bg-elevated); }
.account-checkbox input { accent-color: var(--green); }
.acc-name { font-size: 0.8125rem; font-weight: 700; color: var(--text); }
.acc-badge {
  font-size: 0.5625rem; font-weight: 700; color: #000;
  background: var(--green); padding: 0.08rem 0.35rem; border-radius: var(--radius-pill);
}
.badge-bark { background: #d32f2f; color: #fff; }
.badge-pushplus { background: #1976d2; color: #fff; }
.badge-serverchan { background: var(--green); color: #000; }
.hint { font-size: 0.75rem; color: var(--text-muted); margin-top: 0.25rem; }
</style>
