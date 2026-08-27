/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'

import type { QuotaPool, QuotaPoolCapabilities } from '../../types'
import { QuotaPoolDetail } from '../quota-pool-detail'

const pool: QuotaPool = {
  id: 7,
  name: '平台保障部',
  pool_type: 'normal',
  enabled: true,
  is_default: false,
  base_quota: 500_000_000,
  quota: 400_000_000,
  auto_recharge_amount: -1,
  weekly_limit: -1,
  monthly_limit: -1,
  monthly_refill_enabled: false,
  monthly_refill_top_up: false,
  monthly_refill_amount: 0,
  monthly_refill_day: 1,
  last_refill_month: 0,
  member_count: 1,
}

const memberCapabilities: QuotaPoolCapabilities = {
  can_view: true,
  can_edit: false,
  can_edit_monthly_refill: false,
  can_refill: false,
  can_manage_members: false,
  can_remove_members: false,
  can_manage_admins: false,
  can_delete: false,
}

const adminContacts = [
  {
    id: 9,
    username: 'pool-admin',
    display_name: 'Alice Chen',
    email: 'alice@example.com',
  },
]

function renderDetail(
  capabilities: QuotaPoolCapabilities,
  poolOverrides?: Partial<QuotaPool>
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <QuotaPoolDetail
        pool={{ ...pool, ...poolOverrides }}
        capabilities={capabilities}
        adminContacts={adminContacts}
        selfMode
      />
    </QueryClientProvider>
  )
}

test('ordinary pool member only sees the overview tab', () => {
  renderDetail(memberCapabilities)

  expect(screen.getByRole('tab', { name: 'Overview' })).toBeInTheDocument()
  for (const name of [
    'Members',
    'Transactions',
    'Operation logs',
    'Statistics',
    'Configuration',
  ]) {
    expect(screen.queryByRole('tab', { name })).not.toBeInTheDocument()
  }
  expect(screen.getByText('Pool administrators')).toBeInTheDocument()
  expect(screen.getByText('Alice Chen')).toBeInTheDocument()
  expect(screen.getByText('alice@example.com')).toBeInTheDocument()
})

test('default pool member does not see pool administrator contacts', () => {
  renderDetail(memberCapabilities, {
    name: '产研中心默认额度池(存量)',
    pool_type: 'default',
    is_default: true,
    base_quota: -1,
    quota: -1,
  })

  expect(screen.getByRole('tab', { name: 'Overview' })).toBeInTheDocument()
  expect(screen.queryByText('Pool administrators')).not.toBeInTheDocument()
  expect(screen.queryByText('Alice Chen')).not.toBeInTheDocument()
})

test('pool manager keeps all management tabs', () => {
  renderDetail({ ...memberCapabilities, can_manage_members: true })

  for (const name of [
    'Overview',
    'Members',
    'Transactions',
    'Operation logs',
    'Statistics',
    'Configuration',
  ]) {
    expect(screen.getByRole('tab', { name })).toBeInTheDocument()
  }
  expect(screen.queryByText('Pool administrators')).not.toBeInTheDocument()
})
