import { useEffect, useState } from 'react'
import {
  fetchAccounts,
  createAccount,
  updateAccount,
  deleteAccount,
  testNotifyAccount,
  fetchNotificationSettings,
  updateNotificationSettings,
  fetchNotificationProviders,
} from '../api/monitors'
import type { NotifyAccount, NotificationProviderMeta, PushTemplate } from '../api/types'
import { useToastMessages } from '../hooks/useToastMessages'
import PageHeader from '../components/ui/PageHeader'
import LoadingState from '../components/ui/LoadingState'
import Toasts from '../components/ui/Toasts'
import Modal from '../components/ui/Modal'
import ConfirmModal from '../components/ui/ConfirmModal'

interface AccountConfig {
  [key: string]: string
  token: string
  channel: string
  template: string
  url: string
  sendkey: string
  key: string
  server: string
  group: string
  sound: string
}

interface AccountForm {
  name: string
  service: string
  config: AccountConfig
}

function createEmptyForm(): AccountForm {
  return {
    name: '',
    service: 'pushplus',
    // channel 留空让 pushplus 走默认的 wechat（微信公众号）渠道；
    // 不要默认 mail——mail 渠道需要先在 pushplus 后台配置渠道编码，否则推送失败
    config: { token: '', channel: '', template: '', url: '', sendkey: '', key: '', server: '', group: '', sound: '' },
  }
}

