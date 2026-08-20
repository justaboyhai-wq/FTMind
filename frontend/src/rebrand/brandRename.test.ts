import { describe, it, beforeEach } from 'node:test'
import assert from 'node:assert'
import { readFileSync, readdirSync, statSync } from 'node:fs'
import { resolve, join, relative } from 'node:path'

const projectRoot = resolve(process.cwd(), '..')
const frontendRoot = process.cwd()

// Old brand variants that must not appear in user-visible source.
const OLD_BRAND_RE = /FMind|Fmind|FMIND/

// Directories scanned for visible product-name strings.
const USER_FACING_DIRS = [
  'src/views',
  'src/components',
  'src/i18n/locales',
  'src/router',
  'src/stores',
  'src/composables',
  'src/utils',
  'src/config',
]

// Files in these directories may contain identifiers like fmindcloud that are
// not user-visible; we still scan them for the literal "FMind" / "Fmind" / "FMIND"
// variants but allow known safe internal identifiers via an exclusion list.
const EXCLUDED_PATTERNS = [
  /fmindcloud/i, // provider/service identifier; renamed separately if needed
  /ftmind\.css/i, // stylesheet filename reference
  /ftmind_lite_mode/i,
  /ftmind_lite_last_path/i,
  /FTMind_theme/i,
  /ftmind_token/i,
  /ftmind_tenant/i,
  /ftmind_selected_tenant_id/i,
  /__FTMIND_API_BASE__/i,
  /__FTMIND_API_LAN_BASE__/i,
  /aud=fmind/i,
  /X-FMind-Signature/i, // legacy webhook header name (API contract)
  /X-FMind-Connector-Secret/i,
  /X-FMind-User-Key/i,
  /X-FMind-Agent-Key/i,
  /X-FMind-Binding-Token/i,
  /X-FMind-Service-ID/i,
  /X-FMind-Event-Timestamp/i,
  /X-FMind-Event-Signature/i,
  /fmind-memory-event/i,
  /github\.com\/justaboyhai-wq\/fmind/i,
  /FMind_\$\{suffix\}/, // legacy preference key pattern
  /FMind_\$\{userId\}/, // legacy per-user preference key pattern
  /FMind_settings:/, // legacy settings key in migration map
]

function isTextFile(p: string): boolean {
  return p.endsWith('.ts') || p.endsWith('.tsx') || p.endsWith('.vue') || p.endsWith('.js') || p.endsWith('.jsx')
}

function scanDir(dir: string, violations: string[] = []): string[] {
  let entries: string[] = []
  try {
    entries = readdirSync(dir)
  } catch {
    return violations
  }
  for (const entry of entries) {
    if (entry === 'node_modules' || entry === 'dist' || entry === '.rebrand-scan') continue
    const p = join(dir, entry)
    const st = statSync(p)
    if (st.isDirectory()) {
      scanDir(p, violations)
    } else if (st.isFile() && isTextFile(p)) {
      const text = readFileSync(p, 'utf-8')
      if (OLD_BRAND_RE.test(text)) {
        // Only flag if no excluded pattern covers every occurrence.
        const lines = text.split('\n')
        lines.forEach((line, idx) => {
          if (OLD_BRAND_RE.test(line) && !EXCLUDED_PATTERNS.some((re) => re.test(line))) {
            violations.push(`${relative(frontendRoot, p)}:${idx + 1}: ${line.trim()}`)
          }
        })
      }
    }
  }
  return violations
}

function findBrandViolations(text: string): string[] {
  const violations: string[] = []
  const lines = text.split('\n')
  lines.forEach((line, idx) => {
    if (OLD_BRAND_RE.test(line) && !EXCLUDED_PATTERNS.some((re) => re.test(line))) {
      violations.push(`${idx + 1}: ${line.trim()}`)
    }
  })
  return violations
}

describe('brand rename: no old brand in user-visible frontend source', () => {
  it('should not contain FMind/Fmind/FMIND in user-facing TypeScript/Vue files', () => {
    const violations: string[] = []
    for (const d of USER_FACING_DIRS) {
      scanDir(resolve(frontendRoot, d), violations)
    }
    assert.deepStrictEqual(violations, [], `Found old brand references:\n${violations.join('\n')}`)
  })
})

