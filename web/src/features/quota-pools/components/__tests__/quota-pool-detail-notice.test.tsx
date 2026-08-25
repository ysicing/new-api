/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import { expect, test } from 'vitest'

import type { QuotaPool, QuotaPoolCapabilities } from '../../types'
import { QuotaPoolDetail } from '../quota-pool-detail'

const pool: QuotaPool = {
  id: 9,
  name: '默认额度池',
  pool_type: 'new_user',
  enabled: true,
  is_default: true,
  base_quota: 100_000,
  quota: 100_000,
  auto_recharge_amount: 0,
  weekly_limit: 0,
  monthly_limit: 0,
  monthly_refill_enabled: false,
  monthly_refill_top_up: false,
  monthly_refill_amount: 0,
  monthly_refill_day: 1,
  last_refill_month: 0,
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

function renderDetail(options?: {
  poolType?: QuotaPool['pool_type']
  selfMode?: boolean
  canManageMembers?: boolean
}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <QuotaPoolDetail
        pool={{ ...pool, pool_type: options?.poolType ?? 'new_user' }}
        capabilities={{
          ...memberCapabilities,
          can_manage_members: options?.canManageMembers ?? false,
        }}
        selfMode={options?.selfMode ?? true}
      />
    </QueryClientProvider>
  )
}

test('ordinary member sees the one-time trial notice in the new-user pool', () => {
  renderDetail()

  const alert = screen.getByRole('alert')
  expect(within(alert).getByText('Trial quota notice')).toBeInTheDocument()
  expect(
    within(alert).getByText(
      'Please contact your department quota pool administrator to join the appropriate quota pool. The current pool provides a one-time trial quota only; no additional quota will be granted after it is used up.'
    )
  ).toBeInTheDocument()
  expect(screen.queryByText('Pool administrators')).not.toBeInTheDocument()
})

test('ordinary member does not see the trial notice in a normal pool', () => {
  renderDetail({ poolType: 'normal' })

  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('management view does not show the trial notice for the new-user pool', () => {
  renderDetail({ selfMode: false, canManageMembers: true })

  expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})
