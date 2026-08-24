<template>
  <div class="push-page">
    <div class="page-header">
      <div>
        <h1>推送管理</h1>
        <p class="page-desc">管理推送账户，每个账户可对应一个人/渠道</p>
      </div>
      <div class="header-actions">
        <button class="btn btn-primary" @click="openCreate">新增账户</button>
      </div>
    </div>

    <div class="toast toast-success" v-if="successMsg">{{ successMsg }}</div>
    <div class="toast toast-warning" v-if="pageErrorMsg">{{ pageErrorMsg }}</div>

    <div class="loading" v-if="loading">
      <div class="spinner" />
      <p>加载中...</p>
    </div>

    <template v-else-if="accounts.length === 0">
      <div class="empty">
        <div class="empty-icon">🔔</div>
        <p>还没有推送账户</p>
        <p style="color: var(--text-muted); font-size: 0.8125rem; margin-top: 0.25rem;">
          创建账户后，可在每个监控器中独立选择启用哪些账户
        </p>
        <button class="btn btn-primary btn-sm" style="margin-top: 1rem;" @click="openCreate">新增账户</button>
      </div>
    </template>

    <template v-else>
      <div class="settings-section">
        <div class="section-header">
          <h2>全局推送开关</h2>
        </div>
        <div class="setting-item">
          <div class="setting-info">
            <div class="setting-label">启用推送</div>
            <div class="setting-desc">关闭后所有监控器都不会发送推送</div>
          </div>
          <div class="setting-control">
            <label class="toggle">
              <input type="checkbox" v-model="globalEnabled" @change="saveGlobalEnabled" />
              <span class="toggle-track"></span>
            </label>
          </div>
        </div>
      </div>

      <div class="settings-section">
        <div class="section-header">
          <h2>推送账户（{{ accounts.length }}）</h2>
          <p class="section-desc">每个账户独立配置，可在监控器详情中选用</p>
        </div>

        <div class="account-card" v-for="acc in accounts" :key="acc.id">
          <div class="account-header">
            <div class="account-info">
              <span class="account-name">{{ acc.name }}</span>
              <span class="account-service-badge" :class="'badge-' + acc.service">{{ serviceLabel(acc.service) }}</span>
            </div>
            <div class="account-actions">
              <button class="icon-btn" title="编辑" @click="openEdit(acc)">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
              </button>
              <button class="icon-btn icon-btn-danger" title="删除" @click="confirmDelete(acc)">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
              </button>
            </div>
          </div>
          <div class="account-status">
            <span class="status-dot-ok" />
            <span class="status-text">配置正常</span>
          </div>
        </div>
      </div>
    </template>

    <!-- Create/Edit Modal -->
    <div class="modal-overlay" v-if="showModal" @click.self="showModal = false">
      <div class="modal-container">
        <div class="modal-header">
          <h2>{{ editingAccount ? '编辑账户' : '新增账户' }}</h2>
          <button class="modal-close" @click="showModal = false">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label>账户名称</label>
            <input v-model="form.name" class="form-input" placeholder="如 张三、运维群" />
          </div>
          <div class="form-group">
            <label>推送服务</label>
            <select v-model="form.service" class="form-input">
              <option v-for="provider in providerOptions" :key="provider.service" :value="provider.service">{{ provider.label }}</option>
            </select>
          </div>

          <template v-if="form.service === 'pushplus'">
            <div class="form-group">
              <label>Token</label>
              <input v-model="form.config.token" class="form-input" placeholder="PushPlus 用户令牌" />
            </div>
            <div class="form-group">
              <label>Channel（可选）</label>
              <input v-model="form.config.channel" class="form-input" placeholder="mail / wechat / sms" />
            </div>
            <div class="form-group">
              <label>Template（可选）</label>
              <select v-model="form.config.template" class="form-input">
                <option value="">默认 (html)</option>
                <option value="html">html</option>
                <option value="txt">纯文本 (txt)</option>
                <option value="json">json</option>
                <option value="markdown">markdown</option>
              </select>
            </div>
          </template>

          <template v-if="form.service === 'webhook'">
            <div class="form-group">
              <label>Webhook URL</label>
              <input v-model="form.config.url" class="form-input" placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx" />
            </div>
          </template>

          <template v-if="form.service === 'serverchan'">
            <div class="form-group">
              <label>SendKey</label>
              <input v-model="form.config.sendkey" class="form-input" placeholder="Server酱 SendKey" />
            </div>
            <div class="form-group">
              <label>Channel（可选）</label>
              <select v-model="form.config.channel" class="form-input">
                <option value="">默认通道</option>
                <option value="9">方糖服务号</option>
                <option value="66">企业微信应用消息</option>
                <option value="1">企业微信群机器人</option>
                <option value="2">钉钉群机器人</option>
                <option value="3">飞书群机器人</option>
                <option value="8">Bark iOS</option>
                <option value="18">PushDeer</option>
                <option value="88">自定义</option>
              </select>
            </div>
          </template>

          <template v-if="form.service === 'bark'">
            <div class="form-group">
              <label>Key</label>
              <input v-model="form.config.key" class="form-input" placeholder="Bark 设备 Key" />
            </div>
            <div class="form-group">
              <label>Server（可选）</label>
              <input v-model="form.config.server" class="form-input" placeholder="默认 https://api.day.app" />
            </div>
            <div class="form-group">
              <label>Group（可选）</label>
              <input v-model="form.config.group" class="form-input" placeholder="通知分组名称" />
            </div>
            <div class="form-group">
              <label>Sound（可选）</label>
              <input v-model="form.config.sound" class="form-input" placeholder="自定义铃声名称" />
            </div>
          </template>

          <div class="form-error" v-if="modalError">{{ modalError }}</div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="showModal = false">取消</button>
          <button class="btn btn-primary" :disabled="modalSaving" @click="handleSaveAccount">{{ modalSaving ? '保存中...' : '保存' }}</button>
        </div>
      </div>
    </div>

    <!-- Delete Confirm -->
    <div class="modal-overlay" v-if="deleteTarget" @click.self="deleteTarget = null">
      <div class="modal-container" style="max-width: 400px;">
        <div class="modal-header">
          <h2>确认删除</h2>
          <button class="modal-close" @click="deleteTarget = null">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </div>
        <div class="modal-body">
          <p>确定删除推送账户「{{ deleteTarget.name }}」吗？</p>
          <p style="margin-top: 0.5rem;">已选择此账户的监控器将不再收到推送。</p>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="deleteTarget = null">取消</button>
          <button class="btn btn-danger" @click="handleDeleteAccount">确认删除</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { fetchAccounts, createAccount, updateAccount, deleteAccount, fetchNotificationSettings, updateNotificationSettings, fetchNotificationProviders } from '../api/monitors'
