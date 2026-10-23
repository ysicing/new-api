/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'

import type { QuotaPool, QuotaPoolCapabilities } from '../../types'
import { PoolConfiguration } from '../quota-pool-configuration'

const apiMocks = vi.hoisted(() => ({ updateQuotaPool: vi.fn() }))

vi.mock('../../api', () => apiMocks)

const pool: QuotaPool = {
  id: 7,
  name: '研发池',
  pool_type: 'normal',
  enabled: true,
  is_default: false,
  base_quota: 500_000_000,
  quota: 400_000_000,
  auto_recharge_amount: -1,
  weekly_limit: -1,
  monthly_limit: 0,
  monthly_refill_enabled: false,
  monthly_refill_top_up: false,
  monthly_refill_amount: 100_000_000,
  monthly_refill_day: 1,
  last_refill_month: 0,
  system_auto_recharge: {
    enabled: true,
    interval: 30,
    threshold: 25_000_000,
    amount: 100_000_000,
    weekly_limit: 3,
    monthly_limit: 10,
  },
}

const rootCapabilities: QuotaPoolCapabilities = {
  can_view: true,
  can_edit: true,
  can_edit_monthly_refill: true,
  can_refill: true,
  can_manage_members: true,
  can_remove_members: true,
  can_manage_admins: true,
  can_delete: true,
}

function renderConfiguration(
  capabilities = rootCapabilities,
  targetPool = pool
) {
  const queryClient = new QueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <PoolConfiguration pool={targetPool} capabilities={capabilities} />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  apiMocks.updateQuotaPool.mockReset()
  apiMocks.updateQuotaPool.mockResolvedValue({ success: true })
})

test('root saves recharge sentinels and monthly top-up settings', async () => {
  renderConfiguration()

  fireEvent.change(
    screen.getByRole('spinbutton', { name: 'Recharge amount' }),
    {
      target: { value: '-1' },
    }
  )
  fireEvent.change(screen.getByRole('spinbutton', { name: 'Weekly limit' }), {
    target: { value: '0' },
  })
  fireEvent.change(screen.getByRole('spinbutton', { name: 'Monthly limit' }), {
    target: { value: '-1' },
  })
  fireEvent.click(screen.getByRole('switch', { name: 'Monthly refill' }))
  fireEvent.click(
    screen.getByRole('switch', { name: 'Top up to target quota' })
  )
  fireEvent.change(
    screen.getByRole('spinbutton', { name: 'Monthly refill amount' }),
    { target: { value: '200' } }
  )
  fireEvent.change(
    screen.getByRole('spinbutton', { name: 'Monthly refill day' }),
    { target: { value: '15' } }
  )
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))

  await waitFor(() => {
    expect(apiMocks.updateQuotaPool).toHaveBeenCalledWith(
      7,
      {
        auto_recharge_amount: -1,
        weekly_limit: 0,
        monthly_limit: -1,
        monthly_refill_enabled: true,
        monthly_refill_top_up: true,
        monthly_refill_amount: 200,
        monthly_refill_day: 15,
      },
      undefined
    )
  })
})

