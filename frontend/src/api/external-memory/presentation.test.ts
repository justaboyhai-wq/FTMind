import test from 'node:test'
import assert from 'node:assert/strict'

import {
  allowedReviewActions,
  bindingAssetScopes,
  normalizeMemoryReviewDetail,
  reviewActionPath,
  reviewListPath,
} from './presentation'

test('binding assets always include the backend-required team scope', () => {
  assert.deepEqual(
    bindingAssetScopes(' team-one ', ['knowledge_base:kb-one', 'team:team-one', 'knowledge_base:kb-one']),
    ['knowledge_base:kb-one', 'team:team-one'],
  )
  assert.deepEqual(bindingAssetScopes('team-two', []), ['team:team-two'])
})

test('review list path only includes an explicit status filter', () => {
  assert.equal(reviewListPath(), '/api/v1/external-memory/l3/reviews')
  assert.equal(
    reviewListPath('pending_review'),
    '/api/v1/external-memory/l3/reviews?status=pending_review',
  )
})

test('review actions follow the guarded L3 lifecycle', () => {
  assert.deepEqual(allowedReviewActions('pending_review'), ['approve', 'reject', 'request_changes'])
  assert.deepEqual(allowedReviewActions('changes_requested'), [])
  assert.deepEqual(allowedReviewActions('approved'), ['publish'])
  assert.deepEqual(allowedReviewActions('publishing'), ['publish'])
  assert.deepEqual(allowedReviewActions('published'), [])
})

test('review decision path maps the state-machine action to the backend route', () => {
  assert.equal(
    reviewActionPath('publication/one', 'request_changes'),
    '/api/v1/external-memory/l3/reviews/publication%2Fone/request-changes',
  )
  assert.equal(
    reviewActionPath('publication-one', 'approve'),
    '/api/v1/external-memory/l3/reviews/publication-one/approve',
  )
})

test('review detail normalizes the current Go projection JSON fields', () => {
  const publication = {
    id: 'publication-one',
    title: 'Reviewed memory',
    status: 'pending_review' as const,
    team_id: 'team-one',
    agent_id: 'agent-one',
    memory_id: 'memory-one',
    markdown: '# Reviewed memory',
    evidence: ['conversation:one'],
  }
  const detail = normalizeMemoryReviewDetail({
    Publication: publication,
    ReviewTask: { id: 'review-one' },
    Snapshot: { id: 'snapshot-one' },
    Event: { id: 'event-one' },
  })

  assert.equal(detail.publication, publication)
  assert.deepEqual(detail.review_task, { id: 'review-one' })
  assert.deepEqual(detail.snapshot, { id: 'snapshot-one' })
  assert.deepEqual(detail.event, { id: 'event-one' })
})