export default function PushManagement() {
  const { successMsg, pageErrorMsg, showSuccess, showError } = useToastMessages()

  const [loading, setLoading] = useState(true)
  const [accounts, setAccounts] = useState<NotifyAccount[]>([])
  const [globalEnabled, setGlobalEnabled] = useState(false)
  const [templates, setTemplates] = useState<PushTemplate[]>([])
  const [activeTemplate, setActiveTemplate] = useState('')
  const [savingTemplate, setSavingTemplate] = useState(false)
  const [showTemplateModal, setShowTemplateModal] = useState(false)
  const [templateForm, setTemplateForm] = useState<PushTemplate>({ name: '', title_template: '', content_template: '' })
  const [editingTemplateName, setEditingTemplateName] = useState<string | null>(null)
  const [testingId, setTestingId] = useState<number | null>(null)
  const [showModal, setShowModal] = useState(false)
  const [editingAccount, setEditingAccount] = useState<NotifyAccount | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<NotifyAccount | null>(null)
  const [modalSaving, setModalSaving] = useState(false)
  const [modalError, setModalError] = useState('')

  const [providers, setProviders] = useState<Record<string, NotificationProviderMeta>>({})
  const [form, setForm] = useState<AccountForm>(createEmptyForm)

  function serviceLabel(s: string) {
    return providers[s]?.label || s
  }

  const providerOptions = (() => {
    const entries = Object.entries(providers)
    if (entries.length === 0) {
      return [
        { service: 'pushplus', label: 'PushPlus' },
        { service: 'webhook', label: 'Webhook' },
        { service: 'serverchan', label: 'Server酱' },
        { service: 'bark', label: 'Bark' },
      ]
    }
    return entries.map(([service, meta]) => ({ service, label: meta.label || service }))
  })()

  async function loadAll() {
    setLoading(true)
    try {
      const [accts, settings, providerRes] = await Promise.all([
        fetchAccounts(),
        fetchNotificationSettings(),
        fetchNotificationProviders().catch(() => ({ code: -1, message: '', data: {} as Record<string, NotificationProviderMeta> })),
      ])
      if (accts.code === 0) setAccounts(accts.data || [])
      if (settings.code === 0) {
        setGlobalEnabled(settings.data?.enabled || false)
        setTemplates(settings.data?.templates || [])
        setActiveTemplate(settings.data?.active_template || '')
      }
      if (providerRes.code === 0) setProviders(providerRes.data || {})
    } catch (e) {
      showError('加载失败: ' + (e instanceof Error ? e.message : ''))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadAll()
  }, [])

  async function saveGlobalEnabled(enabled: boolean) {
    try {
      await updateNotificationSettings({ enabled })
      showSuccess(enabled ? '推送已开启' : '推送已关闭')
    } catch (e) {
      showError('操作失败: ' + (e instanceof Error ? e.message : ''))
      setGlobalEnabled(!enabled)
    }
  }

  // ===== 推送模板管理 =====

  function openTemplateCreate() {
    setEditingTemplateName(null)
    setTemplateForm({ name: '', title_template: '', content_template: '' })
    setShowTemplateModal(true)
  }

  function openTemplateEdit(tpl: PushTemplate) {
    setEditingTemplateName(tpl.name)
    setTemplateForm({ name: tpl.name, title_template: tpl.title_template, content_template: tpl.content_template })
    setShowTemplateModal(true)
  }

  // 保存（新增或编辑）模板：整表提交，同时把开关状态一并带上避免覆盖
  async function saveTemplate() {
    const name = templateForm.name.trim()
    if (!name) {
      showError('请输入模板名称')
      return
    }
    if (templates.some((t) => t.name === name && t.name !== editingTemplateName)) {
      showError(`已存在同名模板「${name}」`)
      return
    }
    setSavingTemplate(true)
    try {
      const newTpl: PushTemplate = {
        name,
        title_template: templateForm.title_template,
        content_template: templateForm.content_template,
      }
      const next = editingTemplateName !== null ? templates.map((t) => (t.name === editingTemplateName ? newTpl : t)) : [...templates, newTpl]
      // 编辑了当前选中的模板（含重命名）时，选中项跟随
      const nextActive = editingTemplateName !== null && activeTemplate === editingTemplateName ? name : activeTemplate
      await updateNotificationSettings({ enabled: globalEnabled, templates: next, active_template: nextActive })
      setTemplates(next)
      setActiveTemplate(nextActive)
      showSuccess('推送模板已保存')
      setShowTemplateModal(false)
    } catch (e) {
      showError('保存失败: ' + (e instanceof Error ? e.message : ''))
    } finally {
      setSavingTemplate(false)
    }
  }

  // 选中某个模板（name 为空 = 使用内置默认模板）
  async function selectActiveTemplate(name: string) {
    try {
      await updateNotificationSettings({ enabled: globalEnabled, active_template: name })
      setActiveTemplate(name)
      showSuccess(name ? `已切换模板：${name}` : '已切换为默认模板')
    } catch (e) {
      showError('切换失败: ' + (e instanceof Error ? e.message : ''))
    }
  }

  async function deleteTemplate(tpl: PushTemplate) {
    if (!window.confirm(`确定删除模板「${tpl.name}」吗？`)) return
    setSavingTemplate(true)
    try {
      const next = templates.filter((t) => t.name !== tpl.name)
      const nextActive = activeTemplate === tpl.name ? '' : activeTemplate
      await updateNotificationSettings({ enabled: globalEnabled, templates: next, active_template: nextActive })
      setTemplates(next)
      setActiveTemplate(nextActive)
      showSuccess('模板已删除')
    } catch (e) {
      showError('删除失败: ' + (e instanceof Error ? e.message : ''))
    } finally {
      setSavingTemplate(false)
    }
  }

  function openCreate() {
    setEditingAccount(null)
    setForm(createEmptyForm())
    setModalError('')
    setShowModal(true)
  }

  function openEdit(acc: NotifyAccount) {
    setEditingAccount(acc)
    const cfg = (acc.config || {}) as Partial<AccountConfig>
    const defaultChannel = ''
    setForm({
      name: acc.name,
      service: acc.service,
      config: {
        token: cfg.token || '',
        channel: cfg.channel || defaultChannel,
        template: cfg.template || '',
        url: cfg.url || '',
        sendkey: cfg.sendkey || '',
        key: cfg.key || '',
        server: cfg.server || '',
        group: cfg.group || '',
        sound: cfg.sound || '',
      },
    })
    setModalError('')
    setShowModal(true)
  }

  async function handleDeleteAccount() {
    const acc = deleteTarget
    setDeleteTarget(null)
    if (!acc) return
    try {
      await deleteAccount(acc.id)
      setAccounts((prev) => prev.filter((a) => a.id !== acc.id))
      showSuccess(`「${acc.name}」已删除`)
    } catch (e) {
      showError('删除失败: ' + (e instanceof Error ? e.message : ''))
    }
  }

  async function handleTestAccount(acc: NotifyAccount) {
    setTestingId(acc.id)
    try {
      await testNotifyAccount(acc.id)
      showSuccess(`「${acc.name}」测试推送已发送，请注意查收`)
    } catch (e) {
      showError(`「${acc.name}」测试推送失败: ` + (e instanceof Error ? e.message : ''))
    } finally {
      setTestingId(null)
    }
  }

  function requiredFieldsFor(service: string) {
    return providers[service]?.required_fields || []
  }

  async function handleSaveAccount() {
    if (!form.name.trim()) {
      setModalError('请输入账户名称')
      return
    }
    for (const field of requiredFieldsFor(form.service)) {
      if (!String((form.config as Record<string, unknown>)[field] || '').trim()) {
        setModalError(`请输入 ${field}`)
        return
      }
    }

    setModalError('')
    setModalSaving(true)
    try {
      const payload = {
        name: form.name.trim(),
        service: form.service,
        config: form.config,
      }
      if (editingAccount) {
        await updateAccount(editingAccount.id, payload)
        showSuccess('账户已更新')
      } else {
        await createAccount(payload)
        showSuccess('账户已创建')
      }
      setShowModal(false)
      await loadAll()
    } catch (e) {
      setModalError(e instanceof Error ? e.message : '保存失败')
    } finally {
      setModalSaving(false)
    }
  }

  function updateConfig<K extends keyof AccountConfig>(key: K, value: string) {
    setForm((prev) => ({ ...prev, config: { ...prev.config, [key]: value } }))
  }

  return (
    <div className="push-page">
      <PageHeader
        title="推送通知"
        actions={
          <button className="btn btn-primary" onClick={openCreate}>
            新增账户
          </button>
        }
      />

      <Toasts success={successMsg} error={pageErrorMsg} />

      {loading ? (
        <LoadingState text="加载中..." />
      ) : (
        <div className="push-body">
          {/* ═══ 左栏：推送账户列表 ═══ */}
          <div className="push-left">
            <div className="settings-section">
              <div className="section-header row">
                <h2>推送账户（{accounts.length}）</h2>
                <button className="btn btn-sm btn-primary" onClick={openCreate}>
                  新增账户
                </button>
              </div>

              {accounts.length === 0 ? (
                <p className="hint">还没有推送账户，点击右上角「新增账户」创建</p>
              ) : (
                accounts.map((acc) => (
                  <div key={acc.id} className="account-card" onClick={() => openEdit(acc)} title="点击编辑账户">
                    <div className="account-header">
                      <div className="account-info">
                        <span className="account-name">{acc.name}</span>
                        <span className={`service-tag badge-${acc.service}`}>{serviceLabel(acc.service)}</span>
                      </div>
                      <div className="account-actions">
                        <button
                          className="btn btn-sm btn-ghost"
                          disabled={testingId === acc.id}
                          title="发送一条测试推送，验证配置是否可正常送达"
                          onClick={(e) => {
                            e.stopPropagation()
                            handleTestAccount(acc)
                          }}
                        >
                          {testingId === acc.id ? '测试中...' : '测试推送'}
                        </button>
                        <button
                          className="btn-icon"
                          title="编辑"
                          onClick={(e) => {
                            e.stopPropagation()
                            openEdit(acc)
                          }}
                        >
                          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
                            <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                            <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                          </svg>
                        </button>
                        <button
                          className="btn-icon btn-icon-danger"
                          title="删除"
                          onClick={(e) => {
                            e.stopPropagation()
                            setDeleteTarget(acc)
                          }}
                        >
                          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
                            <polyline points="3 6 5 6 21 6" />
                            <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
                          </svg>
                        </button>
                      </div>
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>

          {/* ═══ 右栏：推送设置 + 推送模板 ═══ */}
          <div className="push-right">
            <div className="settings-section">
              <div className="section-header">
                <h2>推送设置</h2>
              </div>
              <div className="setting-item">
                <div className="setting-info">
                  <div className="setting-label">启用推送</div>
                  <div className="setting-desc">关闭后所有监控器都不会发送推送</div>
                </div>
                <div className="setting-control">
                  <label className="toggle">
                    <input
                      type="checkbox"
                      checked={globalEnabled}
                      onChange={(e) => {
                        setGlobalEnabled(e.target.checked)
                        saveGlobalEnabled(e.target.checked)
                      }}
                    />
                    <span className="toggle-track"></span>
                  </label>
                </div>
              </div>
            </div>

            <div className="settings-section">
              <div className="section-header row">
                <h2>推送模板</h2>
                <button className="btn btn-sm btn-ghost" onClick={openTemplateCreate}>
                  新增
                </button>
              </div>
              <p className="section-desc">点击列表项选用模板，点击「新增」创建</p>

              {/* 内置默认模板：始终可选，不可编辑/删除 */}
              <div
                className={`template-item${activeTemplate === '' ? ' active' : ''}`}
                title="点击选用"
                onClick={() => selectActiveTemplate('')}
              >
                <div className="template-item-head">
                  <span className="template-radio" />
                  <span className="template-name">默认模板</span>
                </div>
                <span className="template-value">
                  标题 {`{site_name} 有 {count} 条更新`} · 内容 {`最新更新内容：{items}`}
                </span>
              </div>

              {templates.map((tpl) => (
                <div
                  key={tpl.name}
                  className={`template-item${activeTemplate === tpl.name ? ' active' : ''}`}
                  title="点击选用"
                  onClick={() => selectActiveTemplate(tpl.name)}
                >
                  <div className="template-item-head">
                    <span className="template-radio" />
                    <span className="template-name">{tpl.name}</span>
                    <span className="template-actions">
                      <button
                        className="btn-icon"
                        title="编辑模板"
                        onClick={(e) => {
                          e.stopPropagation()
                          openTemplateEdit(tpl)
                        }}
                      >
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="15" height="15">
                          <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                          <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                        </svg>
                      </button>
                      <button
                        className="btn-icon btn-icon-danger"
                        title="删除模板"
                        onClick={(e) => {
                          e.stopPropagation()
                          deleteTemplate(tpl)
                        }}
                      >
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="15" height="15">
                          <polyline points="3 6 5 6 21 6" />
                          <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
                        </svg>
                      </button>
                    </span>
                  </div>
                  <span className="template-value">
                    {(tpl.title_template || '默认标题')} · {(tpl.content_template || '默认内容')}
                  </span>
                </div>
              ))}

              {templates.length === 0 && <p className="hint">还没有自定义模板，点「新增」创建</p>}
            </div>
          </div>
        </div>
      )}

      {/* Create/Edit Modal */}
      <Modal
        open={showModal}
        title={editingAccount ? '编辑账户' : '新增账户'}
        onClose={() => setShowModal(false)}
        footer={
          <>
            <button className="btn btn-ghost" onClick={() => setShowModal(false)}>
              取消
            </button>
            <button className="btn btn-primary" disabled={modalSaving} onClick={handleSaveAccount}>
              {modalSaving ? '保存中...' : '保存'}
            </button>
          </>
        }
      >
              <div className="form-group">
                <label>账户名称</label>
                <input
                  value={form.name}
                  onChange={(e) => setForm((prev) => ({ ...prev, name: e.target.value }))}
                  className="form-input"
                  placeholder="如 张三、运维群"
                />
              </div>
              <div className="form-group">
                <label>推送服务</label>
                <select
                  value={form.service}
                  onChange={(e) => setForm((prev) => ({ ...prev, service: e.target.value }))}
                  className="form-input"
                >
                  {providerOptions.map((provider) => (
                    <option key={provider.service} value={provider.service}>
                      {provider.label}
                    </option>
                  ))}
                </select>
              </div>

              {form.service === 'pushplus' && (
                <>
                  <div className="form-group">
                    <label>Token</label>
                    <input
                      value={form.config.token}
                      onChange={(e) => updateConfig('token', e.target.value)}
                      className="form-input"
                      placeholder="PushPlus 用户令牌"
                    />
                  </div>
                  <div className="form-group">
                    <label>Channel（可选）</label>
                    <input
                      value={form.config.channel}
                      onChange={(e) => updateConfig('channel', e.target.value)}
                      className="form-input"
                      placeholder="mail / wechat / sms"
                    />
                  </div>
                  <div className="form-group">
                    <label>Template（可选）</label>
                    <select
                      value={form.config.template}
                      onChange={(e) => updateConfig('template', e.target.value)}
                      className="form-input"
                    >
                      <option value="">默认 (html)</option>
                      <option value="html">html</option>
                      <option value="txt">纯文本 (txt)</option>
                      <option value="json">json</option>
                      <option value="markdown">markdown</option>
                    </select>
                  </div>
                </>
              )}

              {form.service === 'webhook' && (
                <div className="form-group">
                  <label>Webhook URL</label>
                  <input
                    value={form.config.url}
                    onChange={(e) => updateConfig('url', e.target.value)}
                    className="form-input"
                    placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
                  />
                </div>
              )}

              {form.service === 'serverchan' && (
                <>
                  <div className="form-group">
                    <label>SendKey</label>
                    <input
                      value={form.config.sendkey}
                      onChange={(e) => updateConfig('sendkey', e.target.value)}
                      className="form-input"
                      placeholder="Server酱 SendKey"
                    />
                  </div>
                  <div className="form-group">
                    <label>Channel（可选）</label>
                    <select
                      value={form.config.channel}
                      onChange={(e) => updateConfig('channel', e.target.value)}
                      className="form-input"
                    >
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
                </>
              )}

              {form.service === 'bark' && (
                <>
                  <div className="form-group">
                    <label>Key</label>
                    <input
                      value={form.config.key}
                      onChange={(e) => updateConfig('key', e.target.value)}
                      className="form-input"
                      placeholder="Bark 设备 Key"
                    />
                  </div>
                  <div className="form-group">
                    <label>Server（可选）</label>
                    <input
                      value={form.config.server}
                      onChange={(e) => updateConfig('server', e.target.value)}
                      className="form-input"
                      placeholder="默认 https://api.day.app"
                    />
                  </div>
                  <div className="form-group">
                    <label>Group（可选）</label>
                    <input
                      value={form.config.group}
                      onChange={(e) => updateConfig('group', e.target.value)}
                      className="form-input"
                      placeholder="通知分组名称"
                    />
                  </div>
                  <div className="form-group">
                    <label>Sound（可选）</label>
                    <input
                      value={form.config.sound}
                      onChange={(e) => updateConfig('sound', e.target.value)}
                      className="form-input"
                      placeholder="自定义铃声名称"
                    />
                  </div>
                </>
              )}

              {modalError && <div className="form-error">{modalError}</div>}
      </Modal>

      {/* 推送模板 Modal */}
      <Modal
        open={showTemplateModal}
        title={editingTemplateName !== null ? '编辑推送模板' : '新增推送模板'}
        onClose={() => setShowTemplateModal(false)}
        footer={
          <>
            <button className="btn btn-ghost" onClick={() => setShowTemplateModal(false)}>
              取消
            </button>
            <button className="btn btn-primary" disabled={savingTemplate} onClick={saveTemplate}>
              {savingTemplate ? '保存中...' : '保存'}
            </button>
          </>
        }
      >
        <div className="form-group">
          <label>模板名称</label>
          <input
            value={templateForm.name}
            onChange={(e) => setTemplateForm((prev) => ({ ...prev, name: e.target.value }))}
            className="form-input"
            placeholder="如 正式公告、简洁通知"
          />
        </div>
        <div className="form-group">
          <label>标题模板</label>
          <input
            value={templateForm.title_template}
            onChange={(e) => setTemplateForm((prev) => ({ ...prev, title_template: e.target.value }))}
            className="form-input"
            placeholder="默认：{site_name} 有 {count} 条更新"
          />
        </div>
        <div className="form-group">
          <label>内容模板</label>
          <textarea
            value={templateForm.content_template}
            onChange={(e) => setTemplateForm((prev) => ({ ...prev, content_template: e.target.value }))}
            className="form-input"
            rows={4}
            placeholder={'默认：最新更新内容：\n{items}'}
          />
        </div>
        <p className="hint">
          可用变量：{'{site_name}'}（站点名） {'{count}'}（更新条数） {'{items}'}（条目列表） {'{titles}'}（标题合集）；留空使用内置默认模板
        </p>
      </Modal>

      {/* Delete Confirm */}
      <ConfirmModal
        open={deleteTarget !== null}
        title="确认删除"
        danger
        confirmText="确认删除"
        onConfirm={handleDeleteAccount}
        onCancel={() => setDeleteTarget(null)}
      >
        <p>确定删除推送账户「{deleteTarget?.name}」吗？</p>
        <p className="mt-2">已选择此账户的监控器将不再收到推送。</p>
      </ConfirmModal>
    </div>
  )
}
