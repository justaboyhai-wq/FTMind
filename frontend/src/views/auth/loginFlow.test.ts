import test from 'node:test'
import assert from 'node:assert/strict'
import { withTimeout } from './loginFlow'

test('withTimeout rejects a post-login operation that never settles', async () => {
  await assert.rejects(
    withTimeout(new Promise(() => {}), 5, '登录后初始化超时'),
    /登录后初始化超时/,
  )
})
