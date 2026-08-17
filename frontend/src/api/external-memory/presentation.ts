export type MemoryReviewStatus =
  | 'pending_review'
  | 'changes_requested'
  | 'approved'
  | 'publishing'
  | 'published'
  | 'rejected'
  | 'revoked'

export type MemoryReviewAction = 'approve' | 'reject' | 'request_changes' | 'publish'

export function reviewListPath(status?: string): string {
  return status ? `/api/v1/external-memory/l3/reviews?status=${encodeURIComponent(status)}` : '/api/v1/external-memory/l3/reviews'
}

export function allowedReviewActions(status: string): MemoryReviewAction[] {
  if (status === 'pending_review' || status === 'changes_requested') return ['approve', 'reject', 'request_changes']
  if (status === 'approved') return ['publish']
  return []
}
