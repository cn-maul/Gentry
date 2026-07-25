<template>
  <div class="layout">
    <!-- ═══ Left Sidebar ═══ -->
    <aside class="sidebar-left">
      <div class="sidebar-brand">
        <router-link to="/" class="brand-link">
          <span class="brand-icon">
            <svg viewBox="0 0 24 24" fill="currentColor" width="22" height="22">
              <circle cx="12" cy="12" r="3"/>
              <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 18c-4.41 0-8-3.59-8-8s3.59-8 8-8 8 3.59 8 8-3.59 8-8 8z"/>
              <path d="M12 6c-3.31 0-6 2.69-6 6s2.69 6 6 6 6-2.69 6-6-2.69-6-6-6zm0 10c-2.21 0-4-1.79-4-4s1.79-4 4-4 4 1.79 4 4-1.79 4-4 4z"/>
            </svg>
          </span>
          <span class="brand-text">Gentry</span>
        </router-link>
      </div>

      <nav class="sidebar-nav">
        <router-link to="/" class="nav-item" :class="{ active: $route.path === '/' }">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></svg>
          <span>监控列表</span>
        </router-link>
        <router-link to="/add" class="nav-item" :class="{ active: $route.path === '/add' }">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="16"/><line x1="8" y1="12" x2="16" y2="12"/></svg>
          <span>新增监控</span>
        </router-link>
        <router-link to="/push" class="nav-item" :class="{ active: $route.path === '/push' }">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg>
          <span>通知方式</span>
        </router-link>
        <router-link to="/scan-rules" class="nav-item" :class="{ active: $route.path === '/scan-rules' }">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>
          <span>高级规则</span>
        </router-link>
        <router-link to="/settings" class="nav-item" :class="{ active: $route.path === '/settings' }">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
          <span>设置</span>
        </router-link>
      </nav>

      <div class="sidebar-footer">
        <button class="nav-item theme-btn" :title="isDark ? '切换亮色模式' : '切换暗色模式'" @click="toggleTheme">
          <svg v-if="isDark" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18"><circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/></svg>
          <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>
          <span>{{ isDark ? '亮色' : '暗色' }}</span>
        </button>
        <UpdatePanel />
      </div>
    </aside>

    <!-- ═══ Main Content ═══ -->
    <main class="main-content" :class="{ 'has-right-sidebar': showRightSidebar }">
      <slot />
    </main>

    <!-- ═══ Right Sidebar: Stats ═══ -->
    <aside class="sidebar-right" v-if="showRightSidebar">
      <div class="stats-panel">
        <div class="panel-header">
          <h3>系统概览</h3>
          <span class="status-dot" :class="statsOk ? 'dot-ok' : 'dot-err'" :title="statsOk ? '正常运行' : '连接异常'" />
        </div>

        <template v-if="statsLoading && stats.total_monitors === 0">
          <div class="skel" /><div class="skel skel-sm" /><div class="skel-row"><div class="skel" /><div class="skel" /></div>
        </template>
        <template v-else-if="statsError">
          <p class="stats-error">暂时无法读取统计</p>
        </template>
        <template v-else>
          <!-- System Status -->
          <div class="stat-card">
            <div class="stat-hero">
              <span class="stat-hero-num">{{ stats.running_monitors }}</span>
              <span class="stat-hero-sep">/</span>
              <span class="stat-hero-total">{{ stats.total_monitors }}</span>
            </div>
            <p class="stat-hero-label">监控器运行中</p>
            <div class="progress-track">
              <div class="progress-fill" :style="{ width: monitorPercent + '%' }" />
            </div>
          </div>

          <!-- Recent Activity -->
          <div class="stat-section">
            <p class="section-title">近期活动</p>
            <div class="stat-grid">
              <div class="grid-cell">
                <span class="grid-num blue">{{ stats.updates_last_hour }}</span>
                <span class="grid-label">近1小时更新</span>
              </div>
              <div class="grid-cell">
                <span class="grid-num" :class="stats.unnotified_updates > 0 ? 'orange' : ''">{{ stats.unnotified_updates }}</span>
                <span class="grid-label">待推送</span>
              </div>
            </div>
          </div>

          <!-- Cumulative -->
          <div class="stat-section">
            <p class="section-title">累计数据</p>
            <div class="stat-row">
              <span class="row-label">今日已推送</span>
              <span class="row-value green">{{ stats.pushed_today }}</span>
            </div>
            <div class="stat-row">
              <span class="row-label">变更记录</span>
              <span class="row-value">{{ formatNum(stats.total_updates) }}</span>
            </div>
            <div class="stat-row">
              <span class="row-label">推送账户</span>
              <span class="row-value">{{ stats.total_accounts }}</span>
            </div>
          </div>
        </template>

        <div class="panel-footer">
          <span class="updated-text">{{ lastUpdatedText }}</span>
        </div>
      </div>
    </aside>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { fetchStats } from '../api/monitors'
