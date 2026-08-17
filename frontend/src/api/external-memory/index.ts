import { get, post } from '@/utils/request'
import { reviewListPath, type MemoryReviewStatus } from './presentation'

export interface AgentBinding {
  id: string
  team_id: string
  user_id: string
  agent_id: string
  external_agent: string
  connector_type: string
  status: 'active' | 'revoked' | string
  capture_enabled: boolean
  recall_enabled: boolean
  l3_wiki_enabled: boolean
  l3_review_required: boolean
  capability_scopes: string[]
  asset_scopes: string[]
  expires_at?: string
  last_used_at?: string
  created_at?: string
}

export interface CreateAgentBindingRequest {
  team_id: string
  user_id: string
  agent_id: string
  external_agent: string
  connector_type: string
  capability_scopes: string[]
  asset_scopes: string[]
  capture_enabled: boolean
  recall_enabled: boolean
  l3_wiki_enabled: boolean
  l3_review_required: boolean
}

export interface CreateAgentBindingResult {
  binding: AgentBinding
  connector_secret: string
}

export interface MemoryPublication {
  id: string
  title: string
  status: MemoryReviewStatus
  team_id: string
  agent_id: string
  memory_id: string
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
export const rotateAgentBindingKey = (id: string) => post<{ connector_secret: string }>(`/api/v1/agent-bindings/${encodeURIComponent(id)}/keys/rotate`)
export const listMemoryReviews = (status?: string) => get<MemoryPublication[]>(reviewListPath(status))
export const getMemoryReview = (id: string) => get<MemoryReviewDetail>(`/api/v1/external-memory/l3/reviews/${encodeURIComponent(id)}`)
export const reviewMemory = (id: string, action: 'approve' | 'reject' | 'request_changes', comment = '') =>
  post(`/api/v1/external-memory/l3/reviews/${encodeURIComponent(id)}/${action}`, { comment })
export const publishMemory = (id: string, knowledgeBaseId = '') =>
  post(`/api/v1/external-memory/l3/reviews/${encodeURIComponent(id)}/publish`, knowledgeBaseId ? { knowledge_base_id: knowledgeBaseId } : {})
