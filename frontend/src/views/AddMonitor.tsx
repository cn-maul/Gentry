import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router'
import {
  createMonitor,
  updateMonitor,
  fetchMonitorConfig,
  fetchAccounts,
  fetchCategories,
  validateMonitorConfig,
} from '../api/monitors'
import type { Category, NotifyAccount, ValidationResult } from '../api/types'
import MonitorForm from '../components/monitor/form/MonitorForm'
import {
  createEmptyForm,
  toMonitorRequest,
  fromMonitorResponse,
  hasSemanticChange,
  validateForm,
  getDetectionFingerprint,
  suggestMonitorName,
} from '../lib/monitorForm'
import type { MonitorFormState } from '../lib/monitorForm'
import { useToastMessages } from '../hooks/useToastMessages'

export default function AddMonitor() {
  const navigate = useNavigate()
  const { name: routeName } = useParams()
  const isEdit = !!routeName
  const { successMsg, pageErrorMsg } = useToastMessages()

  const [loading, setLoading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)

  const [form, setForm] = useState<MonitorFormState>(createEmptyForm)
  const [originalFormSnapshot, setOriginalFormSnapshot] = useState<MonitorFormState | null>(null)

  const [accounts, setAccounts] = useState<NotifyAccount[]>([])
  const [categories, setCategories] = useState<Category[]>([])

  // Validation
  const [validationResult, setValidationResult] = useState<ValidationResult | null>(null)
  const [validationLoading, setValidationLoading] = useState(false)
  const [, setValidatedFingerprint] = useState('')
  const [, setValidationAttemptFingerprint] = useState('')

  // Baseline warning
  const [baselineWarning, setBaselineWarning] = useState(false)

  const pageTitle = `${isEdit ? '编辑' : '新增'}网页监控`

  // 对应 Vue：watch(getDetectionFingerprint(form)) — 语义变化后失效旧验证结果并刷新基线警告
  const detectionFingerprint = getDetectionFingerprint(form)
  useEffect(() => {
    setValidatedFingerprint((prev) => {
      if (prev && prev !== detectionFingerprint) {
        setValidationResult(null)
        return ''
      }
      return prev
    })
    setBaselineWarning(Boolean(isEdit && originalFormSnapshot && hasSemanticChange(originalFormSnapshot, form)))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [detectionFingerprint, originalFormSnapshot, isEdit])

  // 对应 Vue：watch(JSON.stringify(toMonitorRequest(form))) — 表单变化后失效验证尝试标记
  const requestFingerprint = JSON.stringify(toMonitorRequest(form))
  useEffect(() => {
    setValidationAttemptFingerprint((prev) => {
      if (prev && prev !== requestFingerprint) {
        setValidationResult(null)
        return ''
      }
      return prev
    })
  }, [requestFingerprint])

  // Load accounts, categories and edit data
  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const acctRes = await fetchAccounts()
        if (!cancelled) setAccounts((acctRes.code === 0 ? acctRes.data : []) || [])
      } catch {
        /* ignore */
      }

      try {
        const catRes = await fetchCategories()
        if (!cancelled) setCategories((catRes.code === 0 ? catRes.data : []) || [])
      } catch {
        /* ignore */
      }

      if (routeName) {
        setLoading(true)
        try {
          const res = await fetchMonitorConfig(routeName)
          if (!cancelled) {
            if (res.code === 0 && res.data) {
              const loaded = fromMonitorResponse(res.data)
              setForm(loaded)
              setOriginalFormSnapshot(loaded)
            } else {
              setSubmitError(res.message || '加载配置失败')
            }
          }
        } catch (e) {
          if (!cancelled) setSubmitError('加载配置失败: ' + (e instanceof Error ? e.message : ''))
        } finally {
          if (!cancelled) setLoading(false)
        }
      }
    }
    load()
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [routeName])

  function currentFormWithEnsuredName(prev: MonitorFormState): MonitorFormState {
    if (prev.basic.name.trim()) return prev
    return { ...prev, basic: { ...prev.basic, name: suggestMonitorName(prev.basic.url) } }
  }

  // Validate before submit
  async function runValidation(): Promise<boolean> {
    // 事件处理器闭包中的 form 即最新已提交状态，直接计算并提交，
    // 不依赖 updater 同步执行（React 并发模式下不保证）
    const target = currentFormWithEnsuredName(form)
    setForm(target)
    setValidationAttemptFingerprint(JSON.stringify(toMonitorRequest(target)))
    const localError = validateForm(target)
    if (localError) {
      setValidationResult({ valid: false, errors: [localError], summary: '请先修正表单配置后再验证。' })
      setSubmitError(localError)
      return false
    }
    setValidationLoading(true)
    setSubmitError(null)
    try {
      const payload = toMonitorRequest(target)
      const res = await validateMonitorConfig(payload)
      if (res.code === 0 && res.data) {
        setValidationResult(res.data)
        setValidatedFingerprint(getDetectionFingerprint(target))
        return true
      }
      const message = res.message || '验证失败'
      setValidationResult({ valid: false, errors: [message], summary: '配置未通过验证。' })
      return false
    } catch (e) {
      const message = e instanceof Error ? e.message : '验证失败'
      setValidationResult({ valid: false, errors: [message], summary: '配置未通过验证。' })
      return false
    } finally {
      setValidationLoading(false)
    }
  }

  async function handleSubmit() {
    const target = currentFormWithEnsuredName(form)
    setForm(target)
    const err = validateForm(target)
    if (err) {
      setSubmitError(err)
      return
    }
    setSubmitError(null)
    setSubmitting(true)
    try {
      const payload = toMonitorRequest(target)
      if (routeName) {
        await updateMonitor(routeName, payload)
      } else {
        await createMonitor(payload)
      }
      navigate('/')
    } catch (e) {
      setSubmitError(e instanceof Error ? e.message : '提交失败')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="add-monitor">
      <div className="page-header">
        <div>
          <Link to="/" className="back-btn">
            ← 返回
          </Link>
          <h1>{pageTitle}</h1>
        </div>
      </div>

      {successMsg && <div className="toast toast-success">{successMsg}</div>}
      {pageErrorMsg && <div className="toast toast-warning">{pageErrorMsg}</div>}

      {isEdit && loading ? (
        <div className="loading">
          <div className="spinner" />
          <p>加载配置...</p>
        </div>
      ) : (
        <MonitorForm
          form={form}
          accounts={accounts}
          groups={categories.map((c) => c.name)}
          error={submitError}
          validationResult={validationResult}
          validationLoading={validationLoading}
          showBaselineWarning={baselineWarning}
          onValidate={runValidation}
          onUpdateForm={(newForm) => setForm(newForm)}
          actions={
            <button className="btn btn-primary" disabled={submitting} onClick={handleSubmit}>
              {submitting ? '提交中...' : isEdit ? '保存修改' : form.basic.isActive ? '创建并启动' : '仅创建'}
            </button>
          }
        />
      )}
    </div>
  )
}