<template>
  <div class="monitor-card" :class="{ 'card-error': isError, 'card-stopped': !isRunning }" @click="$emit('view')">
    <div class="card-left">
      <StatusBadge :status="statusText" />
      <span class="card-name">{{ monitor.name }}</span>
      <span class="type-tag" v-if="monitor.strategy_type" :class="'tag-' + monitor.strategy_type">
        {{ monitor.strategy_type === 'field_transition' ? '价格监控' : '新增检测' }}
      </span>
    </div>

    <div class="card-url-wrap">
      <span class="card-url" :title="monitor.url">{{ monitor.url }}</span>
    </div>

    <div class="card-meta">
      <span class="meta-baseline" v-if="monitor.strategy_type === 'field_transition' && monitor.baseline_status" :class="monitor.baseline_status === 'ready' ? 'baseline-ok' : 'baseline-pending'">
        {{ monitor.baseline_status === 'ready' ? '基线就绪' : '待建立' }}
      </span>
      <span class="meta-time" v-if="monitor.last_check">
        {{ formatTime(monitor.last_check) }}
      </span>
      <span class="meta-updates" v-if="monitor.updates_count > 0">
        {{ monitor.updates_count }}
      </span>
      <span class="meta-error-badge" v-if="monitor.last_error" :title="monitor.last_error">
        错误
      </span>
    </div>

    <div class="card-actions" @click.stop>
      <button class="circle-btn" :class="isRunning ? 'btn-pause' : 'btn-play'" :disabled="pending" @click="$emit(isRunning ? 'stop' : 'start')" :title="isRunning ? '暂停' : '启动'">
        <svg v-if="isRunning" viewBox="0 0 24 24" fill="currentColor" width="16" height="16">
          <rect x="6" y="4" width="4" height="16" rx="1"/>
          <rect x="14" y="4" width="4" height="16" rx="1"/>
        </svg>
        <svg v-else viewBox="0 0 24 24" fill="currentColor" width="16" height="16">
          <path d="M8 5v14l11-7z"/>
        </svg>
      </button>
      <button class="icon-btn" title="编辑" :disabled="pending" @click="$emit('edit')">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
      </button>
      <button class="icon-btn icon-btn-danger" title="删除" :disabled="pending" @click="$emit('delete')">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
      </button>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import StatusBadge from './StatusBadge.vue'

const props = defineProps({
  monitor: { type: Object, required: true },
  pending: { type: Boolean, default: false },
})

defineEmits(['start', 'stop', 'edit', 'delete', 'view'])

const isRunning = computed(() => props.monitor.is_running)
const isError = computed(() => !!props.monitor.last_error)
const statusText = computed(() => {
  if (isError.value) return 'error'
  return isRunning.value ? 'running' : 'stopped'
})

function formatTime(t) {
  if (!t) return '—'
  const d = new Date(t)
  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  if (sameDay) return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
}
</script>

<style scoped>
.monitor-card {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.75rem 1rem;
  background: var(--bg-card);
  border-radius: var(--radius-lg);
  cursor: pointer;
  transition: var(--transition);
  user-select: none;
}

.monitor-card:hover {
  background: var(--bg-hover);
}

.card-error {
  background: rgba(243, 114, 127, 0.06);
}

.card-error:hover {
  background: rgba(243, 114, 127, 0.1);
}

.card-stopped {
  opacity: 0.55;
}

.card-left {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  min-width: 140px;
  flex-shrink: 0;
}

.type-tag {
  font-size: 0.5625rem;
  font-weight: 700;
  padding: 0.08rem 0.35rem;
  border-radius: var(--radius-pill);
  flex-shrink: 0;
}

.tag-presence {
  background: var(--success-bg);
  color: var(--green);
}

.tag-field_transition {
  background: #e3f2fd;
  color: #1976d2;
}

.dark .tag-field_transition {
  background: #0d2137;
  color: #64b5f6;
}

.card-name {
  font-weight: 700;
  font-size: 0.875rem;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-url-wrap {
  flex: 1;
  min-width: 0;
}

.card-url {
  font-size: 0.75rem;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: block;
}

.card-meta {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}

.meta-time {
  font-size: 0.75rem;
  color: var(--text-muted);
  white-space: nowrap;
}

.meta-baseline {
  font-size: 0.625rem;
  font-weight: 700;
  padding: 0.08rem 0.35rem;
  border-radius: var(--radius-pill);
  white-space: nowrap;
}

.baseline-ok {
  background: var(--success-bg);
  color: var(--green);
}

.baseline-pending {
  background: var(--bg-elevated);
  color: var(--text-muted);
}

.meta-updates {
  font-size: 0.6875rem;
  font-weight: 700;
  color: var(--text);
  background: var(--bg-elevated);
  padding: 0.15rem 0.5rem;
  border-radius: var(--radius-pill);
  white-space: nowrap;
}

.meta-error-badge {
  font-size: 0.6875rem;
  font-weight: 700;
  color: var(--error);
  background: var(--error-bg);
  padding: 0.15rem 0.5rem;
  border-radius: var(--radius-pill);
  white-space: nowrap;
}

.card-actions {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  flex-shrink: 0;
}

.circle-btn:disabled,
.icon-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
  pointer-events: none;
}

/* Circular play/pause button */
.circle-btn {
  width: 36px;
  height: 36px;
  border: none;
  border-radius: var(--radius-circle);
  cursor: pointer;
  transition: var(--transition);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
}

.btn-play {
  background: var(--green);
  color: #000000;
}

.btn-play:hover {
  background: var(--green-hover);
  transform: scale(1.08);
}

.btn-pause {
  background: var(--bg-elevated);
  color: var(--text);
}

.btn-pause:hover {
  background: var(--bg-hover);
  transform: scale(1.08);
}

/* Icon buttons */
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
  justify-content: center;
  opacity: 0;
}

.monitor-card:hover .icon-btn {
  opacity: 1;
}

.icon-btn svg { width: 18px; height: 18px; }
.icon-btn:hover { background: var(--bg-active); color: var(--text); }
.icon-btn-danger:hover { color: var(--error); }

@media (max-width: 768px) {
  .monitor-card { flex-wrap: wrap; gap: 0.5rem; }
  .card-left { min-width: 0; flex: 1; }
  .icon-btn { opacity: 1; }
  .card-meta { order: 10; width: 100%; }
}
</style>
