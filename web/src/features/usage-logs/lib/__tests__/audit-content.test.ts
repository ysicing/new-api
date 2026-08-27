/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { expect, test } from 'vitest'

import { formatQuota } from '@/lib/format'

import type { LogOtherData } from '../../types'
import { renderAuditContent } from '../format'

const translate = (key: string, options: Record<string, unknown> = {}) =>
  Object.entries(options).reduce(
    (result, [name, value]) => result.replaceAll(`{{${name}}}`, String(value)),
    key
  )

function auditOther(
  action: string,
  params: Record<string, string | number | boolean | string[]>
): LogOtherData {
  return { op: { action, params } }
}

test('renders quota pool member recharge as readable management text', () => {
  const text = renderAuditContent(
    auditOther('quota_pool.member_recharge', {
      user_id: 25,
      user_name: '张三',
      quota_pool_id: 7,
      quota_pool_name: '平台保障部',
      amount: 250,
    }),
    translate
  )

  expect(text).toBe(`Recharged member 张三 (ID: 25) by ${formatQuota(250)}`)
})

test('renders quota pool member add with pool and member names', () => {
  const text = renderAuditContent(
    auditOther('quota_pool.member_add', {
      user_id: 25,
      user_name: '张三',
      quota_pool_id: 7,
      quota_pool_name: '平台保障部',
    }),
    translate
  )

  expect(text).toBe('Added member 张三 (ID: 25) to 平台保障部')
})
