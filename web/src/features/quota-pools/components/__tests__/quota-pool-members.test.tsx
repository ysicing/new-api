/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import {
  QueryClient,
  QueryClientProvider,
  type UseQueryResult,
} from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'

import { formatQuota } from '@/lib/format'
import { ROLE } from '@/lib/roles'

import type {
  ApiResponse,
  PageData,
  QuotaPool,
  QuotaPoolCapabilities,
  QuotaPoolMember,
} from '../../types'
import { PoolMembers } from '../quota-pool-members'

const apiMocks = vi.hoisted(() => ({
  removeQuotaPoolMember: vi.fn(),
  reclaimQuotaPoolMember: vi.fn(),
  rechargeQuotaPoolMember: vi.fn(),
  revokeQuotaPoolAdmin: vi.fn(),
  setQuotaPoolAdmin: vi.fn(),
}))

vi.mock('../../api', () => apiMocks)

const pool: QuotaPool = {
  id: 7,
  name: '研发池',
  pool_type: 'normal',
  enabled: true,
  is_default: false,
  base_quota: 1000,
  quota: 800,
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
  can_manage_members: false,
  can_remove_members: false,
  can_manage_admins: false,
  can_delete: false,
}

function membersQuery(
  quotaPoolAdmin = false,
  memberRole = 1,
  quota = 100,
  usedQuota = 20,
  reclaimAmounts: number[] = [500, 250]
): UseQueryResult<ApiResponse<PageData<QuotaPoolMember>>> {
  return {
    isLoading: false,
    isError: false,
    data: {
      success: true,
      data: {
        items: [
          {
            id: 1,
            username: 'alice',
            display_name: 'Alice',
            email: 'alice@example.com',
            department: '研发一部',
            role: memberRole,
            status: 1,
            quota,
            used_quota: usedQuota,
            quota_pool_id: 7,
            quota_pool_admin: quotaPoolAdmin,
            reclaim_amounts: reclaimAmounts,
          },
        ],
        total: 25,
        page: 1,
        page_size: 10,
      },
    },
  } as unknown as UseQueryResult<ApiResponse<PageData<QuotaPoolMember>>>
}

function renderMembers(
  memberCapabilities = capabilities,
  options?: {
    memberAdmin?: boolean
    memberRole?: number
    quota?: number
    usedQuota?: number
    reclaimAmounts?: number[]
  }
) {
  const queryClient = new QueryClient()
  return render(
    <QueryClientProvider client={queryClient}>
      <PoolMembers
        pool={pool}
        capabilities={memberCapabilities}
        selfMode={false}
        query={membersQuery(
          options?.memberAdmin,
          options?.memberRole,
          options?.quota,
          options?.usedQuota,
          options?.reclaimAmounts
        )}
        page={1}
        pageSize={10}
        keyword=''
        onSearch={vi.fn()}
        onPageChange={vi.fn()}
        onPageSizeChange={vi.fn()}
      />
    </QueryClientProvider>
  )
}

async function openMemberActions() {
  fireEvent.click(screen.getByRole('button', { name: 'Open menu' }))
  return screen.findByRole('menu')
}

beforeEach(() => {
  for (const mock of Object.values(apiMocks)) {
    mock.mockReset()
    mock.mockResolvedValue({ success: true })
  }
})

