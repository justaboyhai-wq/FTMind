export interface SetupCredentialLike {
  status?: string
  credential_purpose?: string
}

export function isPendingSetup(value: SetupCredentialLike): boolean {
  return value.status === 'pending_setup' && value.credential_purpose === 'memory_binding_setup'
}

export function setupCredentialLabel(purpose?: string): string {
  return purpose || 'memory_binding_runtime'
}