import { useToastMessages } from '../composables/useToastMessages'

const { successMsg, pageErrorMsg, showSuccess, showError } = useToastMessages()

const loading = ref(true)
const accounts = ref([])
const globalEnabled = ref(false)
const showModal = ref(false)
const editingAccount = ref(null)
const deleteTarget = ref(null)
const modalSaving = ref(false)
const modalError = ref('')

const providers = ref({})
const form = ref({
  name: '',
  service: 'pushplus',
  config: { token: '', channel: 'mail', template: '', url: '', sendkey: '', key: '', server: '', group: '', sound: '' },
})

function resetForm() {
  form.value = { name: '', service: 'pushplus', config: { token: '', channel: 'mail', template: '', url: '', sendkey: '', key: '', server: '', group: '', sound: '' } }
}

function serviceLabel(s) {
  return providers.value[s]?.label || s
}

const providerOptions = computed(() => {
  const entries = Object.entries(providers.value)
  if (entries.length === 0) {
    return [
      { service: 'pushplus', label: 'PushPlus' },
      { service: 'webhook', label: 'Webhook' },
      { service: 'serverchan', label: 'Server酱' },
      { service: 'bark', label: 'Bark' },
    ]
  }
  return entries.map(([service, meta]) => ({ service, label: meta.label || service }))
})

function getConfig(acc) {
  return acc.config || {}
}

function maskToken(t) {
  if (!t || t.length < 6) return t
  return t.slice(0, 3) + '****' + t.slice(-3)
}

onMounted(loadAll)

