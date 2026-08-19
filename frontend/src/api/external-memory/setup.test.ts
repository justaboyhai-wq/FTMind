import test from 'node:test'
import assert from 'node:assert/strict'

import { isPendingSetup, setupCredentialLabel } from './setup'

test('setup flow distinguishes one-time pending credentials from runtime credentials', () => {
  assert.equal(isPendingSetup({ status: 'pending_setup', credential_purpose: 'memory_binding_setup' }), true)
  assert.equal(isPendingSetup({ status: 'active', credential_purpose: 'memory_binding_runtime' }), false)
  assert.equal(setupCredentialLabel('memory_binding_setup'), 'memory_binding_setup')
})
