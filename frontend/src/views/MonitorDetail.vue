<template>
  <div class="monitor-detail">
    <div class="page-header">
      <div>
        <router-link to="/" class="back-btn pill-link">← 返回</router-link>
        <h1>{{ monitor ? monitor.name : '加载中...' }}</h1>
      </div>
      <div class="header-actions" v-if="monitor">
        <button class="circle-btn" :class="monitor.is_running ? 'btn-pause' : 'btn-play'" @click="toggleRun" :disabled="actionLoading" :title="monitor.is_running ? '暂停' : '启动'">
          <svg v-if="monitor.is_running" viewBox="0 0 24 24" fill="currentColor" width="18" height="18"><rect x="6" y="4" width="4" height="16" rx="1"/><rect x="14" y="4" width="4" height="16" rx="1"/></svg>
          <svg v-else viewBox="0 0 24 24" fill="currentColor" width="18" height="18"><path d="M8 5v14l11-7z"/></svg>
        </button>
        <router-link :to="`/edit/${encodeURIComponent(monitor.name)}`" class="btn btn-ghost btn-sm">编辑</router-link>
        <button class="btn btn-ghost btn-sm" style="color: var(--error);" @click="confirmDelete">删除</button>
      </div>
    </div>

    <div class="toast toast-success" v-if="successMsg">{{ successMsg }}</div>
    <div class="toast toast-warning" v-if="pageErrorMsg">{{ pageErrorMsg }}</div>

    <div class="loading" v-if="loading">
      <div class="spinner" />
      <p>加载监控器详情...</p>
    </div>

    <div class="empty" v-else-if="error">
      <div class="empty-icon">❌</div>
      <p>{{ error }}</p>
      <button class="btn btn-primary btn-sm" style="margin-top: 1rem;" @click="loadData">重试</button>
    </div>

    <template v-else-if="monitor">
      <div class="modal-overlay" v-if="showDeleteConfirm" @click.self="showDeleteConfirm = false">
        <div class="modal-container">
          <div class="modal-header">
            <h2>确认删除</h2>
            <button class="modal-close" @click="showDeleteConfirm = false">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
            </button>
          </div>
          <div class="modal-body">
            <p>确定要删除监控器「{{ monitor?.name }}」吗？</p>
            <p style="margin-top: 0.5rem;">删除后无法恢复，相关更新记录也会被清除。</p>
          </div>
          <div class="modal-footer">
            <button class="btn btn-ghost" @click="showDeleteConfirm = false">取消</button>
            <button class="btn btn-danger" @click="handleDelete" :disabled="actionLoading">{{ actionLoading ? '删除中...' : '确认删除' }}</button>
          </div>
        </div>
      </div>
      <div class="detail-panel settings-section">
        <div class="detail-left">
          <div class="status-summary" :class="{ 'status-error': monitor.last_error }">
            <div class="status-row">
              <StatusBadge :status="monitor.is_running ? 'running' : (monitor.last_error ? 'error' : 'stopped')" />
              <span class="interval-badge">{{ formatInterval(monitor.check_interval) }}</span>
            </div>
            <div class="status-description">
              <template v-if="monitor.last_error">
                <span class="desc-error">检查失败</span>
                <span class="desc-detail">{{ monitor.last_error }}</span>
              </template>
              <template v-else-if="monitor.is_running">
                <span class="desc-ok">运行正常</span>
                <span class="desc-detail" v-if="monitor.last_check">{{ formatTime(monitor.last_check) }}完成检查</span>
              </template>
              <template v-else>
                <span class="desc-paused">已暂停</span>
              </template>
            </div>
          </div>
          <div class="status-grid">
            <div class="status-item">
              <span class="status-label">网址</span>
              <span class="status-value">{{ monitor.url }}</span>
            </div>
            <div class="status-item" v-if="monitor.group">
              <span class="status-label">分组</span>
              <span class="status-value">{{ monitor.group }}</span>
            </div>
            <div class="status-item">
              <span class="status-label">上次检查</span>
              <span class="status-value">{{ monitor.last_check ? formatTime(monitor.last_check) : '—' }}</span>
            </div>
            <div class="status-item">
              <span class="status-label">更新次数</span>
              <span class="status-value">{{ monitor.updates_count || 0 }}</span>
            </div>
            <div class="status-item">
              <span class="status-label">下次检查</span>
              <span class="status-value">{{ monitor.next_check ? formatTime(monitor.next_check) : '—' }}</span>
            </div>
            <div class="status-item">
              <span class="status-label">检查耗时</span>
              <span class="status-value">{{ monitor.last_duration ? formatDuration(monitor.last_duration) : '—' }}</span>
            </div>
            <div class="status-item" v-if="monitor.strategy_type">
              <span class="status-label">监控类型</span>
              <span class="status-value">{{ monitor.strategy_type === 'field_transition' ? '价格监控' : '新增检测' }}</span>
            </div>
            <div class="status-item" v-if="monitor.baseline_status">
              <span class="status-label">基线状态</span>
              <span class="status-value">{{ monitor.baseline_status === 'ready' ? '已建立' : '待建立' }}</span>
            </div>
          </div>
          <div class="status-actions" v-if="monitor.strategy_type === 'field_transition'">
            <button class="btn btn-sm btn-ghost" @click="handleResetBaseline" :disabled="actionLoading">
              重新记录当前价格
            </button>
            <button class="btn btn-sm btn-ghost" @click="handleManualCheck" :disabled="actionLoading">
              立即检查
            </button>
          </div>
          <div class="status-actions" v-else>
            <button class="btn btn-sm btn-ghost" @click="handleManualCheck" :disabled="actionLoading">
              立即检查
            </button>
          </div>
        </div>

        <div class="detail-right" v-if="allAccounts.length > 0">
          <div class="detail-divider"></div>
          <div class="accounts-header">
            <h3>推送账户</h3>
            <router-link to="/push" class="link-sm">管理账户</router-link>
          </div>
          <div class="accounts-list">
            <label v-for="acc in allAccounts" :key="acc.id" class="account-checkbox">
              <input
                type="checkbox"
                :value="acc.id"
                :checked="selectedAccountIDs.includes(acc.id)"
                @change="toggleAccount(acc.id)"
              />
              <span class="acc-name">{{ acc.name }}</span>
              <span class="acc-service-badge" :class="'badge-' + acc.service">{{ serviceLabel(acc.service) }}</span>
            </label>
          </div>
          <p class="hint" v-if="selectedAccountIDs.length === 0">未启用任何推送账户，发现更新时不会推送通知</p>
          <div class="accounts-actions">
            <button class="btn btn-primary btn-sm" :disabled="savingAccounts" @click="saveAccounts">
              {{ savingAccounts ? '保存中...' : '保存' }}
            </button>
          </div>
        </div>
      </div>

      <div class="settings-section" v-if="monitor?.strategy_type !== 'field_transition'">
        <div class="section-header">
          <h2>更新历史</h2>
          <button class="btn btn-sm btn-ghost" :disabled="markLoading" @click="handleMarkAll" v-if="records.length > 0">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><polyline points="9 11 12 14 22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/></svg>
            全部标为已推送
          </button>
        </div>
        <UpdateTable :records="records" :loading="updatesLoading" />
        <div class="pagination" v-if="updatesTotal > updatesPageSize">
          <button class="btn btn-sm btn-ghost" :disabled="updatesPage <= 1 || updatesLoading" @click="changeUpdatesPage(updatesPage - 1)">上一页</button>
          <span>第 {{ updatesPage }} / {{ totalUpdatePages }} 页</span>
          <button class="btn btn-sm btn-ghost" :disabled="updatesPage >= totalUpdatePages || updatesLoading" @click="changeUpdatesPage(updatesPage + 1)">下一页</button>
        </div>
      </div>

      <!-- 价格监控当前快照 -->
      <div class="settings-section" v-if="monitor?.strategy_type === 'field_transition'">
        <div class="section-header">
          <h2>当前快照</h2>
        </div>
        <div class="loading" v-if="snapshotsLoading"><div class="spinner" /></div>
        <div class="snapshots-table" v-else-if="snapshots.length > 0">
          <div class="snapshot-row snapshot-header">
            <span class="snap-col snap-key">商品标识</span>
            <span class="snap-col snap-price">当前价格</span>
            <span class="snap-col snap-currency">币种</span>
            <span class="snap-col snap-valid">状态</span>
            <span class="snap-col snap-time">最后更新</span>
          </div>
          <div class="snapshot-row" v-for="snap in snapshots" :key="snap.id || snap.item_key">
            <span class="snap-col snap-key snapshot-identity" :title="snap.item_title ? `${snap.item_title} (${snap.item_key})` : snap.item_key">
              <strong v-if="snap.item_title">{{ snap.item_title }}</strong>
              <code v-if="snap.item_title && snap.item_key !== snap.item_title">{{ snap.item_key }}</code>
              <span v-else>{{ snap.item_key }}</span>
            </span>
            <span class="snap-col snap-price">{{ snap.price_display || '—' }}</span>
            <span class="snap-col snap-currency">{{ snap.currency || '—' }}</span>
            <span class="snap-col snap-valid">
              <span class="valid-dot" :class="snap.price_valid ? 'valid-yes' : 'valid-no'" />
              {{ snap.price_valid ? '有效' : '无效' }}
            </span>
            <span class="snap-col snap-time">{{ snap.last_seen_at ? formatTime(snap.last_seen_at) : '—' }}</span>
          </div>
        </div>
        <div class="empty" v-else-if="!snapshotsLoading">
          <p>暂无快照数据</p>
        </div>
      </div>

      <!-- 价格监控事件历史 -->
      <div class="settings-section" v-if="monitor?.strategy_type === 'field_transition'">
        <div class="section-header">
          <h2>价格变动历史</h2>
          <button class="btn btn-sm btn-ghost" @click="loadEvents">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14"><path d="M1 4v6h6M23 20v-6h-6"/><path d="M20.49 9A9 9 0 0 0 5.64 5.64L1 10m22 4l-4.64 4.36A9 9 0 0 1 3.51 15"/></svg>
            刷新
          </button>
        </div>
        <div class="events-table" v-if="events.length > 0">
          <div class="event-row" v-for="evt in events" :key="evt.id">
            <div class="event-type-badge" :class="'event-' + evt.event_type">
              {{ evt.event_type === 'price_dropped' ? '降价' : (evt.event_type === 'price_target_reached' ? '到价' : evt.event_type) }}
            </div>
            <div class="event-info">
              <span class="event-title">{{ evt.title }}</span>
              <span class="event-price" v-if="evt.event_type === 'price_dropped' || evt.event_type === 'price_target_reached'">
                <span class="old-price">{{ evt.old_value }}</span>
                <span class="price-arrow">→</span>
                <span class="new-price">{{ evt.new_value }}</span>
                <span class="price-drop" v-if="evt.event_type === 'price_dropped' && evt.change_percent > 0">-{{ evt.change_percent.toFixed(1) }}%</span>
              </span>
            </div>
            <div class="event-time">{{ formatTime(evt.occurred_at) }}</div>
            <div class="event-notified" :class="'status-' + eventDeliveryStatus(evt)">
              {{ deliveryStatusLabel(evt) }}
            </div>
          </div>
        </div>
        <div class="empty" v-else-if="!eventsLoading">
          <p>暂无价格变动记录</p>
        </div>
        <div class="pagination" v-if="eventsTotal > eventsPageSize">
          <button class="btn btn-sm btn-ghost" :disabled="eventsPage <= 1" @click="changeEventsPage(eventsPage - 1)">上一页</button>
          <span>第 {{ eventsPage }} / {{ totalEventPages }} 页</span>
          <button class="btn btn-sm btn-ghost" :disabled="eventsPage >= totalEventPages" @click="changeEventsPage(eventsPage + 1)">下一页</button>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { fetchMonitor, fetchUpdates, fetchEvents, fetchMonitorConfig, fetchSnapshots, fetchAccounts, updateNotifyAccounts, startMonitor, stopMonitor, deleteMonitor, markAllNotified, markRead, resetBaseline, manualCheck } from '../api/monitors'
