import { client, publicClient, type ApiEnvelope } from './client'
import type {
  AIExtractResult,
  ImportResult,
  LLMConnectionTestResult,
  LLMSettings,
  ManualCheckOutcome,
  Monitor,
  NotificationProviderMeta,
  NotificationSettings,
  NotifyAccount,
  PagedUpdates,
  QuickScanRulePayload,
  ScanPreviewResult,
  ScanRule,
  Stats,
  UpdateCheckResult,
  UpdateProxyInfo,
  UpdateScanRulePayload,
  UpdateStatus,
  ValidationResult,
  VersionInfo,
} from './types'
import type { FetchConfig, ExtractionField, MonitorRequest, MonitorConfigResponse } from '@/lib/monitorForm'

// ===== 监控器管理 =====

// 获取所有监控器
export function fetchMonitors(): Promise<ApiEnvelope<Monitor[]>> {
  return client.get('/monitors/').then((r) => r.data)
}

// 获取单个监控器
export function fetchMonitor(name: string): Promise<ApiEnvelope<Monitor>> {
  return client.get(`/monitors/${encodeURIComponent(name)}`).then((r) => r.data)
}

// 新增监控器
export function createMonitor(config: MonitorRequest): Promise<ApiEnvelope<unknown>> {
  return client.post('/monitors/', config).then((r) => r.data)
}

// 更新监控器
export function updateMonitor(name: string, config: MonitorRequest): Promise<ApiEnvelope<unknown>> {
  return client.put(`/monitors/${encodeURIComponent(name)}`, config).then((r) => r.data)
}

// 删除监控器
export function deleteMonitor(name: string): Promise<ApiEnvelope<unknown>> {
  return client.delete(`/monitors/${encodeURIComponent(name)}`).then((r) => r.data)
}

// 启动监控器
export function startMonitor(name: string): Promise<ApiEnvelope<unknown>> {
  return client.post(`/monitors/${encodeURIComponent(name)}/start`).then((r) => r.data)
}

// 停止监控器
export function stopMonitor(name: string): Promise<ApiEnvelope<unknown>> {
  return client.post(`/monitors/${encodeURIComponent(name)}/stop`).then((r) => r.data)
}

// 获取更新历史
export function fetchUpdates(name: string, params: { page?: number; size?: number } = {}): Promise<ApiEnvelope<PagedUpdates>> {
  return client.get(`/monitors/${encodeURIComponent(name)}/updates`, { params }).then((r) => r.data)
}

// 获取监控器完整配置（用于编辑模式）
export function fetchMonitorConfig(name: string): Promise<ApiEnvelope<MonitorConfigResponse>> {
  return client.get(`/monitors/${encodeURIComponent(name)}/config`).then((r) => r.data)
}

// 一键标注所有未推送为已推送
export function markAllNotified(name: string): Promise<ApiEnvelope<{ updated?: number }>> {
  return client.put(`/monitors/${encodeURIComponent(name)}/mark-all-notified`).then((r) => r.data)
}

// 标记监控器已读（未读计数归零）
export function markRead(name: string): Promise<ApiEnvelope<unknown>> {
  return client.post(`/monitors/${encodeURIComponent(name)}/mark-read`).then((r) => r.data)
}

// 更新监控器的推送账户
export function updateNotifyAccounts(name: string, accountIDs: Array<number | string>): Promise<ApiEnvelope<unknown>> {
  return client
    .put(`/monitors/${encodeURIComponent(name)}/notify-accounts`, { notify_account_ids: accountIDs || [] })
    .then((r) => r.data)
}

// ===== 智能扫描 =====

// 规则扫描：按已保存的扫描规则识别网页内容
export function previewScan(params: { url: string }): Promise<ApiEnvelope<ScanPreviewResult>> {
  return client.post('/monitors/preview', params).then((r) => r.data)
}

// 验证监控配置
export function validateMonitorConfig(config: unknown): Promise<ApiEnvelope<ValidationResult>> {
  return client.post('/monitors/validate', config).then((r) => r.data)
}

// ===== 系统接口 =====

// 获取统计数据
export function fetchStats(): Promise<ApiEnvelope<Stats>> {
  return client.get('/stats').then((r) => r.data)
}

// 获取通知设置（仅开关）
export function fetchNotificationSettings(): Promise<ApiEnvelope<NotificationSettings>> {
  return client.get('/settings/notifications').then((r) => r.data)
}

// 更新通知开关
export function updateNotificationSettings(settings: NotificationSettings): Promise<ApiEnvelope<unknown>> {
  return client.put('/settings/notifications', settings).then((r) => r.data)
}

