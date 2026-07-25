import axios from 'axios'

function applyAuth(config) {
  const token = localStorage.getItem('alterbot_auth_token') || ''
  if (token) {
    config.headers = config.headers || {}
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
}

const client = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
})
client.interceptors.request.use(applyAuth)

const rootClient = axios.create({
  baseURL: '/api',
  timeout: 10000,
})
rootClient.interceptors.request.use(applyAuth)

export function setAuthToken(token) {
  if (token) localStorage.setItem('alterbot_auth_token', token)
  else localStorage.removeItem('alterbot_auth_token')
}

export function getAuthToken() {
  return localStorage.getItem('alterbot_auth_token') || ''
}

// 获取所有监控器
export function fetchMonitors() {
  return client.get('/monitors/').then(r => r.data)
}

// 获取所有分组
export function fetchGroups() {
  return rootClient.get('/groups').then(r => r.data)
}

// 获取单个监控器
export function fetchMonitor(name) {
  return client.get(`/monitors/${encodeURIComponent(name)}`).then(r => r.data)
}

// 新增监控器
export function createMonitor(config) {
  return client.post('/monitors/', config).then(r => r.data)
}

// 更新监控器
export function updateMonitor(name, config) {
  return client.put(`/monitors/${encodeURIComponent(name)}`, config).then(r => r.data)
}

// 删除监控器
export function deleteMonitor(name) {
  return client.delete(`/monitors/${encodeURIComponent(name)}`).then(r => r.data)
}

// 启动监控器
export function startMonitor(name) {
  return client.post(`/monitors/${encodeURIComponent(name)}/start`).then(r => r.data)
}

// 停止监控器
export function stopMonitor(name) {
  return client.post(`/monitors/${encodeURIComponent(name)}/stop`).then(r => r.data)
}

// 获取更新历史
export function fetchUpdates(name, params = {}) {
  return client.get(`/monitors/${encodeURIComponent(name)}/updates`, { params }).then(r => r.data)
}

// 获取监控器完整配置（用于编辑模式）
export function fetchMonitorConfig(name) {
  return client.get(`/monitors/${encodeURIComponent(name)}/config`).then(r => r.data)
}

// 一键标注所有未推送为已推送
export function markAllNotified(name) {
  return client.put(`/monitors/${encodeURIComponent(name)}/mark-all-notified`).then(r => r.data)
}

// 标记监控器已读（未读计数归零）
export function markRead(name) {
  return client.post(`/monitors/${encodeURIComponent(name)}/mark-read`).then(r => r.data)
}

// 更新监控器的推送账户
export function updateNotifyAccounts(name, accountIDs) {
  return client.put(`/monitors/${encodeURIComponent(name)}/notify-accounts`, { notify_account_ids: accountIDs || [] }).then(r => r.data)
}

// 智能扫描：预览网页内容
export function previewScan(params) {
  return client.post('/monitors/preview', params).then(r => r.data)
}

// 智能创建：确认并创建监控器
export function smartCreate(params) {
  return client.post('/monitors/smart-create', params).then(r => r.data)
}

// 健康检查
export function healthCheck() {
  return rootClient.get('/health').then(r => r.data)
}

// 获取统计数据
export function fetchStats() {
  return rootClient.get('/stats').then(r => r.data)
}

// 获取通知设置（仅开关）
export function fetchNotificationSettings() {
  return rootClient.get('/settings/notifications').then(r => r.data)
}

// 获取推送服务供应商元数据
export function fetchNotificationProviders() {
  return rootClient.get('/settings/notification-providers').then(r => r.data)
}

// 更新通知开关
export function updateNotificationSettings(settings) {
  return rootClient.put('/settings/notifications', settings).then(r => r.data)
}

// ===== 推送账户 CRUD =====

// 获取所有推送账户
export function fetchAccounts() {
  return rootClient.get('/settings/notification-accounts').then(r => r.data)
}

// 创建推送账户
export function createAccount(data) {
  return rootClient.post('/settings/notification-accounts', data).then(r => r.data)
}

// 更新推送账户
export function updateAccount(id, data) {
  return rootClient.put(`/settings/notification-accounts/${id}`, data).then(r => r.data)
}

// 删除推送账户
export function deleteAccount(id) {
  return rootClient.delete(`/settings/notification-accounts/${id}`).then(r => r.data)
}

// ===== 扫描规则模板 CRUD =====

// 获取扫描规则模板
export function fetchScanRules() {
  return rootClient.get('/settings/scan-rules').then(r => r.data)
}

// 创建扫描规则模板
export function createScanRule(data) {
  return rootClient.post('/settings/scan-rules', data).then(r => r.data)
}

// 从预扫描候选快速创建规则
export function quickCreateScanRule(data) {
  return rootClient.post('/settings/scan-rules/quick', data).then(r => r.data)
}

// 导出/导入规则库
export function exportScanRules() {
  return rootClient.get('/settings/scan-rules/export').then(r => r.data)
}

export function importScanRules(data) {
  return rootClient.post('/settings/scan-rules/import', data).then(r => r.data)
}

// 更新扫描规则模板
export function updateScanRule(id, data) {
  return rootClient.put(`/settings/scan-rules/${id}`, data).then(r => r.data)
}

// 删除扫描规则模板
export function deleteScanRule(id) {
  return rootClient.delete(`/settings/scan-rules/${id}`).then(r => r.data)
}

// 测试指定扫描规则
export function testScanRule(id, data) {
  return rootClient.post(`/settings/scan-rules/${id}/test`, data).then(r => r.data)
}

// ===== 新引擎 API =====

// 获取事件历史
export function fetchEvents(name, params = {}) {
  return client.get(`/monitors/${encodeURIComponent(name)}/events`, { params }).then(r => r.data)
}

// 获取当前快照
export function fetchSnapshots(name) {
  return client.get(`/monitors/${encodeURIComponent(name)}/snapshots`).then(r => r.data)
}

// 重置基线
export function resetBaseline(name) {
  return client.post(`/monitors/${encodeURIComponent(name)}/baseline`).then(r => r.data)
}

// 手动触发检查
export function manualCheck(name) {
  return client.post(`/monitors/${encodeURIComponent(name)}/check`).then(r => r.data)
}

// 验证监控配置
export function validateMonitorConfig(config) {
  return client.post('/monitors/validate', config).then(r => r.data)
}

// ===== 更新接口 =====

export function fetchVersion() {
  return rootClient.get('/version').then(r => r.data)
}

export function checkUpdate() {
  return rootClient.get('/update/check', { timeout: 25000 }).then(r => r.data)
}

export function applyUpdate() {
  return rootClient.post('/update/apply').then(r => r.data)
}

export function fetchUpdateStatus() {
  return rootClient.get('/update/status').then(r => r.data)
}

export function fetchUpdateProxy() {
  return rootClient.get('/update/proxy').then(r => r.data)
}

export function setUpdateProxy(proxy) {
  return rootClient.put('/update/proxy', { proxy }).then(r => r.data)
}
