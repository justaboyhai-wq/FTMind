import test from 'node:test'
import assert from 'node:assert/strict'
import zhCN from './zh-CN'
import enUS from './en-US'

const requiredKeys = [
  'adminMenu', 'myFeedback', 'adminTitle', 'adminDescription', 'adminDetail',
  'exportCsv', 'status', 'categoryLabel', 'descriptionLabel', 'createdAt',
  'filterStatus', 'timeline', 'saveProcess', 'saved', 'title', 'detailTitle',
  'button', 'submit', 'submitted', 'submitFailed', 'commentPlaceholder',
  'addComment', 'commentAdded', 'reopen', 'reopenDefault', 'adminReply',
  'expectedLabel', 'quotedLabel', 'descriptionPlaceholder', 'descriptionTooShort',
]

test('feedback translations expose every visible label in both locales', () => {
  for (const locale of [zhCN, enUS] as any[]) {
    assert.ok(locale.feedback)
    for (const key of requiredKeys) assert.notEqual(locale.feedback[key], undefined, key)
    for (const key of ['wrong_fact', 'outdated', 'citation_mismatch', 'incomplete', 'misunderstood', 'unsafe', 'other']) {
      assert.ok(locale.feedback.categories[key])
    }
    for (const key of ['pending', 'reviewing', 'needs_info', 'fixing', 'resolved', 'dismissed']) {
      assert.ok(locale.feedback.statuses[key])
    }
  }
  assert.equal(zhCN.feedback.adminMenu, '反馈管理')
  assert.equal(zhCN.feedback.myFeedback, '我的反馈')
})
