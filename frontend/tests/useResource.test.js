import test from 'node:test'
import assert from 'node:assert/strict'

import { useResource } from '../src/composables/useResource.js'

function deferred() {
  let resolve, reject
  const promise = new Promise((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

test('load 成功后写入 data 并清空 error', async () => {
  const { data, loading, error, load } = useResource(async () => ({ code: 0, data: [1, 2] }))
  const p = load()
  assert.equal(loading.value, true)
  await p
  assert.deepEqual(data.value, [1, 2])
  assert.equal(loading.value, false)
  assert.equal(error.value, null)
})

test('load 失败置 error，不改动 data', async () => {
  const { data, error, load } = useResource(async () => { throw new Error('网络错误') }, { initial: [] })
  await load()
  assert.deepEqual(data.value, [])
  assert.equal(error.value, '网络错误')
})

test('refresh 失败时保留旧 data', async () => {
  let fail = false
  const { data, error, load, refresh, refreshing } = useResource(async () => {
    if (fail) throw new Error('刷新失败')
    return { code: 0, data: ['old'] }
  })
  await load()
  assert.deepEqual(data.value, ['old'])

  fail = true
  const p = refresh()
  assert.equal(refreshing.value, true)
  await p
  assert.deepEqual(data.value, ['old'])
  assert.equal(error.value, '刷新失败')
  assert.equal(refreshing.value, false)
})

test('防竞态：旧请求乱序返回不覆盖新请求结果', async () => {
  const first = deferred()
  const second = deferred()
  let call = 0
  const { data, load } = useResource(() => {
    call++
    return call === 1 ? first.promise : second.promise
  })

  const p1 = load()
  const p2 = load()
  // 新请求先返回，旧请求后返回
  second.resolve({ code: 0, data: 'new' })
  await p2
  first.resolve({ code: 0, data: 'stale' })
  await p1

  assert.equal(data.value, 'new')
})
