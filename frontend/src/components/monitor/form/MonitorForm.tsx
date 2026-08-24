import { Link } from 'react-router'
import type { ReactNode } from 'react'
import type { MonitorFormState } from '@/lib/monitorForm'
import type { NotifyAccount, ValidationResult } from '@/api/types'
import BasicMonitorForm from './BasicMonitorForm'
import ExtractionEditor from './ExtractionEditor'
import NotificationEditor from './NotificationEditor'
import MonitorValidationPanel from './MonitorValidationPanel'
import MonitorFormSummary from './MonitorFormSummary'
import './MonitorForm.css'

interface MonitorFormProps {
  form: MonitorFormState
  accounts?: NotifyAccount[]
  error?: string | null
  validationResult?: ValidationResult | null
  validationLoading?: boolean
  showBaselineWarning?: boolean
  actions?: ReactNode
  advancedActions?: ReactNode
  onUpdateForm: (form: MonitorFormState) => void
  onValidate: () => void
}

export default function MonitorForm({
  form,
  accounts = [],
  error,
  validationResult,
  validationLoading = false,
  showBaselineWarning = false,
  actions,
  advancedActions,
  onUpdateForm,
  onValidate,
}: MonitorFormProps) {
  const hasNotification = form.notification.accountIds.length > 0

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
          <span className="step-text">确认内容</span>
        </div>
        <div className={`step-line${currentStep > 2 ? ' done' : ''}`} />
        <div className={`step${currentStep === 3 ? ' active' : ''}`}>
          <span className="step-num">3</span>
          <span className="step-text">创建监控</span>
        </div>
      </div>

      <BasicMonitorForm value={form.basic} onChange={(basic) => onUpdateForm({ ...form, basic })} />

      <ExtractionEditor
        value={form.extraction}
        url={form.basic.url}
        onChange={(extraction) => onUpdateForm({ ...form, extraction })}
      />

      <MonitorValidationPanel result={validationResult} loading={validationLoading} />

      <details className="advanced-settings">
        <summary>
          <span>
            <strong>高级设置</strong>
            <small>通知、检查频率和提取规则</small>
          </span>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
            <polyline points="9 18 15 12 9 6" />
          </svg>
        </summary>

        <div className="advanced-content">
          <BasicMonitorForm value={form.basic} advanced onChange={(basic) => onUpdateForm({ ...form, basic })} />

          <ExtractionEditor
            value={form.extraction}
            url={form.basic.url}
            advanced
            onChange={(extraction) => onUpdateForm({ ...form, extraction })}
          />

          <NotificationEditor
            value={form.notification}
            accounts={accounts}
            onChange={(notification) => onUpdateForm({ ...form, notification })}
          />

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
          {advancedActions && <div className="advanced-actions">{advancedActions}</div>}
        </div>
      </details>

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
            <Link to="/push" className="btn btn-ghost btn-sm">
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
