/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { Tools } from '../index'
import type { SelfAutoRechargeEligibility } from '../types'

const apiMocks = vi.hoisted(() => ({
  getSelfAutoRechargeEligibility: vi.fn(),
}))

vi.mock('../api', () => apiMocks)

const eligibleResult: SelfAutoRechargeEligibility = {
  status: 'eligible',
  eligible: true,
  user_quota: 100_000,
  threshold: 1_000_000,
  amount: 500_000,
  pool_name: 'Platform pool',
  pool_type: 'normal',
  weekly: { used: 1, limit: 0 },
  monthly: { used: 2, limit: 10 },
}

function renderTools() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <Tools />
    </QueryClientProvider>
  )
}

async function openEligibilityDialog(user: ReturnType<typeof userEvent.setup>) {
  await user.click(
    screen.getByRole('button', { name: 'Check automatic recharge eligibility' })
  )
}

beforeEach(() => {
  apiMocks.getSelfAutoRechargeEligibility.mockReset()
  apiMocks.getSelfAutoRechargeEligibility.mockResolvedValue({
    success: true,
    data: eligibleResult,
  })
})

test('does not request eligibility until the tool dialog opens', async () => {
  const user = userEvent.setup()
  renderTools()

  expect(apiMocks.getSelfAutoRechargeEligibility).not.toHaveBeenCalled()

  await openEligibilityDialog(user)

  expect(
    await screen.findByText('Eligible for automatic recharge')
  ).toBeInTheDocument()
  expect(apiMocks.getSelfAutoRechargeEligibility).toHaveBeenCalledTimes(1)
})

test('describes the Tools page as read-only account self-service', () => {
  renderTools()

  expect(
    screen.getByText('Account self-service queries and diagnostics.')
  ).toBeInTheDocument()
})

test('shows the eligible snapshot details in the dialog', async () => {
  const user = userEvent.setup()
  renderTools()

  await openEligibilityDialog(user)

  expect(
    await screen.findByText('Eligible for automatic recharge')
  ).toBeInTheDocument()
  expect(screen.getByText('Platform pool')).toBeInTheDocument()
  expect(screen.getByText('1 / Not limited')).toBeInTheDocument()
  expect(screen.getByText('2 / 10')).toBeInTheDocument()
})

test('shows that automatic recharge is unnecessary above the threshold', async () => {
  const user = userEvent.setup()
  apiMocks.getSelfAutoRechargeEligibility.mockResolvedValue({
    success: true,
    data: {
      ...eligibleResult,
      status: 'not_needed',
      eligible: false,
      reason: 'quota_above_threshold',
      user_quota: 2_000_000,
    },
  })
  renderTools()

  await openEligibilityDialog(user)

  expect(
    await screen.findByText('The user balance is above the recharge threshold.')
  ).toBeInTheDocument()
  expect(screen.getAllByText('Automatic recharge is not needed.')).toHaveLength(
    2
  )
})

test.each([
  ['disabled', 'Automatic recharge is disabled globally.'],
  ['quota_pool_not_found', 'The user quota pool no longer exists.'],
  [
    'new_user_pool_disabled',
    'The new-user quota pool does not support automatic recharge.',
  ],
  ['quota_pool_disabled', 'The user quota pool is disabled.'],
  ['amount_not_configured', 'No valid recharge amount is configured.'],
  ['weekly_count_failed', 'The weekly recharge count could not be read.'],
  ['weekly_limited', 'The weekly recharge limit has been reached.'],
  ['monthly_count_failed', 'The monthly recharge count could not be read.'],
  ['monthly_limited', 'The monthly recharge limit has been reached.'],
  ['quota_pool_insufficient', 'The quota pool balance is insufficient.'],
  ['user_disabled', 'The user account is disabled.'],
])(
  'shows the %s blocking rule without exposing its code',
  async (reason, expected) => {
    const user = userEvent.setup()
    apiMocks.getSelfAutoRechargeEligibility.mockResolvedValue({
      success: true,
      data: {
        ...eligibleResult,
        status: 'blocked',
        eligible: false,
        reason,
        guidance: 'quota_pool_admin',
      },
    })
    renderTools()

    await openEligibilityDialog(user)

    expect(await screen.findByText(expected)).toBeInTheDocument()
    expect(screen.queryByText(reason)).not.toBeInTheDocument()
  }
)