async function loadAll() {
  loading.value = true
  try {
    const [accts, settings, providerRes] = await Promise.all([
      fetchAccounts(),
      fetchNotificationSettings(),
      fetchNotificationProviders().catch(() => ({ data: {} })),
    ])
    if (accts.code === 0) accounts.value = accts.data || []
    if (settings.code === 0) globalEnabled.value = settings.data?.enabled || false
    if (providerRes.code === 0) providers.value = providerRes.data || {}
  } catch (e) {
    showError('加载失败: ' + e.message)
  } finally {
    loading.value = false
  }
}

async function saveGlobalEnabled() {
  try {
    await updateNotificationSettings({ enabled: globalEnabled.value })
    showSuccess(globalEnabled.value ? '推送已开启' : '推送已关闭')
  } catch (e) {
    showError('操作失败: ' + (e.response?.data?.message || e.message))
    globalEnabled.value = !globalEnabled.value
  }
}

function openCreate() {
  editingAccount.value = null
  resetForm()
  modalError.value = ''
  showModal.value = true
}

function openEdit(acc) {
  editingAccount.value = acc
  const cfg = getConfig(acc)
  const defaultChannel = acc.service === 'pushplus' ? 'mail' : ''
  form.value = {
    name: acc.name,
    service: acc.service,
    config: { token: cfg.token || '', channel: cfg.channel || defaultChannel, template: cfg.template || '', url: cfg.url || '', sendkey: cfg.sendkey || '', key: cfg.key || '', server: cfg.server || '', group: cfg.group || '', sound: cfg.sound || '' },
  }
  modalError.value = ''
  showModal.value = true
}

function confirmDelete(acc) {
  deleteTarget.value = acc
}

async function handleDeleteAccount() {
  const acc = deleteTarget.value
  deleteTarget.value = null
  try {
    await deleteAccount(acc.id)
    accounts.value = accounts.value.filter(a => a.id !== acc.id)
    showSuccess(`「${acc.name}」已删除`)
  } catch (e) {
    showError('删除失败: ' + (e.response?.data?.message || e.message))
  }
}

function requiredFieldsFor(service) {
  return providers.value[service]?.required_fields || []
}

async function handleSaveAccount() {
  if (!form.value.name.trim()) { modalError.value = '请输入账户名称'; return }
  for (const field of requiredFieldsFor(form.value.service)) {
    if (!String(form.value.config[field] || '').trim()) {
      modalError.value = `请输入 ${field}`
      return
    }
  }

  modalError.value = ''
  modalSaving.value = true
  try {
    const payload = {
      name: form.value.name.trim(),
      service: form.value.service,
      config: form.value.config,
    }
    if (editingAccount.value) {
      await updateAccount(editingAccount.value.id, payload)
      showSuccess('账户已更新')
    } else {
      await createAccount(payload)
      showSuccess('账户已创建')
    }
    showModal.value = false
    await loadAll()
  } catch (e) {
    modalError.value = (e.response?.data?.message || e.message)
  } finally {
    modalSaving.value = false
  }
}
</script>

<style scoped>
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 1.5rem; }
.page-header h1 { font-size: 1.5rem; font-weight: 700; color: var(--text); margin-top: 0.5rem; }

.account-card {
  background: var(--bg-card);
  border-radius: var(--radius-lg);
  padding: 1rem;
  margin-bottom: 0.5rem;
}

.account-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.account-info {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.account-name {
  font-weight: 700;
  font-size: 0.9375rem;
  color: var(--text);
}

.account-service-badge {
  font-size: 0.6875rem;
  font-weight: 700;
  color: var(--text);
  background: var(--green);
  padding: 0.15rem 0.5rem;
  border-radius: var(--radius-pill);
}

.badge-bark { background: #d32f2f; color: #fff; }
.badge-pushplus { background: #1976d2; color: #fff; }
.badge-serverchan { background: var(--green); color: #000; }

.account-actions {
  display: flex;
  gap: 0.25rem;
}

.account-config {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  margin-top: 0.5rem;
}

.account-status {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  margin-top: 0.5rem;
}

.status-dot-ok {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--green);
  flex-shrink: 0;
}

.status-text {
  font-size: 0.75rem;
  color: var(--text-muted);
}

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
}
.icon-btn:hover { background: var(--bg-active); color: var(--text); }
.icon-btn-danger:hover { color: var(--error); }
</style>