import StatusBadge from '../components/StatusBadge.vue'
import UpdateTable from '../components/UpdateTable.vue'
import { useToastMessages } from '../composables/useToastMessages'

const route = useRoute()
const router = useRouter()
const { successMsg, pageErrorMsg, showSuccess, showError } = useToastMessages()

const monitor = ref(null)
const records = ref([])
const updatesPage = ref(1)
const updatesPageSize = 20
const updatesTotal = ref(0)
const totalUpdatePages = computed(() => Math.max(1, Math.ceil(updatesTotal.value / updatesPageSize)))
const loading = ref(true)
const updatesLoading = ref(false)
const error = ref(null)
const actionLoading = ref(false)
const markLoading = ref(false)
const showDeleteConfirm = ref(false)

// 推送账户选择
const allAccounts = ref([])
const selectedAccountIDs = ref([])
const accountDirty = ref(false)
const savingAccounts = ref(false)

// 事件历史
const events = ref([])
const eventsPage = ref(1)
const eventsPageSize = 20
const eventsTotal = ref(0)
const eventsLoading = ref(false)
const totalEventPages = computed(() => Math.max(1, Math.ceil(eventsTotal.value / eventsPageSize)))

// 快照
const snapshots = ref([])
const snapshotsLoading = ref(false)

