import test from 'node:test'
import assert from 'node:assert/strict'

import { allowedReviewActions, reviewListPath } from './presentation'

test('review list path only includes an explicit status filter', () => {
  assert.equal(reviewListPath(), '/api/v1/external-memory/l3/reviews')
  assert.equal(
    reviewListPath('pending_review'),
    '/api/v1/external-memory/l3/reviews?status=pending_review',
  )
})

test('review actions follow the guarded L3 lifecycle', () => {
  assert.deepEqual(allowedReviewActions('pending_review'), ['approve', 'reject', 'request_changes'])
  assert.deepEqual(allowedReviewActions('approved'), ['publish'])
  assert.deepEqual(allowedReviewActions('published'), [])
})
