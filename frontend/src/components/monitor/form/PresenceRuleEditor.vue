<template>
  <div class="settings-section">
    <div class="section-header">
      <h2>新增条目规则</h2>
      <p class="section-desc">当页面中出现新的条目时触发通知</p>
    </div>

    <div class="form-group">
      <label class="checkbox-label">
        <input type="checkbox" :checked="true" disabled />
        检测到新增条目后推送通知
      </label>
      <p class="hint">当前版本仅支持「有新内容就推送」模式</p>
    </div>
  </div>
</template>

<script setup>
</script>

<style scoped>
.section-header { margin-bottom: 1.25rem; padding-bottom: 0.75rem; border-bottom: 1px solid var(--border-light); }
.section-header h2 { font-size: 1.125rem; font-weight: 700; color: var(--text); margin-bottom: 0.15rem; }
.section-desc { font-size: 0.8125rem; color: var(--text-secondary); }
.hint { font-size: 0.75rem; color: var(--text-muted); margin-top: 0.25rem; }
</style>
