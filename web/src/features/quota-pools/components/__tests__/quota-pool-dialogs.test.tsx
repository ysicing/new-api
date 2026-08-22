/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { AddQuotaPoolMemberDialog } from '../quota-pool-dialogs'

const apiMocks = vi.hoisted(() => ({
  addQuotaPoolMember: vi.fn(),
  createQuotaPool: vi.fn(),
  getQuotaPoolCandidates: vi.fn(),
  refillQuotaPool: vi.fn(),
}))

vi.mock('../../api', () => apiMocks)

beforeEach(() => {
  for (const mock of Object.values(apiMocks)) mock.mockReset()
  apiMocks.getQuotaPoolCandidates.mockResolvedValue({
    success: true,
    data: {
      items: [
        {
          id: 12,
          username: 'alice',
          display_name: 'Alice Chen',
          email: 'alice@example.com',
          department: '平台/保障部',
          role: 1,
          status: 1,
          quota: 0,
          used_quota: 0,
          quota_pool_id: 0,
          quota_pool_admin_level: 0,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    },
  })
  apiMocks.addQuotaPoolMember.mockResolvedValue({ success: true })
})

function renderAddMemberDialog(onSaved = vi.fn().mockResolvedValue(undefined)) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <AddQuotaPoolMemberDialog
        poolId={7}
        open
        onOpenChange={() => undefined}
        onSaved={onSaved}
      />
    </QueryClientProvider>
  )
  return onSaved
}

test('selects an eligible user instead of requiring a raw user ID', async () => {
  const user = userEvent.setup()
  const onSaved = renderAddMemberDialog()

  const selector = await screen.findByRole<HTMLInputElement>('combobox', {
    name: 'User',
  })
  await waitFor(() =>
    expect(apiMocks.getQuotaPoolCandidates).toHaveBeenCalledWith(false, {
      page: 1,
      pageSize: 20,
      keyword: '',
    })
  )

  await user.click(selector)
  await user.click(
    await screen.findByRole('option', { name: /alice.*ID:12.*Alice Chen/ })
  )
  await user.click(screen.getByRole('button', { name: 'Add member' }))

  await waitFor(() =>
    expect(apiMocks.addQuotaPoolMember).toHaveBeenCalledWith(7, 12, undefined)
  )
  expect(onSaved).toHaveBeenCalledOnce()
})

test('searches candidates remotely by the entered identity', async () => {
  const user = userEvent.setup()
  renderAddMemberDialog()

  const selector = await screen.findByRole('combobox', { name: 'User' })
  await user.click(selector)
  await user.type(selector, 'alice')

  await waitFor(() =>
    expect(apiMocks.getQuotaPoolCandidates).toHaveBeenLastCalledWith(false, {
      page: 1,
      pageSize: 20,
      keyword: 'alice',
    })
  )
})

test('does not submit a previously selected user after the search text changes', async () => {
  const user = userEvent.setup()
  renderAddMemberDialog()

  const selector = await screen.findByRole<HTMLInputElement>('combobox', {
    name: 'User',
  })
  await user.click(selector)
  await user.click(
    await screen.findByRole('option', { name: /alice.*ID:12.*Alice Chen/ })
  )
  selector.setSelectionRange(0, selector.value.length)
  await user.type(selector, 'bob')
  await user.keyboard('{Escape}')
  await user.click(screen.getByRole('button', { name: 'Add member' }))

  expect(apiMocks.addQuotaPoolMember).not.toHaveBeenCalled()
})

test('does not report an empty result while candidates are loading', async () => {
  apiMocks.getQuotaPoolCandidates.mockReturnValue(new Promise(() => undefined))
  const user = userEvent.setup()
  renderAddMemberDialog()

  await user.click(await screen.findByRole('combobox', { name: 'User' }))

  expect(await screen.findByText('Searching users...')).toBeInTheDocument()
  expect(screen.queryByText('No eligible users found.')).not.toBeInTheDocument()
})

test('shows the candidate API failure instead of an empty result', async () => {
  apiMocks.getQuotaPoolCandidates.mockResolvedValue({
    success: false,
    message: 'raw server error',
  })
  const user = userEvent.setup()
  renderAddMemberDialog()

  await user.click(await screen.findByRole('combobox', { name: 'User' }))

  expect(await screen.findByText('Failed to load users.')).toBeInTheDocument()
  expect(screen.queryByText('No eligible users found.')).not.toBeInTheDocument()
  expect(screen.queryByText('raw server error')).not.toBeInTheDocument()
})
