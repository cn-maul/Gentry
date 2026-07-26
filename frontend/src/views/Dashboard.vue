<template>
  <div class="dashboard">
    <div class="page-header">
      <div>
        <h1>监控器</h1>
        <p class="page-desc">管理和监控网页内容变更</p>
      </div>
      <div class="header-actions">
        <router-link to="/add" class="btn btn-primary">新增监控器</router-link>
      </div>
    </div>

    <div class="toast toast-success" v-if="successMsg">{{ successMsg }}</div>
    <div class="toast toast-warning" v-if="pageErrorMsg">{{ pageErrorMsg }}</div>

    <div class="loading" v-if="loading">
      <div class="spinner" />
      <p>加载中...</p>
    </div>

    <div class="empty" v-else-if="error">
      <div class="empty-icon">❌</div>
      <p>加载失败</p>
      <p style="color: var(--text-muted); font-size: 0.8125rem; margin-top: 0.25rem;">{{ error }}</p>
      <button class="btn btn-primary btn-sm" style="margin-top: 1rem;" @click="loadData">重试</button>
    </div>

    <div class="empty" v-else-if="!monitors || monitors.length === 0">
      <div class="empty-icon">📡</div>
      <p class="empty-title">还没有监控任务</p>
      <p class="empty-desc">粘贴一个网页地址，系统会自动识别需要关注的内容</p>
      <router-link to="/add" class="btn btn-primary" style="margin-top: 1.25rem;">创建第一个监控</router-link>
      <div class="empty-hints">
        <span>支持公告更新</span>
        <span class="hint-dot" />
        <span>商品降价提醒</span>
        <span class="hint-dot" />
        <span>目标价格提醒</span>
      </div>
    </div>

    <!-- Delete confirm modal -->
    <div class="modal-overlay" v-if="deleteTarget" @click.self="deleteTarget = null">
      <div class="modal-container">
        <div class="modal-header">
          <h2>确认删除</h2>
          <button class="modal-close" @click="deleteTarget = null">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="18" height="18"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
          </button>
        </div>
        <div class="modal-body">
          <p>确定要删除监控器「{{ deleteTarget }}」吗？</p>
          <p style="margin-top: 0.5rem;">删除后无法恢复。</p>
        </div>
        <div class="modal-footer">
          <button class="btn btn-ghost" @click="deleteTarget = null">取消</button>
          <button class="btn btn-danger" @click="handleDelete">确认删除</button>
        </div>
      </div>
    </div>

    <template v-else>
      <div v-for="group in groupList" :key="group.name" class="group-section">
        <div class="group-header">
          <h2 class="group-title">{{ group.name }}</h2>
          <span class="group-count">{{ group.items.length }}</span>
        </div>
        <div class="group-list">
          <MonitorCard
            v-for="m in group.items"
            :key="m.name"
            :monitor="m"
            :pending="pendingNames.has(m.name)"
            @start="handleStart(m.name)"
            @stop="handleStop(m.name)"
            @edit="handleEdit(m.name)"
            @delete="deleteTarget = m.name"
            @view="handleView(m.name)"
          />
        </div>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { fetchMonitors, startMonitor, stopMonitor, deleteMonitor } from '../api/monitors'
import MonitorCard from '../components/MonitorCard.vue'
import { useToastMessages } from '../composables/useToastMessages'
import { useResource } from '../composables/useResource'

const router = useRouter()
const deleteTarget = ref(null)
const { successMsg, pageErrorMsg, showSuccess, showError } = useToastMessages()

const {
  data: monitors,
  loading,
  error,
  load: loadData,
  refresh,
} = useResource(fetchMonitors, { initial: [] })

// 目标级 pending：操作期间只禁用对应卡片的按钮，不触发全页 loading
const pendingNames = ref(new Set())

function setPending(name, on) {
  const next = new Set(pendingNames.value)
  if (on) next.add(name)
  else next.delete(name)
  pendingNames.value = next
}

const groupList = computed(() => {
  const map = {}
  for (const m of (monitors.value || [])) {
    const g = m.group || '默认'
    if (!map[g]) map[g] = { name: g, items: [] }
    map[g].items.push(m)
  }
  const keys = Object.keys(map).sort((a, b) => {
    if (a === '默认') return -1
    if (b === '默认') return 1
    return a.localeCompare(b, 'zh')
  })
  return keys.map(k => map[k])
})

onMounted(loadData)

async function toggleMonitor(name, start) {
  setPending(name, true)
  try {
    await (start ? startMonitor(name) : stopMonitor(name))
    showSuccess(start ? `「${name}」已启动` : `「${name}」已暂停`)
    // 本地立即更新状态，再后台校准
    const target = (monitors.value || []).find(m => m.name === name)
    if (target) target.is_running = start
    refresh()
  } catch (e) {
    showError((start ? '启动失败: ' : '暂停失败: ') + e.message)
  } finally {
    setPending(name, false)
  }
}

const handleStart = name => toggleMonitor(name, true)
const handleStop = name => toggleMonitor(name, false)

function handleEdit(name) {
  router.push(`/edit/${encodeURIComponent(name)}`)
}

async function handleDelete() {
  const name = deleteTarget.value
  deleteTarget.value = null
  setPending(name, true)
  try {
    await deleteMonitor(name)
    showSuccess(`「${name}」已删除`)
    // 本地移除，再后台校准
    monitors.value = (monitors.value || []).filter(m => m.name !== name)
    refresh()
  } catch (e) {
    showError('删除失败: ' + e.message)
  } finally {
    setPending(name, false)
  }
}

function handleView(name) {
  router.push(`/monitor/${encodeURIComponent(name)}`)
}
</script>

<style scoped>
.group-section {
  margin-bottom: 1.5rem;
}

.group-header {
  display: flex;
  align-items: baseline;
  gap: 0.65rem;
  margin-bottom: 0.75rem;
}

.group-title {
  font-size: 1rem;
  font-weight: 700;
  color: var(--text);
}

.group-count {
  font-size: 0.6875rem;
  font-weight: 700;
  color: var(--text-muted);
  background: var(--bg-elevated);
  padding: 0.1rem 0.5rem;
  border-radius: var(--radius-pill);
}

.group-list {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.empty-title {
  font-size: 1.125rem;
  font-weight: 700;
  color: var(--text);
  margin-top: 0.5rem;
}

.empty-desc {
  font-size: 0.8125rem;
  color: var(--text-secondary);
  margin-top: 0.25rem;
}

.empty-hints {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-top: 1rem;
  font-size: 0.75rem;
  color: var(--text-muted);
}

.hint-dot {
  width: 3px;
  height: 3px;
  border-radius: 50%;
  background: var(--text-muted);
  flex-shrink: 0;
}
</style>