test('shows a fixed safe blocking rule for an unknown reason', async () => {
  const user = userEvent.setup()
  const unknownReason = 'constructor'
  apiMocks.getSelfAutoRechargeEligibility.mockResolvedValue({
    success: true,
    data: {
      ...eligibleResult,
      status: 'blocked',
      eligible: false,
      reason: unknownReason,
      guidance: 'quota_pool_admin',
    },
  })
  renderTools()

  await openEligibilityDialog(user)

  expect(
    await screen.findByText(
      'Automatic recharge is currently unavailable. Please follow the guidance below and contact the appropriate person.'
    )
  ).toBeInTheDocument()
  expect(screen.queryByText(unknownReason)).not.toBeInTheDocument()
})

test.each([
  ['quota_pool_admin', 'Contact your quota pool administrator.'],
  [
    'department_quota_pool_admin',
    "Contact your department's quota pool administrator.",
  ],
  [
    'operations_oa',
    'Please submit an operations OA work-order approval request.',
  ],
])(
  'shows %s guidance for a blocked eligibility snapshot',
  async (guidance, expected) => {
    const user = userEvent.setup()
    apiMocks.getSelfAutoRechargeEligibility.mockResolvedValue({
      success: true,
      data: {
        ...eligibleResult,
        status: 'blocked',
        eligible: false,
        reason: 'quota_pool_disabled',
        guidance,
      },
    })
    renderTools()

    await openEligibilityDialog(user)

    expect(await screen.findByText(expected)).toBeInTheDocument()
  }
)

test('shows a safe fallback instead of a raw eligibility API error', async () => {
  const user = userEvent.setup()
  apiMocks.getSelfAutoRechargeEligibility.mockRejectedValue(
    new Error('internal database password leaked')
  )
  renderTools()

  await openEligibilityDialog(user)

  expect(
    await screen.findByText('We could not check recharge eligibility.')
  ).toBeInTheDocument()
  expect(
    screen.queryByText('internal database password leaked')
  ).not.toBeInTheDocument()
})

test('retries a failed eligibility request from the dialog', async () => {
  const user = userEvent.setup()
  apiMocks.getSelfAutoRechargeEligibility.mockRejectedValueOnce(
    new Error('temporary failure')
  )
  apiMocks.getSelfAutoRechargeEligibility.mockResolvedValueOnce({
    success: true,
    data: eligibleResult,
  })
  renderTools()

  await openEligibilityDialog(user)
  await screen.findByText('We could not check recharge eligibility.')
  await user.click(screen.getByRole('button', { name: 'Retry' }))

  expect(
    await screen.findByText('Eligible for automatic recharge')
  ).toBeInTheDocument()
  expect(apiMocks.getSelfAutoRechargeEligibility).toHaveBeenCalledTimes(2)
})

test('loads a fresh eligibility snapshot whenever the dialog reopens', async () => {
  const user = userEvent.setup()
  renderTools()

  await openEligibilityDialog(user)
  await screen.findByText('Eligible for automatic recharge')
  await user.click(screen.getByRole('button', { name: 'Close' }))
  await waitFor(() => {
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  await openEligibilityDialog(user)

  expect(
    await screen.findByText('Eligible for automatic recharge')
  ).toBeInTheDocument()
  expect(apiMocks.getSelfAutoRechargeEligibility).toHaveBeenCalledTimes(2)
})

test('announces asynchronous results and keeps aria-busy synchronized', async () => {
  const user = userEvent.setup()
  let resolveRequest:
    | ((value: { success: true; data: SelfAutoRechargeEligibility }) => void)
    | undefined
  apiMocks.getSelfAutoRechargeEligibility.mockImplementation(
    () =>
      new Promise((resolve) => {
        resolveRequest = resolve
      })
  )
  renderTools()

  await openEligibilityDialog(user)

  const liveRegion = document.querySelector('[aria-live="polite"]')
  expect(liveRegion).toHaveAttribute('aria-busy', 'true')

  await act(async () => {
    resolveRequest?.({ success: true, data: eligibleResult })
  })

  expect(
    await screen.findByText('Eligible for automatic recharge')
  ).toBeInTheDocument()
  expect(liveRegion).toHaveAttribute('aria-busy', 'false')
})

test('Escape closes the dialog and returns focus to the tool trigger', async () => {
  const user = userEvent.setup()
  renderTools()
  const trigger = screen.getByRole('button', {
    name: 'Check automatic recharge eligibility',
  })

  await user.click(trigger)
  await screen.findByRole('dialog')
  await user.keyboard('{Escape}')

  await waitFor(() => {
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
  expect(trigger).toHaveFocus()
})