onMounted(loadData)

async function loadData() {
  loading.value = true; error.value = null
  try {
    const [res, configRes, acctsRes] = await Promise.all([
      fetchMonitor(route.params.name),
      fetchMonitorConfig(route.params.name).catch(() => null),
      fetchAccounts().catch(() => ({ data: [] })),
    ])
    if (res.code === 0) {
      monitor.value = res.data
      loadUpdates()
      markRead(route.params.name).catch(() => {})
    } else { error.value = res.message || '监控器不存在' }

    // 加载推送账户列表
    allAccounts.value = (acctsRes.code === 0 ? acctsRes.data : []) || []

    // 解析当前监控器的启用账户 ID
    if (configRes && configRes.code === 0 && configRes.data) {
      selectedAccountIDs.value = configRes.data.notify_account_ids || []
    }

    // 如果是价格监控，加载事件历史
    if (monitor.value?.strategy_type === 'field_transition') {
      loadEvents()
      loadSnapshots()
    }
  } catch (e) { error.value = e.response?.data?.message || e.message }
  finally { loading.value = false }
}

async function loadUpdates() {
  updatesLoading.value = true
  try {
    const res = await fetchUpdates(route.params.name, { page: updatesPage.value, size: updatesPageSize })
    if (res.code === 0 && res.data) {
      records.value = res.data.records || []
      updatesTotal.value = res.data.total || 0
      if (updatesPage.value > totalUpdatePages.value) {
        updatesPage.value = totalUpdatePages.value
        await loadUpdates()
      }
    }
  } catch { /* ignore */ }
  finally { updatesLoading.value = false }
}

