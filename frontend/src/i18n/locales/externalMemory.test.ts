import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import zhCN from './zh-CN'
import enUS from './en-US'

const requiredPaths = [
  'eyebrow', 'title', 'subtitle', 'refresh', 'scopeNote', 'memoryWikiBadge',
  'tabs.bindings', 'tabs.reviews',
  'overview.title', 'overview.desc', 'overview.activeBindings', 'overview.activeBindingsHint',
  'overview.revokedBindings', 'overview.revokedBindingsHint', 'overview.pendingReviews',
  'overview.pendingReviewsHint', 'overview.publishedReviews', 'overview.publishedReviewsHint',
  'overview.openBindings', 'overview.openReviews', 'overview.wikiHint',
  'fields.department', 'fields.selectDepartment', 'fields.selectTeam', 'fields.selectAgent',
  'fields.selectUser', 'fields.selectOrganizationFirst', 'fields.noTeams',
  'fields.organizationLoadFailed', 'fields.userApiKey', 'fields.userApiKeyHelp',
  'fields.selectUserApiKey', 'fields.unassignedKey', 'bindingError',
  'setupWizard.title', 'setupWizard.warning', 'setupWizard.expires', 'setupWizard.instructions',
  'setupWizard.copyPrompt', 'setupWizard.copied', 'status.pending_setup', 'detail.memoryId',
]

const connectorTypes = ['openclaw_plugin', 'hermes_provider', 'openai_proxy', 'anthropic_proxy', 'mcp', 'generic_sdk']
const capabilities = ['memory_context', 'memory_capture', 'memory_recall', 'memory_confirm', 'memory_publish', 'knowledge_search', 'wiki_get', 'document_read', 'context_assemble']

function valueAt(object: Record<string, any>, path: string) {
  return path.split('.').reduce((value, key) => value?.[key], object)
}

test('external memory page has complete Chinese and English translations', () => {
  for (const locale of [zhCN, enUS] as Array<Record<string, any>>) {
    assert.ok(locale.externalMemory)
    for (const path of requiredPaths) assert.notEqual(valueAt(locale.externalMemory, path), undefined, path)
    for (const key of connectorTypes) assert.notEqual(locale.externalMemory.connectorTypes?.[key], undefined, `connectorTypes.${key}`)
    for (const key of capabilities) assert.notEqual(locale.externalMemory.capabilities?.[key], undefined, `capabilities.${key}`)
  }
  assert.equal(zhCN.menu.externalMemory, '外部记忆')
  assert.equal(zhCN.externalMemory.detail.memoryId, '记忆编号')
})

test('external memory view delegates visible labels to i18n', () => {
  const source = readFileSync(resolve(import.meta.dirname, '../../views/memory/ExternalMemoryAdmin.vue'), 'utf8')
  assert.doesNotMatch(source, />L3 Wiki</)
  assert.doesNotMatch(source, />Memory ID</)
  assert.match(source, /externalMemory\.policy\.l3/)
  assert.match(source, /externalMemory\.detail\.memoryId/)
})
