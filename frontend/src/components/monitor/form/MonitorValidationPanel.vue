<template>
  <div class="validation-panel" v-if="loading || result">
    <div class="loading" v-if="loading">
      <div class="spinner" />
      <p>正在抓取网页并验证配置...</p>
    </div>
    <template v-else-if="result">
    <div class="section-header">
      <h2>验证结果</h2>
      <span class="validation-status" :class="result.valid ? 'status-ok' : 'status-err'">
        {{ result.valid ? '验证通过' : '存在问题' }}
      </span>
    </div>

    <div class="validation-grid" v-if="result.items && result.items.length > 0">
      <div class="validation-item" v-for="(item, idx) in result.items.slice(0, 5)" :key="idx" :class="'item-' + item.status">
        <div class="item-header">
          <span class="item-status-icon">
            <svg v-if="item.status === 'ok'" viewBox="0 0 24 24" fill="currentColor" width="14" height="14"><path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z"/></svg>
            <svg v-else-if="item.status === 'warn'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
            <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
          </span>
          <span class="item-label">{{ item.label }}</span>
        </div>
        <p class="item-detail">{{ item.detail }}</p>
        <div class="item-samples" v-if="item.samples && item.samples.length > 0">
          <div class="sample-row" v-for="(s, si) in item.samples.slice(0, 3)" :key="si">
            <span class="sample-key" :title="s.item_key">{{ s.item_key }}</span>
            <span class="sample-raw">{{ s.raw }}</span>
            <span class="sample-arrow" v-if="s.normalized">→</span>
            <span class="sample-normalized" v-if="s.normalized">{{ s.normalized }}</span>
            <span class="sample-currency" v-if="s.currency">{{ s.currency }}</span>
          </div>
        </div>
      </div>
    </div>

    <div class="validation-errors" v-if="result.errors && result.errors.length > 0">
      <div class="error-item" v-for="(err, idx) in result.errors" :key="idx">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
        <span>{{ err }}</span>
      </div>
    </div>

    <div class="validation-summary" v-if="result.summary">
      <p>{{ result.summary }}</p>
    </div>
    </template>
  </div>
</template>

<script setup>
defineProps({
  result: { type: Object, default: null },
  loading: { type: Boolean, default: false },
})
</script>

<style scoped>
.validation-panel {
  padding: 1rem;
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
  margin-bottom: 1rem;
}
.loading { display: flex; align-items: center; justify-content: center; gap: 0.5rem; color: var(--text-secondary); min-height: 72px; }
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid var(--border-light);
}
.section-header h2 { font-size: 1rem; font-weight: 700; color: var(--text); }
.validation-status {
  font-size: 0.6875rem; font-weight: 700;
  padding: 0.15rem 0.5rem; border-radius: var(--radius-pill);
}
.status-ok { background: var(--success-bg); color: var(--success); }
.status-err { background: var(--error-bg); color: var(--error); }

.validation-grid { display: flex; flex-direction: column; gap: 0.5rem; margin-bottom: 1rem; }
.validation-item {
  padding: 0.65rem 0.75rem;
  background: var(--bg-card);
  border-radius: var(--radius-lg);
  border-left: 3px solid var(--border);
}
.item-ok { border-left-color: var(--green); }
.item-warn { border-left-color: var(--warning); }
.item-err { border-left-color: var(--error); }

.item-header { display: flex; align-items: center; gap: 0.4rem; margin-bottom: 0.2rem; }
.item-status-icon { display: flex; }
.item-ok .item-status-icon { color: var(--green); }
.item-warn .item-status-icon { color: var(--warning); }
.item-err .item-status-icon { color: var(--error); }
.item-label { font-size: 0.8125rem; font-weight: 700; color: var(--text); }
.item-detail { font-size: 0.75rem; color: var(--text-secondary); line-height: 1.4; }

.item-samples { margin-top: 0.4rem; display: flex; flex-direction: column; gap: 0.2rem; }
.sample-row {
  display: flex; align-items: center; gap: 0.4rem;
  font-size: 0.75rem; font-family: monospace;
  padding: 0.2rem 0.4rem; background: var(--bg-elevated); border-radius: 4px;
}
.sample-raw { color: var(--text-muted); }
.sample-key { color: var(--text-secondary); max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.sample-arrow { color: var(--text-muted); }
.sample-normalized { color: var(--green); font-weight: 700; }
.sample-currency { color: var(--text-muted); margin-left: auto; }

.validation-errors { display: flex; flex-direction: column; gap: 0.3rem; margin-bottom: 0.75rem; }
.error-item {
  display: flex; align-items: center; gap: 0.4rem;
  padding: 0.5rem 0.75rem;
  background: var(--error-bg); color: var(--error);
  border-radius: var(--radius-lg);
  font-size: 0.8125rem;
}
.validation-summary {
  font-size: 0.75rem; color: var(--text-muted);
  padding-top: 0.5rem;
  border-top: 1px solid var(--border-light);
}
</style>
