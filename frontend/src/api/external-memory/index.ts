import { get, post } from '@/utils/request'
import {
  normalizeMemoryReviewDetail,
  reviewActionPath,
  reviewListPath,
  type MemoryReviewStatus,
  type RawMemoryReviewDetail,
} from './presentation'

export interface AgentBinding {
  id: string
  tenant_id: number
  department_id?: string
  team_id: string
  workspace_id?: string
  project_id?: string
  user_id: string
  agent_id: string
  task_id?: string
  external_agent: string
  connector_type: string
  status: 'pending_setup' | 'active' | 'revoked' | string
  setup_expires_at?: string
  activated_at?: string
  last_handshake_at?: string
  setup_attempts?: number
  capture_enabled: boolean
  recall_enabled: boolean
  l3_wiki_enabled: boolean
  l3_review_required: boolean
  capability_scopes: string[]
  asset_scopes: string[]
  policy_version: number
  created_by?: string
  expires_at?: string
  last_used_at?: string
  created_at?: string
  updated_at?: string
}

export interface CreateAgentBindingRequest {
  user_api_key_id: number
  department_id?: string
  team_id: string
  workspace_id?: string
  project_id?: string
  user_id: string
  agent_id: string
  task_id?: string
  external_agent: string
  connector_type: string
  capability_scopes: string[]
  asset_scopes: string[]
  capture_enabled: boolean
  recall_enabled: boolean
  l3_wiki_enabled: boolean
  l3_review_required: boolean
  expires_at?: string
}

export interface CreateAgentBindingResult {
  binding: AgentBinding
  connector_secret: string
  credential_purpose?: 'memory_binding_setup' | 'memory_binding_runtime' | string
  setup_expires_at?: string
  setup_manifest?: {
    binding_id: string
    external_agent: string
    connector_type: string
    fmind_endpoint: string
    memory_core_endpoint?: string
    memory_proxy_endpoint?: string
    capabilities: string[]
    asset_scopes: string[]
  }
  setup_prompt?: string
}

export interface MemoryPublication {
  id: string
  snapshot_id?: string
  review_task_id?: string
  event_id?: string
  tenant_id?: number
  title: string
  status: MemoryReviewStatus
  team_id: string
  binding_id?: string
  user_id?: string
  department_id?: string
  workspace_id?: string
  project_id?: string
  agent_id: string
  task_id?: string
  memory_id: string
  memory_version?: number
  markdown: string
  evidence: string[]
  review_comment?: string
  reviewed_by?: string
  knowledge_base_id?: string
  published_page_id?: string
  created_at?: string
  updated_at?: string
}

// The list endpoint returns compact publication records. The detail endpoint
// deliberately returns the full L3 projection so that review evidence keeps
// its immutable source lineage; consumers should use `publication` for the
// governed Markdown artifact.
export interface MemoryReviewDetail {
  publication: MemoryPublication
  review_task?: Record<string, unknown>
  snapshot?: Record<string, unknown>
  event?: Record<string, unknown>
}

export const listAgentBindings = () => get<AgentBinding[]>('/api/v1/agent-bindings')
export const createAgentBinding = (payload: CreateAgentBindingRequest) => post<CreateAgentBindingResult>('/api/v1/agent-bindings', payload)
export const revokeAgentBinding = (id: string) => post<void>(`/api/v1/agent-bindings/${encodeURIComponent(id)}/revoke`)
export const rotateAgentBindingKey = (id: string) => post<Pick<CreateAgentBindingResult, 'connector_secret' | 'credential_purpose' | 'setup_prompt' | 'setup_expires_at'>>(`/api/v1/agent-bindings/${encodeURIComponent(id)}/keys/rotate`)
export const getAgentBindingSetupStatus = (id: string) => get<{ binding_id: string; status: string; setup_expires_at?: string; activated_at?: string; last_handshake_at?: string; setup_attempts?: number }>(`/api/v1/agent-bindings/${encodeURIComponent(id)}/setup-status`)
export const listMemoryReviews = (status?: string) => get<MemoryPublication[]>(reviewListPath(status))
export const getMemoryReview = async (id: string): Promise<MemoryReviewDetail> => {
  const detail = await get<RawMemoryReviewDetail<MemoryPublication>>(`/api/v1/external-memory/l3/reviews/${encodeURIComponent(id)}`)
  return normalizeMemoryReviewDetail(detail)
}
export const reviewMemory = (id: string, action: 'approve' | 'reject' | 'request_changes', comment = '') =>
  post(reviewActionPath(id, action), { comment })
export const publishMemory = (id: string, knowledgeBaseId = '') =>
  post(`/api/v1/external-memory/l3/reviews/${encodeURIComponent(id)}/publish`, knowledgeBaseId ? { knowledge_base_id: knowledgeBaseId } : {})
export const revokeMemory = (id: string, comment = '') =>
  post(`/api/v1/external-memory/l3/reviews/${encodeURIComponent(id)}/revoke`, comment ? { comment } : {})
