import { useEffect, useState } from 'react'
import {
  fetchUpdateProxy,
  setUpdateProxy,
  fetchLLMSettings,
  updateLLMSettings,
  testLLMConnection,
  fetchCategories,
  createCategory,
  renameCategory,
  deleteCategory,
  fetchMonitors,
} from '../api/monitors'
import type { Category } from '../api/types'
import UpdatePanel from '../components/UpdatePanel'

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

  // 分类管理
  const [categories, setCategories] = useState<Category[]>([])
  const [monitorCounts, setMonitorCounts] = useState<Record<string, number>>({})
  const [newCategoryName, setNewCategoryName] = useState('')
  const [categoryBusy, setCategoryBusy] = useState(false)
  const [categoryError, setCategoryError] = useState('')
  const [categoryMsg, setCategoryMsg] = useState('')
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editingName, setEditingName] = useState('')

  function loadCategories() {
    fetchCategories()
      .then((res) => {
        if (res.code === 0) setCategories(res.data || [])
      })
      .catch(() => {})
    fetchMonitors()
      .then((res) => {
        if (res.code === 0) {
          const counts: Record<string, number> = {}
          for (const m of res.data || []) {
            const g = (m.group || '').trim() || '默认'
            counts[g] = (counts[g] || 0) + 1
          }
          setMonitorCounts(counts)
        }
      })
      .catch(() => {})
  }

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
    loadCategories()
    return () => {
      cancelled = true
    }
  }, [])

  async function handleCreateCategory() {
    const name = newCategoryName.trim()
    if (!name || categoryBusy) return
    setCategoryBusy(true)
    setCategoryError('')
    setCategoryMsg('')
    try {
      await createCategory(name)
      setNewCategoryName('')
      setCategoryMsg(`已创建分类「${name}」`)
      setTimeout(() => setCategoryMsg(''), 2500)
      loadCategories()
    } catch (error) {
      setCategoryError(error instanceof Error ? error.message : '创建失败')
    }
    setCategoryBusy(false)
  }

  async function handleRenameCategory(id: number) {
    const name = editingName.trim()
    if (!name || categoryBusy) return
    setCategoryBusy(true)
    setCategoryError('')
    setCategoryMsg('')
    try {
      await renameCategory(id, name)
      setEditingId(null)
      setEditingName('')
      setCategoryMsg('分类已重命名')
      setTimeout(() => setCategoryMsg(''), 2500)
      loadCategories()
    } catch (error) {
      setCategoryError(error instanceof Error ? error.message : '重命名失败')
    }
    setCategoryBusy(false)
  }

  async function handleDeleteCategory(id: number, name: string) {
    if (categoryBusy) return
    const confirmed = window.confirm(`确定删除分类「${name}」吗？该分类下的监控器会自动迁移到「默认」。`)
    if (!confirmed) return
    setCategoryBusy(true)
    setCategoryError('')
    setCategoryMsg('')
    try {
      const res = await deleteCategory(id)
      const moved = res.data?.moved
      setCategoryMsg(moved ? `已删除「${name}」，${moved} 个监控器迁移到「默认」` : `已删除分类「${name}」`)
      setTimeout(() => setCategoryMsg(''), 3500)
      loadCategories()
    } catch (error) {
      setCategoryError(error instanceof Error ? error.message : '删除失败')
    }
    setCategoryBusy(false)
  }

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
      <header className="page-header">
        <h1>设置</h1>
        <p className="page-desc">AI 识别、网络更新与系统信息</p>
      </header>

      <section className="settings-section">
        <div className="section-header">
          <h2>AI 模型</h2>
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
          <h2>分类管理</h2>
          <p className="section-desc">维护监控器的分类分组；新建监控时可在高级设置中按分类选择。</p>
        </div>

        <div className="form-group">
          <label>新建分类</label>
          <div className="proxy-row">
            <input
              value={newCategoryName}
              onChange={(e) => setNewCategoryName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') handleCreateCategory()
              }}
              className="form-input"
              placeholder="如 新闻资讯"
            />
            <button className="btn btn-primary" disabled={categoryBusy || !newCategoryName.trim()} onClick={handleCreateCategory}>
              添加
            </button>
          </div>
        </div>

        <div className="category-list">
          {categories.map((cat) => (
            <div key={cat.id} className="category-row">
              {editingId === cat.id ? (
                <>
                  <input
                    value={editingName}
                    onChange={(e) => setEditingName(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') handleRenameCategory(cat.id)
                      if (e.key === 'Escape') {
                        setEditingId(null)
                        setEditingName('')
                      }
                    }}
                    className="form-input"
                    autoFocus
                  />
                  <button className="btn btn-sm btn-primary" disabled={categoryBusy} onClick={() => handleRenameCategory(cat.id)}>
                    保存
                  </button>
                  <button
                    className="btn btn-sm btn-ghost"
                    onClick={() => {
                      setEditingId(null)
                      setEditingName('')
                    }}
                  >
                    取消
                  </button>
                </>
              ) : (
                <>
                  <span className="category-name">{cat.name}</span>
                  <span className="category-count">{monitorCounts[cat.name] ?? 0} 个监控</span>
                  <div className="category-actions">
                    <button
                      className="btn btn-sm btn-ghost"
                      disabled={categoryBusy}
                      onClick={() => {
                        setEditingId(cat.id)
                        setEditingName(cat.name)
                      }}
                    >
                      重命名
                    </button>
                    <button
                      className="btn btn-sm btn-ghost btn-danger-text"
                      disabled={categoryBusy || cat.name === '默认'}
                      title={cat.name === '默认' ? '默认分类不可删除' : '删除后其下监控器迁移到「默认」'}
                      onClick={() => handleDeleteCategory(cat.id, cat.name)}
                    >
                      删除
                    </button>
                  </div>
                </>
              )}
            </div>
          ))}
          {categories.length === 0 && (
            <p className="hint">
              <span className="empty-inline">暂无分类，可新建自定义分类；未分类的监控器属于「默认」。</span>
            </p>
          )}
        </div>
        {categoryMsg && <p className="hint group-hint">{categoryMsg}</p>}
        {categoryError && <p className="hint error-hint">{categoryError}</p>}
      </section>

      <section className="settings-section">
        <div className="section-header">
          <h2>更新代理</h2>
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
          <h2>版本与更新</h2>
          <p className="section-desc">检查新版本并自动更新；下载失败时可在上方配置代理</p>
        </div>
        <UpdatePanel />
      </section>

      <section className="settings-section">
        <div className="section-header">
          <h2>关于</h2>
        </div>
        <p className="section-desc">
          Gentry 是一个自托管的网页内容变更监控工具：规则驱动的内容识别、多渠道实时推送、全自动热更新。所有数据仅保存在本机
          SQLite 中。
        </p>
      </section>
    </div>
  )
}