// 获取推送服务供应商元数据
export function fetchNotificationProviders(): Promise<ApiEnvelope<Record<string, NotificationProviderMeta>>> {
  return client.get('/settings/notification-providers').then((r) => r.data)
}

// ===== 推送账户 CRUD =====

// 获取所有推送账户
export function fetchAccounts(): Promise<ApiEnvelope<NotifyAccount[]>> {
  return client.get('/settings/notification-accounts').then((r) => r.data)
}

// 创建推送账户
export function createAccount(data: { name: string; service: string; config: Record<string, unknown> }): Promise<ApiEnvelope<unknown>> {
  return client.post('/settings/notification-accounts', data).then((r) => r.data)
}

// 更新推送账户
export function updateAccount(id: number, data: { name: string; service: string; config: Record<string, unknown> }): Promise<ApiEnvelope<unknown>> {
  return client.put(`/settings/notification-accounts/${id}`, data).then((r) => r.data)
}

// 删除推送账户
export function deleteAccount(id: number): Promise<ApiEnvelope<unknown>> {
  return client.delete(`/settings/notification-accounts/${id}`).then((r) => r.data)
}

// ===== 扫描规则模板 CRUD =====

// 获取扫描规则模板
export function fetchScanRules(): Promise<ApiEnvelope<ScanRule[]>> {
  return client.get('/settings/scan-rules').then((r) => r.data)
}

// 从预扫描候选快速创建规则
export function quickCreateScanRule(data: QuickScanRulePayload): Promise<ApiEnvelope<unknown>> {
  return client.post('/settings/scan-rules/quick', data).then((r) => r.data)
}

// 导出/导入规则库
export function exportScanRules(): Promise<ApiEnvelope<unknown>> {
  return client.get('/settings/scan-rules/export').then((r) => r.data)
}

export function importScanRules(data: unknown): Promise<ApiEnvelope<ImportResult>> {
  return client.post('/settings/scan-rules/import', data).then((r) => r.data)
}

// 更新扫描规则模板
export function updateScanRule(id: number, data: UpdateScanRulePayload): Promise<ApiEnvelope<unknown>> {
  return client.put(`/settings/scan-rules/${id}`, data).then((r) => r.data)
}

// 删除扫描规则模板
export function deleteScanRule(id: number): Promise<ApiEnvelope<unknown>> {
  return client.delete(`/settings/scan-rules/${id}`).then((r) => r.data)
}

// ===== 手动检查 =====

// 手动触发检查
export function manualCheck(name: string): Promise<ApiEnvelope<ManualCheckOutcome>> {
  return client.post(`/monitors/${encodeURIComponent(name)}/check`).then((r) => r.data)
}

// ===== AI 模型接入 =====

// 获取 AI 模型接入配置（api_key 脱敏）
export function fetchLLMSettings(): Promise<ApiEnvelope<LLMSettings>> {
  return client.get('/settings/llm').then((r) => r.data)
}

// 保存 AI 模型接入配置
export function updateLLMSettings(settings: LLMSettings): Promise<ApiEnvelope<unknown>> {
  return client.put('/settings/llm', settings).then((r) => r.data)
}

// 测试 AI 模型连通性
export function testLLMConnection(): Promise<ApiEnvelope<LLMConnectionTestResult>> {
  return client.post('/settings/llm/test').then((r) => r.data)
}

// AI 提取扫描规则：按关键词辅助定位内容区域
export function aiExtractScanRule(params: { url: string; keywords?: string }): Promise<ApiEnvelope<AIExtractResult>> {
  return client.post('/settings/scan-rules/ai-extract', params, { timeout: 180000 }).then((r) => r.data)
}

// ===== 更新接口 =====

export function fetchVersion(): Promise<ApiEnvelope<VersionInfo>> {
  return publicClient.get('/version').then((r) => r.data)
}

export function checkUpdate(): Promise<ApiEnvelope<UpdateCheckResult>> {
  return publicClient.get('/update/check', { timeout: 25000 }).then((r) => r.data)
}

export function applyUpdate(): Promise<ApiEnvelope<unknown>> {
  return client.post('/update/apply').then((r) => r.data)
}

export function fetchUpdateStatus(): Promise<ApiEnvelope<UpdateStatus>> {
  return client.get('/update/status').then((r) => r.data)
}

export function fetchUpdateProxy(): Promise<ApiEnvelope<UpdateProxyInfo>> {
  return client.get('/update/proxy').then((r) => r.data)
}

export function setUpdateProxy(proxy: string): Promise<ApiEnvelope<unknown>> {
  return client.put('/update/proxy', { proxy }).then((r) => r.data)
}

// 重新导出便于上层使用
export type { ExtractionField, FetchConfig, MonitorRequest, MonitorConfigResponse }
