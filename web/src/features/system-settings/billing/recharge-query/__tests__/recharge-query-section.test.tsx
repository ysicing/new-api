/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { RechargeQuerySection } from '../recharge-query-section'

const apiMocks = vi.hoisted(() => ({
  getAutoRechargeEligibility: vi.fn(),
  listQuotaPoolRechargeRecords: vi.fn(),
}))

vi.mock('../api', () => apiMocks)

function renderSection() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <RechargeQuerySection />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  apiMocks.getAutoRechargeEligibility.mockReset()
  apiMocks.listQuotaPoolRechargeRecords.mockReset()
  apiMocks.listQuotaPoolRechargeRecords.mockResolvedValue({
    success: true,
    data: {
      page: 1,
      page_size: 20,
      total: 1,
      items: [
        {
          id: 1,
          pool_id: 7,
          pool_name: '研发额度池',
          user_id: 12,
          user_name: 'alice',
          user_email: 'alice@example.com',
          operator_id: 0,
          operator_name: '',
          type: 'allocate_auto',
          amount: 500_000,
          created_at: 1_788_100_000,
        },
      ],
    },
  })
  apiMocks.getAutoRechargeEligibility.mockResolvedValue({
    success: true,
    data: {
      eligible: true,
      reason: '',
      user_id: 12,
      username: 'alice',
      email: 'alice@example.com',
      user_quota: 100_000,
      threshold: 1_000_000,
      pool_id: 7,
      pool_name: '研发额度池',
      pool_quota: 5_000_000,
      amount: 500_000,
      weekly: { used: 2, limit: 0 },
      monthly: { used: 2, limit: 10 },
    },
  })
})

test('loads this-week records by default and switches to the past month', async () => {
  const user = userEvent.setup()
  renderSection()

  expect(await screen.findByText('alice@example.com')).toBeInTheDocument()
  expect(apiMocks.listQuotaPoolRechargeRecords).toHaveBeenCalledWith({
    page: 1,
    pageSize: 20,
    period: 'week',
  })

  await user.click(screen.getByRole('button', { name: 'Past month' }))

  await waitFor(() => {
    expect(apiMocks.listQuotaPoolRechargeRecords).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      period: 'month',
    })
  })
})

test('queries one user without triggering a recharge action', async () => {
  const user = userEvent.setup()
  renderSection()

  await user.type(
    screen.getByRole('searchbox', { name: 'User ID, username, or email' }),
    'alice@example.com'
  )
  await user.click(screen.getByRole('button', { name: 'Check eligibility' }))

  await waitFor(() => {
    expect(apiMocks.getAutoRechargeEligibility).toHaveBeenCalledWith(
      'alice@example.com'
    )
  })
  expect(
    await screen.findByText('Eligible for automatic recharge')
  ).toBeInTheDocument()
  expect(screen.getByText('2 / Not limited')).toBeInTheDocument()
  expect(screen.getByText('2 / 10')).toBeInTheDocument()
})

test('announces recharge-record loading to assistive technology', () => {
  apiMocks.listQuotaPoolRechargeRecords.mockReturnValue(new Promise(() => {}))

  renderSection()

  expect(screen.getByRole('status')).toHaveTextContent('Loading...')
})

test('uses the normalized page returned by the recharge records API', async () => {
  const user = userEvent.setup()
  const firstPage = await apiMocks.listQuotaPoolRechargeRecords()
  apiMocks.listQuotaPoolRechargeRecords.mockImplementation(
    async (params: { page: number }) => ({
      ...firstPage,
      data: {
        ...firstPage.data,
        page: 1,
        total: params.page === 1 ? 21 : 1,
      },
    })
  )

  renderSection()
  expect(await screen.findByText(/Page 1 of 2/)).toBeInTheDocument()

  await user.click(screen.getByRole('button', { name: 'Next page' }))

  await waitFor(() => {
    expect(apiMocks.listQuotaPoolRechargeRecords).toHaveBeenLastCalledWith({
      page: 2,
      pageSize: 20,
      period: 'week',
    })
  })
  expect(await screen.findByText(/Page 1 of 1/)).toBeInTheDocument()
})
