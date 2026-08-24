<template>
  <div class="update-panel">
    <button class="version-btn" :disabled="checking || state === 'downloading' || state === 'restarting'" @click="handleClick">
      <span v-if="state === 'idle'">{{ version }}</span>
      <span v-else-if="state === 'checking'">检查中...</span>
      <span v-else-if="state === 'uptodate'">已是最新</span>
      <span v-else-if="state === 'error'">检查失败</span>
      <span v-else-if="state === 'downloading'">下载更新中...</span>
      <span v-else-if="state === 'restarting'">即将重启...</span>
      <span v-else-if="state === 'failed'">{{ errorMsg || '更新失败' }}</span>
    </button>

    <div class="update-actions" v-if="state === 'available'">
      <button class="update-btn" @click="handleUpdate">升级到 {{ latestVersion }}</button>
    </div>
    <div class="update-actions" v-if="state === 'downloading'">
      <div class="progress-bar"><div class="progress-fill fill-anim" /></div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { fetchVersion, checkUpdate, applyUpdate, fetchUpdateStatus } from '../api/monitors'

const version = ref('')
const state = ref('idle')
const latestVersion = ref('')
const downloadURL = ref('')
const errorMsg = ref('')
let statusTimer = null

onMounted(async () => {
  try {
    const res = await fetchVersion()
    if (res.code === 0 && res.data) {
      version.value = res.data.version
    }
  } catch {}
})

onUnmounted(() => {
  if (statusTimer) clearInterval(statusTimer)
})

async function handleClick() {
  state.value = 'checking'
  try {
    const res = await checkUpdate()
    if (res.code === 0 && res.data) {
      if (res.data.has_update) {
        latestVersion.value = res.data.latest_version
        downloadURL.value = res.data.download_url
        state.value = 'available'
      } else {
        state.value = 'uptodate'
        setTimeout(() => { state.value = 'idle' }, 2000)
      }
    } else {
      state.value = 'error'
      setTimeout(() => { state.value = 'idle' }, 2000)
    }
  } catch {
    state.value = 'error'
    setTimeout(() => { state.value = 'idle' }, 2000)
  }
}

async function handleUpdate() {
  if (!downloadURL.value) return
  state.value = 'downloading'
  try {
    await applyUpdate()
    pollStatus()
  } catch {
    state.value = 'failed'
    errorMsg.value = '请求失败'
    setTimeout(() => { state.value = 'idle' }, 3000)
  }
}

function pollStatus() {
  statusTimer = setInterval(async () => {
    try {
      const res = await fetchUpdateStatus()
      if (res.code === 0 && res.data) {
        if (res.data.status === 'done') {
          state.value = 'restarting'
          clearInterval(statusTimer)
        } else if (res.data.status === 'error') {
          errorMsg.value = res.data.message || '更新失败'
          state.value = 'failed'
          clearInterval(statusTimer)
          setTimeout(() => { state.value = 'idle' }, 5000)
        }
      }
    } catch {
      state.value = 'restarting'
      clearInterval(statusTimer)
    }
  }, 2000)
}
</script>

<style scoped>
.update-panel {
  border-top: 1px solid var(--border-light);
  padding: 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.version-btn {
  width: 100%;
  padding: 0.4rem;
  border: none;
  border-radius: var(--radius);
  background: var(--bg-elevated);
  color: var(--text-muted);
  font-size: 0.6875rem;
  font-weight: 600;
  cursor: pointer;
  transition: var(--transition);
  text-align: center;
}
.version-btn:hover { color: var(--text); background: var(--bg-hover); }
.version-btn:disabled { opacity: 0.7; cursor: default; }

.update-btn {
  width: 100%;
  padding: 0.4rem;
  border: none;
  border-radius: var(--radius);
  background: var(--green);
  color: #000;
  font-size: 0.75rem;
  font-weight: 700;
  cursor: pointer;
  transition: var(--transition);
}
.update-btn:hover { opacity: 0.9; }
.update-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.progress-bar {
  height: 3px;
  background: var(--bg-elevated);
  border-radius: 2px;
  overflow: hidden;
  margin-top: 0.25rem;
}
.progress-fill {
  height: 100%;
  background: var(--green);
  border-radius: 2px;
}
.fill-anim {
  width: 60%;
  animation: progress-indeterminate 1.5s ease-in-out infinite;
}
@keyframes progress-indeterminate {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(200%); }
}
</style>
