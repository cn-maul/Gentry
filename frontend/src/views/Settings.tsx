import { useEffect, useState } from 'react'
import {
  fetchUpdateProxy,
  setUpdateProxy,
  fetchLLMSettings,
  updateLLMSettings,
  testLLMConnection,
} from '../api/monitors'
import UpdatePanel from '../components/UpdatePanel'
import PageHeader from '../components/ui/PageHeader'

export default function Settings() {
  const [proxy, setProxy] = useState('')
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [errorMsg, setErrorMsg] = useState('')

  // AI 模型接入
  const [aiBaseURL, setAIBaseURL] = useState('')
  const [aiAPIKey, setAIAPIKey] = useState('')
  const [aiModel, setAIModel] = useState('')
  const [aiSaving, setAISaving] = useState(false)
  const [aiSaved, setAISaved] = useState(false)
  const [aiErrorMsg, setAIErrorMsg] = useState('')
  const [aiTesting, setAITesting] = useState(false)
  const [aiTestMsg, setAITestMsg] = useState('')

  useEffect(() => {
    let cancelled = false
    fetchUpdateProxy()
      .then((res) => {
        if (!cancelled && res.code === 0 && res.data?.proxy) {
          setProxy(res.data.proxy)
        }
      })
      .catch(() => {})
    fetchLLMSettings()
      .then((res) => {
        if (!cancelled && res.code === 0 && res.data) {
          setAIBaseURL(res.data.base_url || '')
          setAIAPIKey(res.data.api_key || '')
          setAIModel(res.data.model || '')
        }
      })
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [])

  async function handleSave() {
    setSaving(true)
    setSaved(false)
    setErrorMsg('')
    try {
      await setUpdateProxy(proxy)
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch (error) {
      setErrorMsg(error instanceof Error ? error.message : '保存失败，请检查代理地址')
    }
    setSaving(false)
  }

  async function handleAISave() {
    setAISaving(true)
    setAISaved(false)
    setAIErrorMsg('')
    setAITestMsg('')
    try {
      await updateLLMSettings({ base_url: aiBaseURL, api_key: aiAPIKey, model: aiModel })
      setAISaved(true)
      setTimeout(() => setAISaved(false), 2000)
    } catch (error) {
      setAIErrorMsg(error instanceof Error ? error.message : '保存失败')
    }
    setAISaving(false)
  }

  async function handleAITest() {
    setAITesting(true)
    setAITestMsg('')
    setAIErrorMsg('')
    try {
      // 先保存当前输入再测试，避免「配置已改但测的是旧配置」的困惑
      await updateLLMSettings({ base_url: aiBaseURL, api_key: aiAPIKey, model: aiModel })
      const res = await testLLMConnection()
      if (res.code === 0 && res.data?.ok) {
        setAITestMsg(`连接成功：${res.data.model || aiModel} 回复「${res.data.answer || 'ok'}」`)
      } else {
        setAITestMsg(res.message || '连接失败')
      }
    } catch (error) {
      setAITestMsg(error instanceof Error ? error.message : '连接失败')
    }
    setAITesting(false)
  }

  return (
    <div className="settings-page">
      <PageHeader title="设置" />

      <section className="settings-section">
        <div className="section-header">
          <div className="section-title-row">
            <span className="section-icon section-icon-brand">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
                <rect x="4" y="4" width="16" height="16" rx="3" />
                <path d="M9 9h6" />
                <path d="M9 13h6" />
                <path d="M9 17h3" />
              </svg>
            </span>
            <h2>AI 模型</h2>
          </div>
          <p className="section-desc">
            配置 OpenAI 兼容接口后，可在「规则库」页使用 AI 提取自动识别内容结构。支持 DeepSeek、通义千问、Moonshot、OpenAI、Ollama 等服务。
          </p>
        </div>

        <div className="form-group">
          <label>接入地址（Base URL）</label>
          <input
            value={aiBaseURL}
            onChange={(e) => setAIBaseURL(e.target.value)}
            className="form-input"
            placeholder="https://api.deepseek.com/v1"
          />
          <p className="hint">填到版本号即可，系统会自动拼接 /chat/completions；本地 Ollama 填 http://127.0.0.1:11434/v1</p>
        </div>

        <div className="form-group">
          <label>API Key</label>
          <input
            value={aiAPIKey}
            onChange={(e) => setAIAPIKey(e.target.value)}
            className="form-input"
            placeholder="sk-...（本地服务可留空）"
            autoComplete="off"
          />
        </div>

        <div className="form-group">
          <label>模型名称</label>
          <input
            value={aiModel}
            onChange={(e) => setAIModel(e.target.value)}
            className="form-input"
            placeholder="deepseek-chat"
          />
        </div>

        <div className="ai-actions">
          <button className="btn btn-primary" disabled={aiSaving} onClick={handleAISave}>
            {aiSaving ? '保存中...' : '保存'}
          </button>
          <button className="btn btn-ghost" disabled={aiTesting || !aiBaseURL.trim() || !aiModel.trim()} onClick={handleAITest}>
            {aiTesting ? '测试中...' : '测试连接'}
          </button>
          {aiSaved && <span className="hint">已保存</span>}
        </div>
        {aiErrorMsg && <p className="hint error-hint">{aiErrorMsg}</p>}
        {aiTestMsg && <p className="hint">{aiTestMsg}</p>}
      </section>

      <section className="settings-section">
        <div className="section-header">
          <div className="section-title-row">
            <span className="section-icon section-icon-brand">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
                <rect x="3" y="6" width="18" height="12" rx="2" />
                <path d="M3 10h18" />
                <path d="M7 15h2" />
                <path d="M12 15h5" />
              </svg>
            </span>
            <h2>更新代理</h2>
          </div>
          <p className="section-desc">如果下载更新时连接 GitHub 失败，可以设置代理地址</p>
        </div>

        <div className="form-group">
          <label>代理地址</label>
          <div className="proxy-row">
            <input value={proxy} onChange={(e) => setProxy(e.target.value)} className="form-input" placeholder="http://127.0.0.1:7897" />
            <button className="btn btn-primary" disabled={saving} onClick={handleSave}>
              {saving ? '保存中...' : '保存'}
            </button>
          </div>
          {saved && <p className="hint">已保存</p>}
          {errorMsg && <p className="hint error-hint">{errorMsg}</p>}
        </div>
      </section>

      <section className="settings-section">
        <div className="section-header">
          <div className="section-title-row">
            <span className="section-icon section-icon-brand">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
                <circle cx="12" cy="12" r="9" />
                <path d="M12 7v5l3 2" />
              </svg>
            </span>
            <h2>版本与更新</h2>
          </div>
          <p className="section-desc">检查新版本并自动更新；下载失败时可在上方配置代理</p>
        </div>
        <UpdatePanel />
      </section>

      <section className="settings-section">
        <div className="section-header">
          <div className="section-title-row">
            <span className="section-icon section-icon-brand">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" width="16" height="16">
                <circle cx="12" cy="12" r="9" />
                <path d="M3.5 9h17" />
                <path d="M3.5 15h17" />
                <path d="M12 3a15 15 0 0 1 0 18" />
                <path d="M12 3a15 15 0 0 0 0 18" />
              </svg>
            </span>
            <h2>关于</h2>
          </div>
        </div>
        <p className="section-desc">
          Gentry 是一个自托管的网页内容变更监控工具：规则驱动的内容识别、多渠道实时推送、全自动热更新。所有数据仅保存在本机
          SQLite 中。
        </p>
      </section>
    </div>
  )
}
