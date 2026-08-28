import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router'
import type { ReactNode } from 'react'
import type { MonitorFormState } from '@/lib/monitorForm'
import type { NotifyAccount, ScanContainer, ValidationResult } from '@/api/types'
import { previewScan } from '@/api/monitors'
import BasicMonitorForm from './BasicMonitorForm'
import RuleMatchPanel from './RuleMatchPanel'
import NotificationEditor from './NotificationEditor'
import MonitorValidationPanel from './MonitorValidationPanel'
import MonitorFormSummary from './MonitorFormSummary'

interface MonitorFormProps {
  form: MonitorFormState
  accounts?: NotifyAccount[]
  /** 可选分类（来自设置 → 分类管理），高级设置中以下拉选择 */
  groups?: string[]
  error?: string | null
  validationResult?: ValidationResult | null
  validationLoading?: boolean
  showBaselineWarning?: boolean
  actions?: ReactNode
  onUpdateForm: (form: MonitorFormState) => void
  onValidate: () => void
}

export default function MonitorForm({
  form,
  accounts = [],
  groups = [],
  error,
  validationResult,
  validationLoading = false,
  showBaselineWarning = false,
  actions,
  onUpdateForm,
  onValidate,
}: MonitorFormProps) {
  const hasNotification = form.notification.accountIds.length > 0

  // ===== 匹配规则状态 =====
  const [matching, setMatching] = useState(false)
  const [matchResult, setMatchResult] = useState<{ containers?: ScanContainer[] } | null>(null)
  const [matched, setMatched] = useState(false)
  const [matchError, setMatchError] = useState<string | null>(null)
  const [appliedIndex, setAppliedIndex] = useState<number | null>(null)

  // 使用 ref 取最新表单，避免覆盖匹配期间的用户编辑
  const formRef = useRef(form)
  formRef.current = form

  // URL 变化时只重置匹配状态，不清空已加载的提取配置（编辑模式保留原配置）
  const lastUrl = useRef(form.basic.url)
  useEffect(() => {
    if (lastUrl.current !== form.basic.url) {
      lastUrl.current = form.basic.url
      setMatchResult(null)
      setMatched(false)
      setMatchError(null)
      setAppliedIndex(null)
    }
  }, [form.basic.url])

  async function handleMatch() {
    if (matching || !form.basic.url.trim()) return
    setMatching(true)
    setMatched(false)
    setMatchError(null)
    setAppliedIndex(null)
    try {
      const res = await previewScan({ url: form.basic.url.trim() })
      if (res.code === 0 && res.data) {
        setMatchResult(res.data)
        setMatched(true)
      } else {
        setMatchError(res.message || '匹配失败')
      }
    } catch (e) {
      setMatchError(e instanceof Error ? e.message : '匹配失败')
    }
    setMatching(false)
  }

  function applyCandidate(container: ScanContainer, index: number) {
    const current = formRef.current
    const config = container.config || {}
    const fields = (config.fields || []).map((f) => ({
      name: f.name || '',
      selector: f.selector || '',
      type: f.type || 'text',
      attr: f.attr || '',
      transform: f.transform || '',
    }))
    const extracted: MonitorFormState['extraction'] = {
      ...current.extraction,
      containerSelector: config.container || container.container_css || '',
      itemSelector: config.item || container.item_css || '',
    }
    if (config.fetch_config) {
      extracted.sourceMode = (config.fetch_config.mode as MonitorFormState['extraction']['sourceMode']) || 'html'
      extracted.sourceUrl = config.fetch_config.url || ''
      extracted.itemsPath = config.fetch_config.items_path || ''
      extracted.filterPath = config.fetch_config.filter_path || ''
      extracted.filterEquals = config.fetch_config.filter_equals ?? ''
      extracted.sourceHeaders = config.fetch_config.headers || {}
    } else {
      extracted.sourceMode = 'html'
      extracted.sourceUrl = ''
      extracted.itemsPath = ''
      extracted.filterPath = ''
      extracted.filterEquals = ''
      extracted.sourceHeaders = {}
    }
    if (fields.length > 0) extracted.fields = fields
    onUpdateForm({ ...current, extraction: extracted })
    setAppliedIndex(index)
  }

  let currentStep = 1
  if (form.basic.url) {
    currentStep = form.extraction.containerSelector ? 3 : 2
  }

  return (
    <div className="monitor-form">
      <div className="step-indicator">
        <div className={`step${currentStep === 1 ? ' active' : ''}${currentStep > 1 ? ' done' : ''}`}>
          <span className="step-num">1</span>
          <span className="step-text">填写网址</span>
        </div>
        <div className={`step-line${currentStep > 1 ? ' done' : ''}`} />
        <div className={`step${currentStep === 2 ? ' active' : ''}${currentStep > 2 ? ' done' : ''}`}>
          <span className="step-num">2</span>
          <span className="step-text">匹配规则</span>
        </div>
        <div className={`step-line${currentStep > 2 ? ' done' : ''}`} />
        <div className={`step${currentStep === 3 ? ' active' : ''}`}>
          <span className="step-num">3</span>
          <span className="step-text">创建监控</span>
        </div>
      </div>

      <BasicMonitorForm
        value={form.basic}
        matching={matching}
        onMatchRule={handleMatch}
        onChange={(basic) => onUpdateForm({ ...form, basic })}
      />

      <RuleMatchPanel
        url={form.basic.url}
        matching={matching}
        matched={matched}
        result={matchResult}
        matchError={matchError}
        appliedIndex={appliedIndex}
        onApply={applyCandidate}
        onDismiss={() => setMatched(false)}
      />

      <MonitorValidationPanel result={validationResult} loading={validationLoading} />

      <details className="advanced-settings">
        <summary>
          <span>
            <strong>高级设置</strong>
            <small>检查频率和分类</small>
          </span>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
            <polyline points="9 18 15 12 9 6" />
          </svg>
        </summary>

        <div className="advanced-content">
          <BasicMonitorForm value={form.basic} advanced groups={groups} onChange={(basic) => onUpdateForm({ ...form, basic })} />

          <div className="validation-actions">
            <div>
              <strong>检查当前配置</strong>
              <p>抓取一次网页，确认系统能够读取到所需内容。</p>
            </div>
            <button className="btn btn-ghost" type="button" disabled={validationLoading} onClick={onValidate}>
              {validationLoading ? '检查中...' : '运行检查'}
            </button>
          </div>

          <MonitorFormSummary form={form} />
        </div>
      </details>

      <div className="form-group monitor-name-option">
        <label>
          监控名称 <span>可选</span>
        </label>
        <input
          value={form.basic.name}
          onChange={(e) => onUpdateForm({ ...form, basic: { ...form.basic, name: e.target.value } })}
          className="form-input"
          placeholder="留空将自动使用网站名称"
        />
      </div>

      {/* 推送账户：基础设置的一部分，位于监控名称下方 */}
      <NotificationEditor
        value={form.notification}
        accounts={accounts}
        onChange={(notification) => onUpdateForm({ ...form, notification })}
      />

      <div className="start-option">
        <label className="checkbox-label">
          <input
            type="checkbox"
            checked={form.basic.isActive}
            onChange={(e) => onUpdateForm({ ...form, basic: { ...form.basic, isActive: e.target.checked } })}
          />
          创建后立即开始监控
        </label>
      </div>

      <div className={`notification-summary${hasNotification ? '' : ' not-configured'}`}>
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="14" height="14">
          <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
          <path d="M13.73 21a2 2 0 0 1-3.46 0" />
        </svg>
        {hasNotification ? (
          <span>通知已配置，变化时将发送提醒</span>
        ) : (
          <>
            <span>未配置通知，仅记录变化不发送提醒</span>
            <Link to="/notifications" className="btn btn-ghost btn-sm">
              添加通知方式
            </Link>
          </>
        )}
      </div>

      {error && <div className="form-error">{error}</div>}

      {showBaselineWarning && (
        <div className="baseline-warning">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
            <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
            <line x1="12" y1="9" x2="12" y2="13" />
            <line x1="12" y1="17" x2="12.01" y2="17" />
          </svg>
          <span>此修改会清除当前比较基线。保存后首次检查只建立新基线，不会发送通知。</span>
        </div>
      )}

      <div className="form-actions">
        <Link to="/" className="btn btn-ghost">
          取消
        </Link>
        {actions}
      </div>
    </div>
  )
}