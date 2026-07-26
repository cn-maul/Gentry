import { ref, onUnmounted } from 'vue'

export function useToastMessages() {
  const successMsg = ref('')
  const pageErrorMsg = ref('')
  let successTimer = null
  let errorTimer = null

  onUnmounted(() => {
    clearTimeout(successTimer)
    clearTimeout(errorTimer)
  })

  function showSuccess(msg) {
    // 清掉正在展示的错误，避免两条 toast 重叠
    clearTimeout(errorTimer)
    pageErrorMsg.value = ''
    successMsg.value = msg
    clearTimeout(successTimer)
    successTimer = setTimeout(() => { successMsg.value = '' }, 3000)
  }

  function showError(msg) {
    clearTimeout(successTimer)
    successMsg.value = ''
    pageErrorMsg.value = msg
    clearTimeout(errorTimer)
    errorTimer = setTimeout(() => { pageErrorMsg.value = '' }, 5000)
  }

  return { successMsg, pageErrorMsg, showSuccess, showError }
}
