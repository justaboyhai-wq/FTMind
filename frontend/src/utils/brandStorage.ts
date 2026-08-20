/**
 * Brand-key migration helpers.
 *
 * The application originally stored per-user/session state under keys prefixed
 * with the legacy brand name. After the rebrand to FTMind we write new state
 * with the `ftmind_` / `FTMind_` prefix while transparently migrating any
 * legacy values on first access.
 */

const LEGACY_TO_NEW: Record<string, string> = {
  fmind_token: 'ftmind_token',
  fmind_refresh_token: 'ftmind_refresh_token',
  fmind_tenant: 'ftmind_tenant',
  fmind_selected_tenant_id: 'ftmind_selected_tenant_id',
  fmind_selected_tenant_name: 'ftmind_selected_tenant_name',
  fmind_user: 'ftmind_user',
  fmind_knowledge_bases: 'ftmind_knowledge_bases',
  fmind_current_kb: 'ftmind_current_kb',
  fmind_last_chat_model_id: 'ftmind_last_chat_model_id',
  fmind_memberships: 'ftmind_memberships',
  fmind_lite_mode: 'ftmind_lite_mode',
  fmind_lite_last_path: 'ftmind_lite_last_path',
  fmind_auto_setup_failed: 'ftmind_auto_setup_failed',
  fmind_cmdk_recent: 'ftmind_cmdk_recent',
  fmind_pending_tenant_switch_toast: 'ftmind_pending_tenant_switch_toast',
  FMind_theme: 'FTMind_theme',
  FMind_settings: 'FTMind_settings',
}

/**
 * Migrate legacy brand-prefixed local/session storage keys to their FTMind
 * equivalents. Existing new keys take precedence so that a user who has
 * already upgraded is not logged out or reset.
 */
export function migrateLegacyStorage(storage: Storage): void {
  for (const [legacyKey, newKey] of Object.entries(LEGACY_TO_NEW)) {
    const existingNew = storage.getItem(newKey)
    const legacyValue = storage.getItem(legacyKey)
    if (existingNew === null && legacyValue !== null) {
      storage.setItem(newKey, legacyValue)
      storage.removeItem(legacyKey)
    }
  }
}

/**
 * Read a storage value, preferring the new FTMind key and falling back to the
 * legacy brand key. Returns null when neither exists.
 */
export function readStorageWithCompat(storage: Storage, newKey: string, legacyKey: string): string | null {
  return storage.getItem(newKey) ?? storage.getItem(legacyKey)
}
