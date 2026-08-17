export type MemoryReviewStatus =
  | 'pending_review'
  | 'changes_requested'
  | 'approved'
  | 'publishing'
  | 'published'
  | 'rejected'
  | 'revoked'

export type MemoryReviewAction = 'approve' | 'reject' | 'request_changes' | 'publish'

export interface RawMemoryReviewDetail<TPublication = Record<string, unknown>> {
  Publication?: TPublication
  ReviewTask?: Record<string, unknown>
  Snapshot?: Record<string, unknown>
  Event?: Record<string, unknown>
  publication?: TPublication
  review_task?: Record<string, unknown>
  snapshot?: Record<string, unknown>
  event?: Record<string, unknown>
}

export interface NormalizedMemoryReviewDetail<TPublication = Record<string, unknown>> {
  publication: TPublication
  review_task?: Record<string, unknown>
  snapshot?: Record<string, unknown>
  event?: Record<string, unknown>
}

export function reviewListPath(status?: string): string {
  return status ? `/api/v1/external-memory/l3/reviews?status=${encodeURIComponent(status)}` : '/api/v1/external-memory/l3/reviews'
}

export function bindingAssetScopes(teamId: string, scopes: string[]): string[] {
  const normalized = scopes.map(scope => scope.trim()).filter(Boolean)
  normalized.push(`team:${teamId.trim()}`)
  return [...new Set(normalized)]
}

export function reviewActionPath(id: string, action: Exclude<MemoryReviewAction, 'publish'>): string {
  const routeAction = action === 'request_changes' ? 'request-changes' : action
  return `/api/v1/external-memory/l3/reviews/${encodeURIComponent(id)}/${routeAction}`
}

export function normalizeMemoryReviewDetail<TPublication>(
  detail: RawMemoryReviewDetail<TPublication>,
): NormalizedMemoryReviewDetail<TPublication> {
  const publication = detail.publication ?? detail.Publication
  if (!publication) throw new Error('memory review response is missing Publication')
  return {
    publication,
    review_task: detail.review_task ?? detail.ReviewTask,
    snapshot: detail.snapshot ?? detail.Snapshot,
    event: detail.event ?? detail.Event,
  }
}

export function allowedReviewActions(status: string): MemoryReviewAction[] {
  if (status === 'pending_review') return ['approve', 'reject', 'request_changes']
  if (status === 'approved' || status === 'publishing') return ['publish']
  return []
}
