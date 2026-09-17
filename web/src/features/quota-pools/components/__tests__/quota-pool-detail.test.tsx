/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

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
  poolOverrides?: Partial<QuotaPool>,
  selfMode = true
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
        selfMode={selfMode}
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

vi.mock('@/lib/api', () => ({ api: { get: vi.fn() } }))
afterEach(() => vi.clearAllMocks())

test.each([false, true])(
  'history pages and page sizes are independent for self=%s',
  async (self) => {
    vi.mocked(api.get).mockImplementation(async (url, config) => {
      const page = config?.params?.p ?? 1
      const isLog = String(url).endsWith('/operation_logs')
      const item = isLog
        ? {
            id: page,
            username: 'operator',
            user_id: 1,
            content: `log-${page}`,
            other: '',
            created_at: 1,
          }
        : {
            id: page,
            user_name: `member-${page}`,
            amount: 10,
            type: 'allocate_manual',
            created_at: 1,
          }
      return { data: { success: true, data: { items: [item], total: 21 } } }
    })
    renderDetail(
      { ...memberCapabilities, can_manage_members: true },
      undefined,
      self
    )
    fireEvent.click(screen.getByRole('tab', { name: 'Transactions' }))
    await screen.findByText('member-1')
    expect(screen.getByRole('button', { name: 'Previous page' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByText('member-2')
    const prefix = self ? '/api/quota_pool/self' : '/api/quota_pool/7'
    expect(api.get).toHaveBeenCalledWith(`${prefix}/transactions`, {
      params: { p: 2, page_size: 10 },
    })

    fireEvent.click(screen.getByRole('tab', { name: 'Operation logs' }))
    await screen.findByText('log-1')
    fireEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByText('log-2')
    expect(api.get).toHaveBeenCalledWith(`${prefix}/operation_logs`, {
      params: { p: 2, page_size: 10 },
    })

    fireEvent.click(screen.getByRole('tab', { name: 'Transactions' }))
    await screen.findByText('member-2')
    await waitFor(() =>
      expect(
        screen.getByRole('combobox', { name: 'Rows per page' })
      ).toBeEnabled()
    )
    fireEvent.change(screen.getByRole('combobox', { name: 'Rows per page' }), {
      target: { value: '20' },
    })
    await screen.findByText('member-1')
    expect(api.get).toHaveBeenCalledWith(`${prefix}/transactions`, {
      params: { p: 1, page_size: 20 },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Next page' }))
    await screen.findByText('member-2')
    expect(screen.getByRole('button', { name: 'Next page' })).toBeDisabled()

    fireEvent.click(screen.getByRole('tab', { name: 'Operation logs' }))
    await screen.findByText('log-2')
    expect(screen.getByRole('combobox', { name: 'Rows per page' })).toHaveValue(
      '10'
    )
  }
)

test('empty history disables both page navigation buttons', async () => {
  vi.mocked(api.get).mockResolvedValue({
    data: { success: true, data: { items: [], total: 0 } },
  })
  renderDetail({ ...memberCapabilities, can_manage_members: true })
  fireEvent.click(screen.getByRole('tab', { name: 'Transactions' }))
  await waitFor(() =>
    expect(
      screen.getByRole('combobox', { name: 'Rows per page' })
    ).toBeEnabled()
  )
  expect(screen.getByRole('button', { name: 'Previous page' })).toBeDisabled()
  expect(screen.getByRole('button', { name: 'Next page' })).toBeDisabled()
})

test('history navigation waits for the request and can go back after a failed page', async () => {
  const firstPage = {
    data: {
      success: true,
      data: {
        items: [
          {
            id: 1,
            user_name: 'first-member',
            amount: 10,
            type: 'allocate_manual',
            created_at: 1,
          },
        ],
        total: 21,
      },
    },
  }
  const pending = Promise.withResolvers<typeof firstPage>()
  vi.mocked(api.get)
    .mockResolvedValueOnce(firstPage)
    .mockImplementationOnce(() => pending.promise)
    .mockResolvedValue(firstPage)
  renderDetail({ ...memberCapabilities, can_manage_members: true })
  fireEvent.click(screen.getByRole('tab', { name: 'Transactions' }))
  await screen.findByText('first-member')
  fireEvent.click(screen.getByRole('button', { name: 'Next page' }))
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Next page' })).toBeDisabled()
  )
  expect(screen.getByRole('button', { name: 'Previous page' })).toBeDisabled()
  expect(screen.getByRole('combobox', { name: 'Rows per page' })).toBeDisabled()
  pending.reject(new Error('network unavailable'))
  await screen.findByText('Loading failed')
  expect(screen.getByRole('button', { name: 'Previous page' })).toBeEnabled()
  fireEvent.click(screen.getByRole('button', { name: 'Previous page' }))
  await screen.findByText('first-member')
})