async function changeUpdatesPage(page) {
  if (page < 1 || page > totalUpdatePages.value) return
  updatesPage.value = page
  await loadUpdates()
}

async function loadEvents() {
  eventsLoading.value = true
  try {
    const res = await fetchEvents(route.params.name, { page: eventsPage.value, size: eventsPageSize })
    if (res.code === 0 && res.data) {
      events.value = res.data.events || []
      eventsTotal.value = res.data.total || 0
    }
  } catch { /* ignore */ }
  finally { eventsLoading.value = false }
}

async function changeEventsPage(page) {
  if (page < 1 || page > totalEventPages.value) return
  eventsPage.value = page
  await loadEvents()
}

async function toggleRun() {
  actionLoading.value = true
  try {
    if (monitor.value.is_running) {
      await stopMonitor(route.params.name)
      showError('监控器已暂停')
    } else {
      await startMonitor(route.params.name)
      showSuccess('监控器已启动')
    }
    await loadData()
  } catch (e) { showError('操作失败: ' + (e.response?.data?.message || e.message)) }
  finally { actionLoading.value = false }
}

function confirmDelete() { showDeleteConfirm.value = true }

async function handleDelete() {
  actionLoading.value = true
  try {
    await deleteMonitor(route.params.name)
    router.push('/')
  } catch (e) {
    showError('删除失败: ' + (e.response?.data?.message || e.message))
    showDeleteConfirm.value = false; actionLoading.value = false
  }
}

