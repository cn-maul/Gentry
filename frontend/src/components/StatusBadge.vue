<template>
  <span class="status-badge" :class="`badge-${type}`">
    <span class="badge-dot" />
    {{ label }}
  </span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: { type: String, default: 'unknown' },
})

const type = computed(() => {
  const s = props.status
  if (s === 'running') return 'success'
  if (s === 'stopped') return 'default'
  if (s === 'error') return 'error'
  return 'default'
})

const label = computed(() => {
  const s = props.status
  if (s === 'running') return '运行中'
  if (s === 'stopped') return '已停止'
  if (s === 'error') return '异常'
  return '未知'
})
</script>

<style scoped>
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.15rem 0.55rem;
  border-radius: var(--radius-pill);
  font-size: 0.6875rem;
  font-weight: 600;
  text-transform: capitalize;
  letter-spacing: 0.3px;
}

.badge-dot {
  width: 6px;
  height: 6px;
  border-radius: var(--radius-circle);
}

.badge-success {
  background: var(--success-bg);
  color: var(--success);
}
.badge-success .badge-dot { background: var(--success); }

.badge-error {
  background: var(--error-bg);
  color: var(--error);
}
.badge-error .badge-dot { background: var(--error); }

.badge-default {
  background: var(--bg-elevated);
  color: var(--text-muted);
}
.badge-default .badge-dot { background: var(--text-muted); }
</style>