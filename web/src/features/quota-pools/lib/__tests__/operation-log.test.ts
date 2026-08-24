/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { expect, test } from 'vitest'

import { formatQuota } from '@/lib/format'

import type { QuotaPoolOperationLog } from '../../types'
import { renderQuotaPoolOperation } from '../operation-log'

const translate = (key: string, options: Record<string, unknown> = {}) =>
  Object.entries(options).reduce(
    (result, [name, value]) => result.replaceAll(`{{${name}}}`, String(value)),
    key
  )

const baseParams = {
  user_id: 25,
  user_name: '张三',
  quota_pool_id: 7,
  quota_pool_name: '平台保障部',
  target_pool_id: 9,
  target_pool_name: '默认额度池',
  amount: 250,
  fields: 3,
  enabled: true,
}

function operationLog(
  action: string,
  params: Record<string, unknown> = baseParams,
  content = action
): QuotaPoolOperationLog {
  return {
    id: 1,
    user_id: 8,
    username: 'admin-a',
    content,
    other: JSON.stringify({ op: { action, params } }),
    created_at: 1_700_000_000,
  }
}

test.each([
  [
    'quota_pool.create',
    `Created quota pool 平台保障部 with initial quota ${formatQuota(250)}`,
  ],
  ['quota_pool.sync_system', 'Synchronized system quota pools'],
  ['quota_pool.update', 'Updated 3 settings for quota pool 平台保障部'],
  ['quota_pool.enabled', 'Enabled quota pool 平台保障部'],
  ['quota_pool.delete', 'Deleted quota pool 平台保障部'],
  [
    'quota_pool.refill',
    `Added ${formatQuota(250)} temporary quota to 平台保障部`,
  ],
  ['quota_pool.self_update', 'Updated 3 auto-recharge settings for 平台保障部'],
  ['quota_pool.member_add', 'Added member 张三 (ID: 25) to 平台保障部'],
  ['quota_pool.member_move', 'Moved member 张三 (ID: 25) into 平台保障部'],
  [
    'quota_pool.member_remove',
    `Removed member 张三 (ID: 25) to 默认额度池 and reclaimed ${formatQuota(250)}`,
  ],
  [
    'quota_pool.member_recharge',
    `Recharged member 张三 (ID: 25) by ${formatQuota(250)}`,
  ],
  [
    'quota_pool.member_reclaim',
    `Reclaimed ${formatQuota(250)} from member 张三 (ID: 25)`,
  ],
  [
    'quota_pool.admin_grant',
    'Set member 张三 (ID: 25) as a pool administrator',
  ],
  [
    'quota_pool.admin_revoke',
    "Removed member 张三 (ID: 25)'s pool administrator role",
  ],
])('renders %s as readable text', (action, expected) => {
  expect(renderQuotaPoolOperation(operationLog(action), translate)).toBe(
    expected
  )
})

test('renders the disabled pool state', () => {
  expect(
    renderQuotaPoolOperation(
      operationLog('quota_pool.enabled', { ...baseParams, enabled: false }),
      translate
    )
  ).toBe('Disabled quota pool 平台保障部')
})

test('falls back to IDs when snapshot names are missing', () => {
  expect(
    renderQuotaPoolOperation(
      operationLog('quota_pool.member_remove', {
        user_id: 25,
        quota_pool_id: 7,
        target_pool_id: 9,
        amount: 250,
      }),
      translate
    )
  ).toBe(`Removed member #25 to #9 and reclaimed ${formatQuota(250)}`)
})

test.each([
  ['invalid JSON', '{', 'legacy content'],
  ['missing op', JSON.stringify({ admin_info: {} }), 'legacy content'],
  [
    'unknown action',
    JSON.stringify({ op: { action: 'quota_pool.unknown', params: {} } }),
    'legacy content',
  ],
])('falls back to content for %s', (_name, other, content) => {
  expect(
    renderQuotaPoolOperation(
      { ...operationLog('quota_pool.unknown'), other, content },
      translate
    )
  ).toBe(content)
})