test('pool policy editor cannot edit root-only monthly refill settings', () => {
  renderConfiguration({
    ...rootCapabilities,
    can_edit_monthly_refill: false,
  })

  expect(
    screen.getByRole('spinbutton', { name: 'Recharge amount' })
  ).toBeInTheDocument()
  expect(
    screen.queryByRole('spinbutton', { name: 'Base quota' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('switch', { name: 'Monthly refill' })
  ).not.toBeInTheDocument()
})

test('read-only configuration renders zero recharge amount as disabled', () => {
  renderConfiguration(
    {
      ...rootCapabilities,
      can_edit: false,
      can_edit_monthly_refill: false,
    },
    { ...pool, auto_recharge_amount: 0 }
  )

  expect(screen.getByText('Disabled')).toBeInTheDocument()
})

test('root can fund a zero-balance migrated pool and configure monthly refill', async () => {
  renderConfiguration(rootCapabilities, {
    ...pool,
    pool_type: 'normal',
    is_default: false,
    base_quota: 0,
    quota: 0,
  })
  expect(
    screen.getByRole('switch', { name: 'Monthly refill' })
  ).toBeInTheDocument()
  fireEvent.change(screen.getByRole('spinbutton', { name: 'Base quota' }), {
    target: { value: '100' },
  })
  fireEvent.click(screen.getByRole('switch', { name: 'Monthly refill' }))
  fireEvent.change(screen.getByRole('spinbutton', { name: 'Weekly limit' }), {
    target: { value: '2' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => {
    expect(apiMocks.updateQuotaPool).toHaveBeenCalledWith(
      7,
      {
        base_quota: 100,
        auto_recharge_amount: -1,
        weekly_limit: 2,
        monthly_limit: 0,
        monthly_refill_enabled: true,
        monthly_refill_top_up: false,
        monthly_refill_amount: 200,
        monthly_refill_day: 1,
      },
      undefined
    )
  })
})

test('root can save recharge rules without changing a migrated zero base quota', async () => {
  renderConfiguration(rootCapabilities, { ...pool, base_quota: 0, quota: 0 })
  fireEvent.change(screen.getByRole('spinbutton', { name: 'Weekly limit' }), {
    target: { value: '2' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(apiMocks.updateQuotaPool).toHaveBeenCalled())
  const values = apiMocks.updateQuotaPool.mock.calls[0]?.[1]
  expect(values).toMatchObject({ weekly_limit: 2 })
  expect(values).not.toHaveProperty('base_quota')
})

test('saving recharge rules after a pool refresh does not submit a stale base quota', async () => {
  const queryClient = new QueryClient()
  const { rerender } = render(
    <QueryClientProvider client={queryClient}>
      <PoolConfiguration pool={pool} capabilities={rootCapabilities} />
    </QueryClientProvider>
  )
  rerender(
    <QueryClientProvider client={queryClient}>
      <PoolConfiguration
        pool={{ ...pool, base_quota: 600_000_000 }}
        capabilities={rootCapabilities}
      />
    </QueryClientProvider>
  )
  fireEvent.change(screen.getByRole('spinbutton', { name: 'Weekly limit' }), {
    target: { value: '2' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))

  await waitFor(() => expect(apiMocks.updateQuotaPool).toHaveBeenCalled())
  expect(apiMocks.updateQuotaPool.mock.calls[0]?.[1]).not.toHaveProperty(
    'base_quota'
  )
  expect(screen.getByRole('spinbutton', { name: 'Base quota' })).toHaveValue(
    1200
  )
})

test('a saved base quota edit is not resubmitted after another refill', async () => {
  const queryClient = new QueryClient()
  const { rerender } = render(
    <QueryClientProvider client={queryClient}>
      <PoolConfiguration pool={pool} capabilities={rootCapabilities} />
    </QueryClientProvider>
  )
  fireEvent.change(screen.getByRole('spinbutton', { name: 'Base quota' }), {
    target: { value: '1100' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => {
    expect(apiMocks.updateQuotaPool.mock.calls[0]?.[1]).toHaveProperty(
      'base_quota',
      1100
    )
    expect(screen.getByRole('button', { name: 'Save' })).not.toBeDisabled()
  })
  rerender(
    <QueryClientProvider client={queryClient}>
      <PoolConfiguration
        pool={{ ...pool, base_quota: 600_000_000 }}
        capabilities={rootCapabilities}
      />
    </QueryClientProvider>
  )
  fireEvent.change(screen.getByRole('spinbutton', { name: 'Weekly limit' }), {
    target: { value: '2' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Save' }))
  await waitFor(() => expect(apiMocks.updateQuotaPool).toHaveBeenCalledTimes(2))
  expect(apiMocks.updateQuotaPool.mock.calls[1]?.[1]).not.toHaveProperty(
    'base_quota'
  )
})
