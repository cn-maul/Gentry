import type { ExtractionField, FetchConfig } from '@/lib/monitorForm'

/** 监控器（列表/详情） */
export interface Monitor {
  name: string
  url: string
  group?: string
  is_running?: boolean
  status?: string
  container?: string
  item?: string
  last_check?: string
  next_check?: string
  last_error?: string
  last_duration?: number
  check_interval?: number
  updates_count?: number
  baseline_status?: string
}

/** 变更记录 */
export interface UpdateRecord {
  ID: number
  CreatedAt: string
  URL: string
  Title: string
  Notified: boolean
}

export interface PagedUpdates {
  records: UpdateRecord[]
  total: number
}

/** 系统统计 */
export interface Stats {
  total_monitors: number
  running_monitors: number
  total_updates: number
  updates_last_hour: number
  unnotified_updates: number
  pushed_today: number
  total_accounts: number
}

/** 推送账户 */
export interface NotifyAccount {
  id: number
  name: string
  service: string
  config?: Record<string, unknown>
  enabled?: boolean
}

/** 推送服务供应商元数据 */
export interface NotificationProviderMeta {
  label: string
  required_fields?: string[]
}

/** 通知设置 */
export interface NotificationSettings {
  enabled: boolean
}

/** 规则扫描候选 */
export interface ScanContainer {
  container_css?: string
  item_css?: string
  item_count: number
  strategy?: string
  rule_name?: string
  rule_address?: string
  diagnostics?: string[]
  sample_items?: Array<{
    title?: string
    date?: string
    url?: string
  }>
  config?: {
    container?: string
    item?: string
    fields?: ExtractionField[]
    fetch_config?: FetchConfig
  }
}

export interface ScanPreviewResult {
  containers?: ScanContainer[]
}

/** 监控器分类 */
export interface Category {
  id: number
  name: string
}

/** 配置验证 */
export interface ValidationSample {
  item_key: string
  raw?: string
}

export interface ValidationItem {
  status: 'ok' | 'warn' | 'err'
  label: string
  detail: string
  samples?: ValidationSample[]
}

export interface ValidationResult {
  valid: boolean
  items?: ValidationItem[]
  errors?: string[]
  summary?: string
  extracted_items?: number
}

/** 扫描规则 */
export interface ScanRule {
  id: number
  name: string
  source_url?: string
  url_contains?: string
  scope_type?: string
  container?: string
  item?: string
  fields?: ExtractionField[]
  fetch_config?: string | FetchConfig
  priority?: number
  enabled?: boolean
  description?: string
}

export interface QuickScanRulePayload {
  name: string
  url: string
  scope_type: string
  config?: {
    container?: string
    item?: string
    fetch_config?: FetchConfig
    fields?: ExtractionField[]
  }
}

export interface UpdateScanRulePayload {
  name: string
  url_contains: string
  source_url: string
  scope_type: string
  container: string
  item: string
  priority: number
  enabled: boolean
  description: string
  fetch_config?: FetchConfig
  fields: ExtractionField[]
}

export interface ImportResult {
  imported?: number
  skipped?: number
}

/** 版本与自更新 */
export interface VersionInfo {
  version: string
}

export interface UpdateCheckResult {
  has_update: boolean
  latest_version: string
  download_url: string
}

export interface UpdateStatus {
  status: string
  message?: string
}

export interface UpdateProxyInfo {
  proxy?: string
}

/** 手动检查结果 */
export interface ManualCheckOutcome {
  is_first_baseline?: boolean
  count?: number
}

/** AI 模型接入配置（api_key 为脱敏回显） */
export interface LLMSettings {
  base_url: string
  api_key: string
  model: string
  configured?: boolean
}

export interface LLMConnectionTestResult {
  ok: boolean
  model?: string
  message?: string
  answer?: string
}

/** 捕获管线诊断信息：记录尝试次数、失败原因与关键词命中 */
export interface CaptureDiagnostics {
  attempts: number
  failures?: string[]
  keyword_hits: number
  item_count: number
}

/** 规则捕获结果（统一捕获管线产物，草稿需人工确认后保存） */
export interface CaptureResult {
  config: {
    container?: string
    item?: string
    fields?: ExtractionField[]
  }
  samples?: Array<{
    title?: string
    url?: string
    date?: string
  }>
  verified: boolean
  message: string
  diagnostics?: CaptureDiagnostics
}