import UpdatePanel from './UpdatePanel.vue'

const route = useRoute()
const showRightSidebar = computed(() => route.name === 'dashboard')

const STORAGE_KEY = 'gentry_theme'
const isDark = ref(false)
const statsLoading = ref(true)
const statsError = ref(false)
const statsOk = ref(true)
const lastUpdated = ref(null)

const stats = reactive({
  total_monitors: 0,
  running_monitors: 0,
  total_updates: 0,
  updates_last_hour: 0,
  unnotified_updates: 0,
  pushed_today: 0,
  total_accounts: 0,
})

const monitorPercent = computed(() => {
  if (!stats.total_monitors) return 0
  return Math.round((stats.running_monitors / stats.total_monitors) * 100)
})

const lastUpdatedText = computed(() => {
  if (!lastUpdated.value) return ''
  const diff = Math.floor((Date.now() - lastUpdated.value) / 1000)
  if (diff < 10) return '刚刚更新'
  if (diff < 60) return `${diff}秒前更新`
  return `${Math.floor(diff / 60)}分钟前更新`
})

let statsTimer = null
let tickTimer = null

function formatNum(n) {
  if (n >= 10000) return (n / 10000).toFixed(1) + '万'
  if (n >= 1000) return n.toLocaleString('zh-CN')
  return n
}

function applyTheme(dark) {
  document.documentElement.classList.toggle('dark', dark)
  isDark.value = dark
  localStorage.setItem(STORAGE_KEY, dark ? 'dark' : 'light')
}

function toggleTheme() {
  applyTheme(!isDark.value)
}

async function loadStats() {
  try {
    const res = await fetchStats()
    if (res.code === 0 && res.data) {
      Object.assign(stats, res.data)
      statsOk.value = true
      lastUpdated.value = Date.now()
    } else {
      statsOk.value = false
    }
  } catch (_) {
    statsOk.value = false
    if (!lastUpdated.value) statsError.value = true
  } finally {
    statsLoading.value = false
  }
}

onMounted(() => {
  const saved = localStorage.getItem(STORAGE_KEY)
  applyTheme(saved === 'dark')
  loadStats()
  statsTimer = setInterval(loadStats, 15000)
  tickTimer = setInterval(() => { lastUpdated.value = lastUpdated.value }, 10000)
})

onUnmounted(() => {
  if (statsTimer) clearInterval(statsTimer)
  if (tickTimer) clearInterval(tickTimer)
})
</script>

<style scoped>
/* ═══ Layout ═══ */
.layout {
  display: flex;
  min-height: 100vh;
  background: var(--bg-surface);
}

/* ═══ Left Sidebar ═══ */
.sidebar-left {
  width: 200px;
  min-width: 200px;
  background: var(--bg-base);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  z-index: 100;
}

.sidebar-brand {
  padding: 1.25rem 1rem;
  border-bottom: 1px solid var(--border-light);
}

.brand-link {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  text-decoration: none;
  color: var(--text);
}

.brand-icon { color: var(--green); display: flex; }
.brand-text { font-size: 1rem; font-weight: 700; letter-spacing: -0.3px; }

.sidebar-nav {
  flex: 1;
  padding: 0.75rem 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  padding: 0.55rem 0.75rem;
  border-radius: var(--radius);
  color: var(--text-secondary);
  text-decoration: none;
  font-size: 0.8125rem;
  font-weight: 600;
  transition: color 0.2s ease, background-color 0.2s ease;
  border: none;
  background: none;
  cursor: pointer;
  width: 100%;
  text-align: left;
}

.nav-item:hover {
  color: var(--text);
  background: var(--bg-hover);
}

.nav-item.active {
  color: var(--text);
  background: var(--bg-elevated);
  box-shadow: inset 3px 0 0 var(--green);
  border-radius: 0 var(--radius) var(--radius) 0;
}

.nav-item svg { flex-shrink: 0; opacity: 0.7; }
.nav-item.active svg { opacity: 1; }

.sidebar-footer {
  padding: 0.5rem;
  border-top: 1px solid var(--border-light);
}

.theme-btn { font-size: 0.75rem; color: var(--text-muted); }
.theme-btn:hover { color: var(--text-secondary); }

/* ═══ Main Content ═══ */
.main-content {
  flex: 1;
  margin-left: 200px;
  padding: 2rem;
  min-width: 0;
  max-width: calc(1120px + 4rem);
  background: var(--bg-base);
}

.main-content.has-right-sidebar {
  margin-right: 260px;
}

/* ═══ Right Sidebar ═══ */
.sidebar-right {
  width: 260px;
  min-width: 260px;
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  overflow-y: auto;
  padding: 1.25rem 1rem;
}

/* ═══ Stats Panel ═══ */
.stats-panel {
  background: var(--bg-base);
  border: 1px solid var(--border-light);
  border-radius: var(--radius-lg);
  padding: 1rem;
  position: sticky;
  top: 1rem;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 1rem;
}

