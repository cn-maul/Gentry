<template>
  <div class="page settings-page">
    <header class="page-header">
      <h1>设置</h1>
    </header>

    <section class="settings-section">
      <div class="section-header">
        <h2>更新代理</h2>
        <p class="section-desc">如果下载更新时连接 GitHub 失败，可以设置代理地址</p>
      </div>

      <div class="form-group">
        <label>代理地址</label>
        <div class="proxy-row">
          <input v-model="proxy" class="form-input" placeholder="http://127.0.0.1:7897" />
          <button class="btn btn-primary" :disabled="saving" @click="handleSave">{{ saving ? '保存中...' : '保存' }}</button>
        </div>
        <p class="hint" v-if="saved">已保存</p>
        <p class="hint error-hint" v-if="errorMsg">{{ errorMsg }}</p>
      </div>
    </section>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { fetchUpdateProxy, setUpdateProxy } from '../api/monitors'

const proxy = ref('')
const saving = ref(false)
const saved = ref(false)
const errorMsg = ref('')

onMounted(async () => {
  try {
    const res = await fetchUpdateProxy()
    if (res.code === 0 && res.data?.proxy) {
      proxy.value = res.data.proxy
    }
  } catch {}
})

async function handleSave() {
  saving.value = true
  saved.value = false
  errorMsg.value = ''
  try {
    await setUpdateProxy(proxy.value)
    saved.value = true
    setTimeout(() => { saved.value = false }, 2000)
  } catch (error) {
    errorMsg.value = error.response?.data?.message || '保存失败，请检查代理地址'
  }
  saving.value = false
}
</script>

<style scoped>
.settings-page { max-width: 640px; }

.page-header { margin-bottom: 1.5rem; }
.page-header h1 { font-size: 1.5rem; font-weight: 700; color: var(--text); }

.settings-section { margin-bottom: 1.5rem; }
.section-header { margin-bottom: 1rem; }
.section-header h2 { font-size: 1rem; font-weight: 700; color: var(--text); }
.section-desc { font-size: 0.8125rem; color: var(--text-secondary); margin-top: 0.25rem; }

.proxy-row { display: flex; gap: 0.5rem; }
.proxy-row .form-input { flex: 1; }

.hint { font-size: 0.75rem; color: var(--green); margin-top: 0.35rem; }
.error-hint { color: var(--error); }
</style>