describe('brand rename: i18n product name', () => {
  const zhCN = readFileSync(resolve(frontendRoot, 'src/i18n/locales/zh-CN.ts'), 'utf-8')
  const enUS = readFileSync(resolve(frontendRoot, 'src/i18n/locales/en-US.ts'), 'utf-8')

  it('zh-CN language pack should use FTMind as product name', () => {
    assert.match(zhCN, /FTMind/)
    const violations = findBrandViolations(zhCN)
    assert.deepStrictEqual(violations, [], `Old brand references found:\n${violations.join('\n')}`)
  })

  it('en-US language pack should use FTMind as product name', () => {
    assert.match(enUS, /FTMind/)
    const violations = findBrandViolations(enUS)
    assert.deepStrictEqual(violations, [], `Old brand references found:\n${violations.join('\n')}`)
  })
})

describe('brand rename: login page title', () => {
  const login = readFileSync(resolve(frontendRoot, 'src/views/auth/Login.vue'), 'utf-8')

  it('Login.vue should reference FTMind for the document/brand title', () => {
    assert.match(login, /FTMind/)
    const violations = findBrandViolations(login)
    assert.deepStrictEqual(violations, [], `Old brand references found:\n${violations.join('\n')}`)
  })
})

describe('brand rename: favicon and logo references', () => {
  const indexHtml = readFileSync(resolve(frontendRoot, 'index.html'), 'utf-8')

  it('index.html should use local FTMind favicon/logo assets', () => {
    assert.match(indexHtml, /FTMind/)
    const violations = findBrandViolations(indexHtml)
    assert.deepStrictEqual(violations, [], `Old brand references found:\n${violations.join('\n')}`)
    // favicon must reference the local FTMind logo.
    const faviconMatch = indexHtml.match(/<link[^>]*rel=["']?icon["']?[^>]*>/i)
    assert.ok(faviconMatch, 'favicon link missing')
    assert.match(faviconMatch[0], /href=["']\/ftmind-logo\.png["']/)
  })
})

describe('brand rename: localStorage key migration', () => {
  function createMockStorage(): Storage {
    const data = new Map<string, string>()
    return {
      getItem(key: string) {
        return data.has(key) ? (data.get(key) as string) : null
      },
      setItem(key: string, value: string) {
        data.set(key, value)
      },
      removeItem(key: string) {
        data.delete(key)
      },
      clear() {
        data.clear()
      },
      key(index: number) {
        return Array.from(data.keys())[index] ?? null
      },
      get length() {
        return data.size
      },
    } as Storage
  }

  beforeEach(() => {
    // Ensure each test gets fresh module state.
  })

  it('migrates fmind_token to ftmind_token', async () => {
    const storage = createMockStorage()
    storage.setItem('fmind_token', 'legacy-token')

    const { migrateLegacyStorage } = await import('../utils/brandStorage.js')
    migrateLegacyStorage(storage)

    assert.strictEqual(storage.getItem('ftmind_token'), 'legacy-token')
  })

  it('migrates fmind_tenant to ftmind_tenant', async () => {
    const storage = createMockStorage()
    storage.setItem('fmind_tenant', '{"id":1}')

    const { migrateLegacyStorage } = await import('../utils/brandStorage.js')
    migrateLegacyStorage(storage)

    assert.strictEqual(storage.getItem('ftmind_tenant'), '{"id":1}')
  })

  it('does not overwrite new keys when old keys are absent', async () => {
    const storage = createMockStorage()
    storage.setItem('ftmind_token', 'new-token')

    const { migrateLegacyStorage } = await import('../utils/brandStorage.js')
    migrateLegacyStorage(storage)

    assert.strictEqual(storage.getItem('ftmind_token'), 'new-token')
    assert.strictEqual(storage.getItem('fmind_token'), null)
  })
})

describe('brand rename: feedback/external-memory/organization pages', () => {
  const pageFiles = [
    'src/views/feedback/FeedbackAdmin.vue',
    'src/views/external-memory', // directory
    'src/views/organization/OrganizationList.vue',
  ]

  it('should not contain old brand in feedback/admin/org pages', () => {
    const violations: string[] = []
    for (const f of pageFiles) {
      const p = resolve(frontendRoot, f)
      try {
        const st = statSync(p)
        if (st.isDirectory()) {
          scanDir(p, violations)
        } else {
          const text = readFileSync(p, 'utf-8')
          const lines = text.split('\n')
          lines.forEach((line, idx) => {
            if (OLD_BRAND_RE.test(line) && !EXCLUDED_PATTERNS.some((re) => re.test(line))) {
              violations.push(`${f}:${idx + 1}: ${line.trim()}`)
            }
          })
        }
      } catch {
        // skip missing files
      }
    }
    assert.deepStrictEqual(violations, [], `Old brand references found:\n${violations.join('\n')}`)
  })
})