.panel-header h3 {
  font-size: 0.6875rem;
  font-weight: 700;
  color: var(--text-muted);
}

.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}
.dot-ok { background: var(--green); box-shadow: 0 0 6px rgba(29, 185, 84, 0.4); }
.dot-err { background: var(--error); box-shadow: 0 0 6px rgba(211, 47, 47, 0.4); }

/* Skeleton */
.skel {
  height: 2rem;
  background: var(--bg-elevated);
  border-radius: var(--radius);
  margin-bottom: 0.5rem;
  animation: skel-pulse 1.2s ease-in-out infinite;
}
.skel-sm { height: 0.75rem; width: 60%; }
.skel-row { display: flex; gap: 0.5rem; }
.skel-row .skel { flex: 1; height: 3rem; }
@keyframes skel-pulse { 0%,100% { opacity: 0.5; } 50% { opacity: 1; } }

.stats-error {
  font-size: 0.8125rem;
  color: var(--text-muted);
  text-align: center;
  padding: 1.5rem 0;
}

/* Hero: running ratio */
.stat-card {
  margin-bottom: 0.75rem;
}

.stat-hero {
  display: flex;
  align-items: baseline;
  gap: 0.15rem;
}

.stat-hero-num {
  font-size: 2rem;
  font-weight: 800;
  color: var(--text);
  line-height: 1;
  font-variant-numeric: tabular-nums;
}

.stat-hero-sep {
  font-size: 1.25rem;
  font-weight: 400;
  color: var(--text-muted);
  margin: 0 0.1rem;
}

.stat-hero-total {
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--text-muted);
}

.stat-hero-label {
  font-size: 0.75rem;
  color: var(--text-secondary);
  margin-top: 0.2rem;
  font-weight: 500;
}

.progress-track {
  height: 4px;
  background: var(--bg-elevated);
  border-radius: 2px;
  margin-top: 0.5rem;
  overflow: hidden;
}

.progress-fill {
  height: 100%;
  background: var(--green);
  border-radius: 2px;
  transition: width 0.6s cubic-bezier(0.23, 1, 0.32, 1);
}

/* Section */
.stat-section {
  padding: 0.65rem 0;
  border-top: 1px solid var(--border-light);
}

.section-title {
  font-size: 0.625rem;
  font-weight: 700;
  color: var(--text-muted);
  margin-bottom: 0.5rem;
}

/* 2-col grid */
.stat-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.35rem;
}

.grid-cell {
  display: flex;
  flex-direction: column;
  padding: 0.5rem 0.6rem;
  border-radius: var(--radius);
  background: var(--bg-surface);
}

.grid-num {
  font-size: 1.25rem;
  font-weight: 700;
  line-height: 1.2;
  font-variant-numeric: tabular-nums;
  color: var(--text);
}

.grid-num.blue { color: #1976d2; }
.grid-num.orange { color: var(--warning); }

.grid-label {
  font-size: 0.625rem;
  color: var(--text-muted);
  margin-top: 0.1rem;
  font-weight: 500;
}

/* Row list */
.stat-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 0.3rem 0;
}

.row-label {
  font-size: 0.75rem;
  color: var(--text-secondary);
}

.row-value {
  font-size: 0.8125rem;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  color: var(--text);
}

.row-value.green { color: var(--green); }

/* Footer */
.panel-footer {
  margin-top: 0.75rem;
  padding-top: 0.6rem;
  border-top: 1px solid var(--border-light);
}

.updated-text {
  font-size: 0.625rem;
  color: var(--text-muted);
}

/* ═══ Responsive ═══ */
@media (max-width: 1100px) {
  .sidebar-right { display: none; }
  .main-content.has-right-sidebar { margin-right: 0; }
}

@media (max-width: 768px) {
  .sidebar-left {
    position: fixed;
    bottom: 0;
    top: auto;
    left: 0;
    right: 0;
    width: 100%;
    min-width: 100%;
    height: 56px;
    flex-direction: row;
    border-right: none;
    border-top: 1px solid var(--border);
    z-index: 200;
    background: var(--bg-base);
  }

  .sidebar-brand,
  .sidebar-footer { display: none; }

  .sidebar-nav {
    flex-direction: row;
    justify-content: space-around;
    padding: 0;
    gap: 0;
    width: 100%;
    align-items: center;
  }

  .nav-item {
    flex-direction: column;
    gap: 0.15rem;
    padding: 0.4rem 0.5rem;
    font-size: 0.625rem;
    justify-content: center;
    box-shadow: none;
    border-radius: var(--radius);
  }

  .nav-item.active {
    color: var(--green);
    background: var(--success-bg);
    box-shadow: none;
  }

  .nav-item.active svg { opacity: 1; }

  .nav-item:nth-child(n+5) { display: none; }

  .main-content {
    margin-left: 0;
    padding: 1rem;
    padding-bottom: 72px;
    max-width: none;
  }

  .main-content.has-right-sidebar { margin-right: 0; }
}
</style>
