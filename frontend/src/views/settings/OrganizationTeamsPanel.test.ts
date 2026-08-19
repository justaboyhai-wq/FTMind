import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import test from 'node:test'

const here = dirname(fileURLToPath(import.meta.url))
const orgSource = readFileSync(resolve(here, 'OrganizationTeamsPanel.vue'), 'utf8')
const bindingSource = readFileSync(resolve(here, '../memory/ExternalMemoryAdmin.vue'), 'utf8')

test('organization panel manages departments, teams, members, and agents', () => {
  assert.match(orgSource, /listDepartments\(\)/)
  assert.match(orgSource, /createDepartment\(/)
  assert.match(orgSource, /createTeam\(/)
  assert.match(orgSource, /addTeamMember\(/)
  assert.match(orgSource, /removeTeamMember\(/)
  assert.match(orgSource, /addTeamAgent\(/)
  assert.match(orgSource, /removeTeamAgent\(/)
  assert.match(orgSource, /agent\.is_builtin/)
  assert.match(orgSource, /builtin-/)
  assert.match(orgSource, /noBindableAgents/)
})

test('external binding uses organization selectors instead of free-form ids', () => {
  assert.match(bindingSource, /selectedDepartmentId/)
  assert.match(bindingSource, /teamOptions/)
  assert.match(bindingSource, /agentOptions/)
  assert.match(bindingSource, /userOptions/)
  assert.doesNotMatch(bindingSource, /FMind Agent ID.*t-input/)
  assert.doesNotMatch(bindingSource, /team ID.*t-input/)
})
