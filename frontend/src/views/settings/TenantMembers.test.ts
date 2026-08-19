import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import test from 'node:test'

const here = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(here, 'TenantMembers.vue'), 'utf8')

test('member permissions popover renders backend-driven memory permissions', () => {
  assert.match(source, /authStore\.permissions\.role_matrix/)
  assert.match(source, /authStore\.refreshPermissions\(\)/)
  assert.match(source, /onMounted/)
  assert.match(source, /memoryPermissionDefinitions/)
  assert.match(source, /memory\.capture/)
  assert.match(source, /memory\.review/)
  assert.match(source, /memory\.publish/)
  assert.match(source, /wiki\.get/)
  assert.match(source, /memory-permissions-summary/)
})

test('member management exposes organization and permission sections', () => {
  assert.match(source, /tenantMember\.tabs\.organization/)
  assert.match(source, /tenantMember\.tabs\.permissions/)
  assert.match(source, /OrganizationTeamsPanel/)
  assert.match(source, /activeMemberSection/)
})