async function handleMarkAll() {
  markLoading.value = true
  try {
    const res = await markAllNotified(route.params.name)
    const n = res.data?.updated || 0
    showSuccess(`已将 ${n} 条记录标为已推送`)
    await loadUpdates()
  } catch (e) { showError('操作失败: ' + (e.message)) }
  finally { markLoading.value = false }
}

function toggleAccount(accountID) {
  const idx = selectedAccountIDs.value.indexOf(accountID)
  if (idx >= 0) {
    selectedAccountIDs.value.splice(idx, 1)
  } else {
    selectedAccountIDs.value.push(accountID)
  }
  accountDirty.value = true
}

async function saveAccounts() {
  savingAccounts.value = true
  try {
    await updateNotifyAccounts(route.params.name, selectedAccountIDs.value)
    showSuccess(selectedAccountIDs.value.length > 0 ? '推送账户已保存' : '已关闭所有推送账户')
    accountDirty.value = false
  } catch (e) {
    showError('保存失败: ' + (e.response?.data?.message || e.message))
  } finally {
    savingAccounts.value = false
  }
}

function formatTime(t) { if (!t) return '—'; return new Date(t).toLocaleString('zh-CN') }
function eventDeliveryStatus(evt) {
  if (evt?.delivery_status) return evt.delivery_status
  return evt?.notified ? 'delivered' : 'pending'
}
function deliveryStatusLabel(evt) {
  const labels = {
    pending: '待推送',
    delivered: '已推送',
    skipped: '已跳过',
    partial: '部分成功',
    failed: '推送失败',
  }
  return labels[eventDeliveryStatus(evt)] || eventDeliveryStatus(evt)
}
function formatInterval(ns) { if (!ns) return '—'; const s = Math.floor(ns / 1e9); if (s >= 3600) return `${Math.round(s / 3600)} 小时`; if (s >= 60) return `${Math.round(s / 60)} 分钟`; return `${s} 秒` }
function formatDuration(ns) { if (!ns) return '—'; const ms = Math.round(ns / 1e6); if (ms >= 1000) return `${(ms / 1000).toFixed(1)}s`; return `${ms}ms` }
function serviceLabel(s) {
  if (s === 'pushplus') return 'PushPlus'
  if (s === 'webhook') return 'Webhook'
  if (s === 'serverchan') return 'Server酱'
  return s
}

async function handleResetBaseline() {
  if (!confirm('确定要重新记录当前价格吗？这会将当前价格作为新的比较起点，但不会删除历史价格记录。')) return
  actionLoading.value = true
  try {
    await resetBaseline(route.params.name)
    showSuccess('基线已重置，下次检查将建立新基线')
    await loadData()
  } catch (e) {
    showError('重置失败: ' + (e.response?.data?.message || e.message))
  } finally {
    actionLoading.value = false
  }
}

async function loadSnapshots() {
  snapshotsLoading.value = true
  try {
    const res = await fetchSnapshots(route.params.name)
    if (res.code === 0 && res.data) {
      snapshots.value = Array.isArray(res.data) ? res.data : (res.data.snapshots || [])
    }
  } catch { /* ignore */ }
  finally { snapshotsLoading.value = false }
}

