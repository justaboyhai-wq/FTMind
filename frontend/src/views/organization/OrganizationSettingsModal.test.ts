import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const componentPath = fileURLToPath(new URL('./OrganizationSettingsModal.vue', import.meta.url))
const source = readFileSync(componentPath, 'utf8')

test('member permission dialog surfaces actual memory permissions before role cards', () => {
  const summary = source.indexOf('class="memory-permissions-summary"')
  const roleGrid = source.indexOf('class="permissions-compact-grid permissions-role-grid"')

  assert.notEqual(summary, -1, 'actual memory permission summary must be rendered')
  assert.notEqual(roleGrid, -1, 'role permission matrix must be rendered')
  assert.ok(summary < roleGrid, 'memory permissions must be visible before the role-card grid')
})

test('member permission dialog renders memory capability rows from backend role matrix', () => {
  assert.match(source, /v-for="item in role\.memory"/)
  assert.match(source, /memory\.capture/)
  assert.match(source, /memory\.review/)
  assert.match(source, /memory\.publish/)
  assert.match(source, /wiki\.get/)
})