test('quota pool members support server search and pagination controls', () => {
  const onSearch = vi.fn()
  const onPageChange = vi.fn()
  const onPageSizeChange = vi.fn()
  const queryClient = new QueryClient()

  render(
    <QueryClientProvider client={queryClient}>
      <PoolMembers
        pool={pool}
        capabilities={capabilities}
        query={membersQuery()}
        page={1}
        pageSize={10}
        keyword=''
        onSearch={onSearch}
        onPageChange={onPageChange}
        onPageSizeChange={onPageSizeChange}
      />
    </QueryClientProvider>
  )

  fireEvent.change(screen.getByRole('searchbox', { name: 'Search' }), {
    target: { value: 'alice' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Search' }))
  expect(onSearch).toHaveBeenCalledWith('alice')

  fireEvent.click(screen.getByRole('button', { name: 'Next page' }))
  expect(onPageChange).toHaveBeenCalledWith(2)

  fireEvent.change(screen.getByRole('combobox', { name: 'Rows per page' }), {
    target: { value: '20' },
  })
  expect(onPageSizeChange).toHaveBeenCalledWith(20)
})

test('member quota shows available and total amounts with available progress', () => {
  renderMembers(capabilities, { quota: 1_000_000, usedQuota: 200_000 })

  expect(
    screen.getByRole('columnheader', {
      name: 'Available quota / Total quota',
    })
  ).toBeInTheDocument()
  expect(screen.getByText(formatQuota(1_000_000))).toBeInTheDocument()
  expect(screen.getByText(formatQuota(1_200_000))).toBeInTheDocument()
  expect(
    screen.getByRole('progressbar', { name: 'Available quota' })
  ).toHaveAttribute('aria-valuenow', '83.33333333333334')
})

test('member quota progress is zero when total quota is zero', () => {
  renderMembers(capabilities, { quota: 0, usedQuota: 0 })

  expect(
    screen.getByRole('progressbar', { name: 'Available quota' })
  ).toHaveAttribute('aria-valuenow', '0')
})

test('marks pool administrators next to the member name', () => {
  renderMembers(capabilities, { memberAdmin: true })

  expect(screen.getByText('Pool administrator')).toBeInTheDocument()
})

test('does not mark ordinary members as pool administrators', () => {
  renderMembers()

  expect(screen.queryByText('Pool administrator')).not.toBeInTheDocument()
})

test('member recharge requires confirmation before submitting', async () => {
  renderMembers({ ...capabilities, can_manage_members: true })

  await openMemberActions()
  fireEvent.click(screen.getByRole('menuitem', { name: 'Recharge' }))

  expect(apiMocks.rechargeQuotaPoolMember).not.toHaveBeenCalled()
  expect(screen.getByRole('alertdialog')).toHaveTextContent(
    'Confirm recharge for Alice?'
  )
  fireEvent.click(screen.getByRole('button', { name: 'Confirm' }))

  await waitFor(() => {
    expect(apiMocks.rechargeQuotaPoolMember).toHaveBeenCalledWith(7, 1, false)
  })
})

test('member reclaim submits the selected allowed amount', async () => {
  renderMembers({ ...capabilities, can_manage_members: true })

  await openMemberActions()
  fireEvent.click(screen.getByRole('menuitem', { name: 'Reclaim' }))
  fireEvent.click(screen.getByRole('radio', { name: formatQuota(250) }))
  fireEvent.click(screen.getByRole('button', { name: 'Confirm' }))

  await waitFor(() => {
    expect(apiMocks.reclaimQuotaPoolMember).toHaveBeenCalledWith(
      7,
      1,
      250,
      false
    )
  })
})

test('member removal requires confirmation and explains quota recovery', async () => {
  renderMembers({ ...capabilities, can_remove_members: true })

  await openMemberActions()
  fireEvent.click(screen.getByRole('menuitem', { name: 'Remove member' }))

  expect(apiMocks.removeQuotaPoolMember).not.toHaveBeenCalled()
  const dialog = screen.getByRole('alertdialog', { name: 'Remove member' })
  expect(dialog).toHaveTextContent('Alice')
  expect(dialog).toHaveTextContent(formatQuota(100))
  expect(dialog).toHaveTextContent('No trial quota will be granted again.')
  fireEvent.click(screen.getByRole('button', { name: 'Confirm removal' }))

  await waitFor(() => {
    expect(apiMocks.removeQuotaPoolMember).toHaveBeenCalledWith(7, 1, false)
  })
})

test('canceling member removal does not call the API', async () => {
  renderMembers({ ...capabilities, can_remove_members: true })

  await openMemberActions()
  fireEvent.click(screen.getByRole('menuitem', { name: 'Remove member' }))
  fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

  expect(apiMocks.removeQuotaPoolMember).not.toHaveBeenCalled()
})

test('pool administrator cannot remove another pool administrator', () => {
  renderMembers(
    { ...capabilities, can_remove_members: true },
    { memberAdmin: true }
  )

  expect(
    screen.queryByRole('button', { name: 'Open menu' })
  ).not.toBeInTheDocument()
})

test('pool administrator cannot remove a privileged system user', () => {
  renderMembers(
    { ...capabilities, can_remove_members: true },
    { memberRole: 10 }
  )

  expect(
    screen.queryByRole('button', { name: 'Open menu' })
  ).not.toBeInTheDocument()
})

test('global administrator can remove a pool administrator', async () => {
  renderMembers(
    {
      ...capabilities,
      can_remove_members: true,
      can_manage_admins: true,
    },
    { memberAdmin: true }
  )

  await openMemberActions()
  expect(
    screen.getByRole('menuitem', { name: 'Remove member' })
  ).toBeInTheDocument()
})

test('root user cannot be set as a pool administrator', () => {
  renderMembers(
    { ...capabilities, can_manage_admins: true },
    { memberRole: ROLE.SUPER_ADMIN }
  )

  expect(
    screen.queryByRole('button', { name: 'Open menu' })
  ).not.toBeInTheDocument()
})

test('groups member operations in the actions menu', async () => {
  renderMembers({
    ...capabilities,
    can_manage_members: true,
    can_remove_members: true,
    can_manage_admins: true,
  })

  expect(
    screen.getByRole('columnheader', { name: 'Actions' })
  ).toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Recharge' })
  ).not.toBeInTheDocument()
  const menuTrigger = screen.getByRole('button', { name: 'Open menu' })
  expect(menuTrigger.closest('td')).toHaveClass('w-12', 'text-right')
  expect(screen.getByText('Alice').closest('td')).not.toHaveClass('w-12')
  fireEvent.click(menuTrigger)

  for (const name of [
    'Recharge',
    'Reclaim',
    'Set pool administrator',
    'Remove member',
  ]) {
    expect(await screen.findByRole('menuitem', { name })).toBeInTheDocument()
  }
})

test('disables reclaim in the menu when no reclaim amount is available', async () => {
  renderMembers(
    { ...capabilities, can_manage_members: true },
    { reclaimAmounts: [] }
  )

  await openMemberActions()
  expect(screen.getByRole('menuitem', { name: 'Reclaim' })).toHaveAttribute(
    'aria-disabled',
    'true'
  )
})

test('an existing root pool administrator can still be removed', async () => {
  renderMembers(
    { ...capabilities, can_manage_admins: true },
    { memberAdmin: true, memberRole: ROLE.SUPER_ADMIN }
  )

  await openMemberActions()
  expect(
    screen.getByRole('menuitem', { name: 'Remove pool administrator' })
  ).toBeInTheDocument()
})

test('failed member removal keeps the confirmation dialog open', async () => {
  apiMocks.removeQuotaPoolMember.mockResolvedValue({
    success: false,
    message: 'remove failed',
  })
  renderMembers({ ...capabilities, can_remove_members: true })

  await openMemberActions()
  fireEvent.click(screen.getByRole('menuitem', { name: 'Remove member' }))
  fireEvent.click(screen.getByRole('button', { name: 'Confirm removal' }))

  await waitFor(() => expect(apiMocks.removeQuotaPoolMember).toHaveBeenCalled())
  expect(
    screen.getByRole('alertdialog', { name: 'Remove member' })
  ).toBeInTheDocument()
})

test('network failure keeps the member removal dialog open', async () => {
  apiMocks.removeQuotaPoolMember.mockRejectedValue(new Error('network error'))
  renderMembers({ ...capabilities, can_remove_members: true })

  await openMemberActions()
  fireEvent.click(screen.getByRole('menuitem', { name: 'Remove member' }))
  fireEvent.click(screen.getByRole('button', { name: 'Confirm removal' }))

  await waitFor(() => expect(apiMocks.removeQuotaPoolMember).toHaveBeenCalled())
  expect(
    screen.getByRole('alertdialog', { name: 'Remove member' })
  ).toBeInTheDocument()
})