async function handleManualCheck() {
  actionLoading.value = true
  try {
    const res = await manualCheck(route.params.name)
    const outcome = res.data || {}
    if (outcome.is_first_baseline) {
      showSuccess(monitor.value?.strategy_type === 'field_transition'
        ? '检查完成，已建立新的价格基线，本次未发送通知'
        : '检查完成，已建立初始基线')
    } else if ((outcome.count || 0) > 0) {
      showSuccess(`检查完成，发现 ${outcome.count} 条变化`)
    } else {
      showSuccess('检查完成，未发现符合条件的变化')
    }
    await loadData()
  } catch (e) {
    showError('检查失败: ' + (e.response?.data?.message || e.message))
  } finally {
    actionLoading.value = false
  }
}
</script>

<style scoped>
.status-row { display: flex; align-items: center; gap: 0.75rem; margin-bottom: 0.75rem; }
.interval-badge { font-size: 0.75rem; color: var(--text-muted); background: var(--bg-elevated); padding: 0.15rem 0.5rem; border-radius: var(--radius-pill); }

.status-summary {
  padding: 1rem;
  background: var(--bg-surface);
  border-radius: var(--radius-lg);
  margin-bottom: 1rem;
}

.status-summary.status-error {
  background: var(--error-bg);
}

.status-description {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
}

.desc-error { font-size: 0.9375rem; font-weight: 700; color: var(--error); }
.desc-ok { font-size: 0.9375rem; font-weight: 700; color: var(--success); }
.desc-paused { font-size: 0.9375rem; font-weight: 700; color: var(--text-muted); }
.desc-detail { font-size: 0.8125rem; color: var(--text-secondary); }

.status-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 0.75rem; }
.status-item { display: flex; flex-direction: column; gap: 0.1rem; }
.status-label { font-size: 0.6875rem; font-weight: 700; color: var(--text-muted); }
.status-value { font-size: 0.875rem; color: var(--text); word-break: break-all; }
.error-text { color: var(--error); }
.status-actions { display: flex; gap: 0.5rem; margin-top: 1rem; }

/* 统一返回按钮 */
.back-btn {
  display: inline-flex; align-items: center; gap: 0.3rem;
  padding: 0.35rem 0.85rem; border-radius: var(--radius-pill);
  font-size: 0.8125rem; font-weight: 700; color: var(--text-secondary);
  background: var(--bg-elevated); text-decoration: none;
  transition: var(--transition);
}
.back-btn:hover { background: var(--bg-hover); color: var(--text); }

/* Page header */
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 1.5rem; }
.page-header h1 { font-size: 1.5rem; font-weight: 700; color: var(--text); margin-top: 0.5rem; }

