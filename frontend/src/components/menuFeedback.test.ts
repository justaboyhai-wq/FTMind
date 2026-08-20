import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

test('sidebar renders computed bottom menu items', async () => {
  const source = await readFile(new URL('./menu.vue', import.meta.url), 'utf8')
  assert.match(source, /v-for="\(item, index\) in bottomMenuItems"/)
})
