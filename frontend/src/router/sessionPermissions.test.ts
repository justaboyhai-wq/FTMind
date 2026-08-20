import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const source = readFileSync(resolve(import.meta.dirname, 'index.ts'), 'utf8')

test('restoring a browser session refreshes the permission snapshot', () => {
  const start = source.indexOf('async function hydrateSessionFromToken')
  const end = source.indexOf('\nlet autoSetupAttempted', start)
  const hydrateSession = source.slice(start, end)

  assert.match(hydrateSession, /await authStore\.refreshPermissions\(\)/)
})