/* Circular button */
.circle-btn {
  width: 44px; height: 44px; border: none; border-radius: var(--radius-circle);
  cursor: pointer; transition: var(--transition);
  display: flex; align-items: center; justify-content: center; padding: 0;
}
.circle-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.btn-play { background: var(--green); color: #000000; }
.btn-play:hover:not(:disabled) { background: var(--green-hover); transform: scale(1.08); }
.btn-pause { background: var(--bg-elevated); color: var(--text); }
.btn-pause:hover:not(:disabled) { background: var(--bg-hover); transform: scale(1.08); }

.header-actions { display: flex; gap: 0.5rem; align-items: center; }

/* 左右布局 */
.detail-panel { display: flex; gap: 0; }
.detail-left { flex: 1; min-width: 0; }
.detail-divider { width: 1px; background: var(--border-light); margin: 0 1.25rem; align-self: stretch; }
.detail-right { width: 240px; flex-shrink: 0; }

/* 推送账户列表 */
.accounts-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.75rem; }
.accounts-header h3 { font-size: 0.9375rem; font-weight: 700; color: var(--text); }
.link-sm { font-size: 0.75rem; color: var(--text-secondary); text-decoration: none; }
.link-sm:hover { color: var(--text); }
.accounts-list { display: flex; flex-direction: column; gap: 0.4rem; }
.account-checkbox {
  display: flex; align-items: center; gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  border-radius: var(--radius-lg);
  background: var(--bg-card);
  cursor: pointer;
  transition: var(--transition);
}
.account-checkbox:hover { background: var(--bg-hover); }
.account-checkbox input { accent-color: var(--green); }
.acc-name { font-size: 0.875rem; font-weight: 700; color: var(--text); }
.acc-service-badge {
  font-size: 0.625rem; font-weight: 700; color: #000000;
  background: var(--green); padding: 0.1rem 0.4rem; border-radius: var(--radius-pill);
}
.badge-bark { background: #d32f2f; color: #fff; }
.badge-pushplus { background: #1976d2; color: #fff; }
.badge-serverchan { background: var(--green); color: #000; }
.accounts-actions { margin-top: 0.75rem; display: flex; justify-content: flex-end; }
.hint { font-size: 0.75rem; color: var(--text-muted); margin-top: 0.5rem; }
.pagination { display: flex; align-items: center; justify-content: center; gap: 0.75rem; margin-top: 1rem; font-size: 0.8125rem; color: var(--text-muted); }

/* 快照表格 */
.snapshots-table { display: flex; flex-direction: column; gap: 0.2rem; }
.snapshot-row { display: flex; align-items: center; gap: 0.75rem; padding: 0.4rem 0.75rem; background: var(--bg-card); border-radius: var(--radius-lg); font-size: 0.8125rem; }
.snapshot-header { background: transparent; font-weight: 700; font-size: 0.6875rem; color: var(--text-muted); }
.snap-col { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.snap-key { flex: 2; }
.snapshot-identity { display: grid; gap: 0.1rem; }
.snapshot-identity strong, .snapshot-identity code, .snapshot-identity > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.snapshot-identity strong { color: var(--text); font-size: 0.8125rem; }
.snapshot-identity code { color: var(--text-muted); font-size: 0.6875rem; }
.snap-price { flex: 1; font-weight: 700; font-variant-numeric: tabular-nums; }
.snap-currency { flex: 0.5; color: var(--text-muted); }
.snap-valid { flex: 0.5; display: flex; align-items: center; gap: 0.3rem; }
.snap-time { flex: 1; color: var(--text-muted); font-size: 0.75rem; }
.valid-dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; }
.valid-yes { background: var(--green); }
.valid-no { background: var(--error); }

/* 事件表格 */
.events-table { display: flex; flex-direction: column; gap: 0.25rem; }
.event-row { display: flex; align-items: center; gap: 0.75rem; padding: 0.5rem 0.75rem; background: var(--bg-card); border-radius: var(--radius-lg); font-size: 0.8125rem; }
.event-type-badge { font-size: 0.625rem; font-weight: 700; padding: 0.15rem 0.4rem; border-radius: var(--radius-pill); flex-shrink: 0; }
.event-price_dropped { background: #ff4444; color: #fff; }
.event-price_target_reached { background: #7c4dff; color: #fff; }
.event-item_added { background: var(--green); color: #000; }
.event-info { flex: 1; display: flex; flex-direction: column; gap: 0.15rem; min-width: 0; }
.event-title { color: var(--text); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.event-price { display: flex; align-items: center; gap: 0.4rem; font-size: 0.75rem; }
.old-price { color: var(--text-muted); text-decoration: line-through; }
.price-arrow { color: var(--text-muted); }
.new-price { color: var(--text); font-weight: 700; }
.price-drop { color: #ff4444; font-weight: 700; }
.event-time { color: var(--text-muted); font-size: 0.75rem; flex-shrink: 0; }
.event-notified { font-size: 0.625rem; font-weight: 700; padding: 0.1rem 0.4rem; border-radius: var(--radius-pill); flex-shrink: 0; }
.event-notified.status-delivered { background: var(--green); color: #000; }
.event-notified.status-pending { background: var(--bg-elevated); color: var(--text-muted); }
.event-notified.status-skipped { background: var(--bg-elevated); color: var(--text-secondary); }
.event-notified.status-partial { background: #ffb020; color: #000; }
.event-notified.status-failed { background: var(--error); color: #fff; }

@media (max-width: 768px) {
  .status-grid { grid-template-columns: 1fr; }
  .page-header { flex-direction: column; gap: 0.75rem; }
  .detail-panel { flex-direction: column; }
  .detail-divider { width: 100%; height: 1px; margin: 1rem 0; }
  .detail-right { width: 100%; }
}
</style>
