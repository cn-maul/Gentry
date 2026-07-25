<template>
  <div class="form-summary">
    <div class="section-header"><h2>确认配置</h2></div>

    <div class="summary-grid">
      <div class="summary-row">
        <span class="summary-label">监控类型</span>
        <span class="summary-value">
          <span class="type-badge" :class="'badge-' + form.monitorType">
            {{ form.monitorType === 'presence' ? '新增检测' : '价格监控' }}
          </span>
        </span>
      </div>
      <div class="summary-row">
        <span class="summary-label">名称</span>
        <span class="summary-value">{{ form.basic.name || '—' }}</span>
      </div>
      <div class="summary-row">
        <span class="summary-label">URL</span>
        <span class="summary-value url-value">{{ form.basic.url || '—' }}</span>
      </div>
      <div class="summary-row" v-if="form.basic.group">
        <span class="summary-label">分组</span>
        <span class="summary-value">{{ form.basic.group }}</span>
      </div>
      <div class="summary-row">
        <span class="summary-label">检查间隔</span>
        <span class="summary-value">{{ formatInterval(form.basic.interval) }}</span>
      </div>
      <div class="summary-row">
        <span class="summary-label">{{ form.monitorType === 'field_transition' ? '价格来源' : '内容区域' }}</span>
        <code class="summary-code">{{ form.extraction.containerSelector || '—' }}</code>
      </div>
      <div class="summary-row" v-if="form.extraction.itemSelector">
        <span class="summary-label">{{ form.monitorType === 'field_transition' ? '商品条目' : '列表项' }}</span>
        <code class="summary-code">{{ form.extraction.itemSelector }}</code>
      </div>
      <div class="summary-row">
        <span class="summary-label">字段</span>
        <span class="summary-value">{{ fieldNames }}</span>
      </div>

      <template v-if="form.monitorType === 'field_transition'">
        <div class="summary-divider"></div>
        <div class="summary-row">
          <span class="summary-label">页面场景</span>
          <span class="summary-value">{{ form.rule.pageMode === 'list' ? '商品列表页' : '单商品详情页' }}</span>
        </div>
        <div class="summary-row">
          <span class="summary-label">身份模式</span>
          <span class="summary-value">{{ form.rule.identity.mode === 'source_url' ? '页面 URL' : '字段: ' + form.rule.identity.field }}</span>
        </div>
        <div class="summary-row">
          <span class="summary-label">监控字段</span>
          <span class="summary-value">{{ form.rule.target.field }} ({{ valueTypeLabel(form.rule.target.valueType) }})</span>
        </div>
        <div class="summary-row">
          <span class="summary-label">变化规则</span>
          <span class="summary-value">{{ transitionLabel }}</span>
        </div>
      </template>

      <div class="summary-divider"></div>
      <div class="summary-row">
        <span class="summary-label">通知方式</span>
        <span class="summary-value">{{ notificationLabel }}</span>
      </div>
      <div class="summary-row">
        <span class="summary-label">推送账户</span>
        <span class="summary-value">{{ form.notification.accountIds.length }} 个</span>
      </div>
    </div>

    <div class="summary-baseline" v-if="form.monitorType === 'field_transition'">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg>
      <span>首次检查仅建立基线，不发送通知</span>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  form: { type: Object, required: true },
})

const fieldNames = computed(() => {
  return props.form.extraction.fields.map(f => f.name).filter(Boolean).join(', ') || '—'
})

const transitionLabel = computed(() => {
  const t = props.form.rule.transition
  if (t.operator === 'at_or_below') {
    return t.targetPrice === '' ? '降到目标价及以下' : `价格 ≤ ${t.targetPrice} 元`
  }
  const hasAmt = t.minAmount !== '' && t.minAmount !== null && Number(t.minAmount) > 0
  const hasPct = t.minPercent !== '' && t.minPercent !== null && Number(t.minPercent) > 0
  if (hasAmt && hasPct) return `降价 ≥ ${t.minAmount} 元 且 ≥ ${t.minPercent}%`
  if (hasAmt) return `降价 ≥ ${t.minAmount} 元`
  if (hasPct) return `降价 ≥ ${t.minPercent}%`
  return '任意降价'
})

const notificationLabel = computed(() => {
  if (props.form.notification.filter === 'keyword') {
    return `关键词匹配: ${props.form.notification.keywords}`
  }
  return props.form.monitorType === 'field_transition' ? '所有符合条件的价格事件' : '所有新内容'
})

function valueTypeLabel(t) {
  const map = { money: '金额' }
  return map[t] || t
}

function formatInterval(s) {
  if (!s) return '—'
  if (s >= 3600) return `${Math.round(s / 3600)} 小时`
  if (s >= 60) return `${Math.round(s / 60)} 分钟`
  return `${s} 秒`
}
</script>

<style scoped>
.form-summary {
  padding: 1rem;
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
  margin-bottom: 1rem;
}
.section-header {
  margin-bottom: 1rem;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid var(--border-light);
}
.section-header h2 { font-size: 1rem; font-weight: 700; color: var(--text); }

.summary-grid { display: flex; flex-direction: column; gap: 0; }
.summary-row {
  display: flex;
  align-items: baseline;
  padding: 0.4rem 0;
  gap: 1rem;
}
.summary-label {
  font-size: 0.6875rem;
  font-weight: 700;
  color: var(--text-muted);
  min-width: 70px;
  flex-shrink: 0;
}
.summary-value {
  font-size: 0.8125rem;
  color: var(--text);
  word-break: break-all;
}
.url-value { font-size: 0.75rem; color: var(--text-secondary); }
.summary-code {
  font-size: 0.75rem; color: var(--green);
  background: var(--bg-elevated); padding: 0.1rem 0.35rem; border-radius: 4px;
}
.summary-divider {
  height: 1px;
  background: var(--border-light);
  margin: 0.25rem 0;
}
.type-badge {
  font-size: 0.625rem; font-weight: 700;
  padding: 0.1rem 0.4rem; border-radius: var(--radius-pill);
}
.badge-presence { background: var(--success-bg); color: var(--green); }
.badge-field_transition { background: #e3f2fd; color: #1976d2; }
.dark .badge-field_transition { background: #0d2137; color: #64b5f6; }

.summary-baseline {
  display: flex;
  align-items: flex-start;
  gap: 0.5rem;
  margin-top: 0.75rem;
  padding: 0.5rem 0.6rem;
  background: var(--bg-elevated);
  border-radius: var(--radius-lg);
  font-size: 0.75rem;
  color: var(--text-secondary);
}
.summary-baseline svg { flex-shrink: 0; margin-top: 1px; color: var(--text-muted); }
</style>
