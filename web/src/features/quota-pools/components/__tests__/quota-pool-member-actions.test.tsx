/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { render, screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import type {
  QuotaPool,
  QuotaPoolCapabilities,
  QuotaPoolMember,
} from '../../types'
import { QuotaPoolMemberActions } from '../quota-pool-member-actions'

const pool: QuotaPool = {
  id: 7,
  name: '默认额度池',
  pool_type: 'new_user',
  enabled: true,
  is_default: true,
  base_quota: -1,
  quota: -1,
  auto_recharge_amount: 0,
  weekly_limit: 0,
  monthly_limit: 0,
  monthly_refill_enabled: false,
  monthly_refill_top_up: false,
  monthly_refill_amount: 0,
  monthly_refill_day: 1,
  last_refill_month: 0,
}

const capabilities: QuotaPoolCapabilities = {
  can_view: true,
  can_edit: false,
  can_edit_monthly_refill: false,
  can_refill: false,
  can_manage_members: true,
  can_remove_members: true,
  can_manage_admins: false,
  can_delete: false,
}

const member: QuotaPoolMember = {
  id: 1,
  username: 'alice',
  display_name: 'Alice',
  email: 'alice@example.com',
  department: '研发一部',
  role: 1,
  status: 1,
  quota: 100,
  used_quota: 20,
  quota_pool_id: 7,
  quota_pool_admin: false,
}

test('protected pools do not expose member removal', () => {
  render(
    <QuotaPoolMemberActions
      pool={pool}
      capabilities={capabilities}
      member={member}
      onQuotaAction={vi.fn()}
      onRemove={vi.fn()}
      onAdminAction={vi.fn()}
    />
  )

  expect(
    screen.queryByRole('button', { name: 'Remove member' })
  ).not.toBeInTheDocument()
})
