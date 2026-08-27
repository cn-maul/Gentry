import { useEffect, useState } from 'react'
import {
  fetchAccounts,
  createAccount,
  updateAccount,
  deleteAccount,
  fetchNotificationSettings,
  updateNotificationSettings,
  fetchNotificationProviders,
} from '../api/monitors'
import type { NotifyAccount, NotificationProviderMeta } from '../api/types'
import { useToastMessages } from '../hooks/useToastMessages'
import PageHeader from '../components/ui/PageHeader'
import EmptyState from '../components/ui/EmptyState'
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
    config: { token: '', channel: 'mail', template: '', url: '', sendkey: '', key: '', server: '', group: '', sound: '' },
  }
}

export default function PushManagement() {
  const { successMsg, pageErrorMsg, showSuccess, showError } = useToastMessages()

  const [loading, setLoading] = useState(true)
  const [accounts, setAccounts] = useState<NotifyAccount[]>([])
  const [globalEnabled, setGlobalEnabled] = useState(false)
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
      if (settings.code === 0) setGlobalEnabled(settings.data?.enabled || false)
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

  function openCreate() {
    setEditingAccount(null)
    setForm(createEmptyForm())
    setModalError('')
    setShowModal(true)
  }

  function openEdit(acc: NotifyAccount) {
    setEditingAccount(acc)
    const cfg = (acc.config || {}) as Partial<AccountConfig>
    const defaultChannel = acc.service === 'pushplus' ? 'mail' : ''
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
        desc="配置推送渠道与全局开关；每个监控器可独立选择接收账户"
        actions={
          <button className="btn btn-primary" onClick={openCreate}>
            新增账户
          </button>
        }
      />

      <Toasts success={successMsg} error={pageErrorMsg} />

      {loading ? (
        <LoadingState text="加载中..." />
      ) : accounts.length === 0 ? (
        <EmptyState
          icon="🔔"
          title="还没有推送账户"
          desc="创建账户后，可在每个监控器中独立选择启用哪些账户"
          action={
            <button className="btn btn-primary" onClick={openCreate}>
              新增账户
            </button>
          }
        />
      ) : (
        <>
          <div className="settings-section">
            <div className="section-header">
              <h2>全局推送开关</h2>
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
            <div className="section-header">
              <h2>推送账户（{accounts.length}）</h2>
              <p className="section-desc">每个账户独立配置，可在监控器详情中选用</p>
            </div>

            {accounts.map((acc) => (
              <div key={acc.id} className="account-card">
                <div className="account-header">
                  <div className="account-info">
                    <span className="account-name">{acc.name}</span>
                    <span className={`service-tag badge-${acc.service}`}>{serviceLabel(acc.service)}</span>
                  </div>
                  <div className="account-actions">
                    <button className="btn-icon" title="编辑" onClick={() => openEdit(acc)}>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
                        <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                        <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
                      </svg>
                    </button>
                    <button className="btn-icon btn-icon-danger" title="删除" onClick={() => setDeleteTarget(acc)}>
                      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
                        <polyline points="3 6 5 6 21 6" />
                        <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
                      </svg>
                    </button>
                  </div>
                </div>
                <div className="account-status">
                  <span className="status-dot-ok" />
                  <span className="status-text">配置正常</span>
                </div>
              </div>
            ))}
          </div>
        </>
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
