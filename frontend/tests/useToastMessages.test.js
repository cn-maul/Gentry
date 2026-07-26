import test from 'node:test'
import assert from 'node:assert/strict'

import { useToastMessages } from '../src/composables/useToastMessages.js'

// useToastMessages 在组件上下文外调用时 onUnmounted 会打印警告，不影响行为。

test('success 与 error 使用独立 timer：后触发的 error 不清除 success 的定时', (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] })
  const { successMsg, pageErrorMsg, showSuccess, showError } = useToastMessages()

  showSuccess('已保存')
  assert.equal(successMsg.value, '已保存')

  // 展示 error 会清空 success（避免重叠）
  showError('失败了')
  assert.equal(successMsg.value, '')
  assert.equal(pageErrorMsg.value, '失败了')

  // error 定时 5 秒后消失
  t.mock.timers.tick(5000)
  assert.equal(pageErrorMsg.value, '')
})

test('error 展示中触发 success 会立即清掉 error', (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] })
  const { successMsg, pageErrorMsg, showSuccess, showError } = useToastMessages()

  showError('失败了')
  showSuccess('成功')
  assert.equal(pageErrorMsg.value, '')
  assert.equal(successMsg.value, '成功')

  // success 3 秒后消失，且不会因 error 的残留 timer 提前消失
  t.mock.timers.tick(2999)
  assert.equal(successMsg.value, '成功')
  t.mock.timers.tick(1)
  assert.equal(successMsg.value, '')
})

test('连续两次 success 重置定时', (t) => {
  t.mock.timers.enable({ apis: ['setTimeout'] })
  const { successMsg, showSuccess } = useToastMessages()

  showSuccess('第一条')
  t.mock.timers.tick(2000)
  showSuccess('第二条')
  t.mock.timers.tick(2000)
  // 距第二条仅 2 秒，应仍在展示
  assert.equal(successMsg.value, '第二条')
  t.mock.timers.tick(1000)
  assert.equal(successMsg.value, '')
})
