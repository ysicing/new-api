/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { useAuthStore, type AuthUser } from '@/stores/auth-store'

import { QuotaPools } from '../index'
import type { QuotaPool, QuotaPoolCapabilities } from '../types'

const apiMocks = vi.hoisted(() => ({
  addQuotaPoolMember: vi.fn(),
  setQuotaPoolEnabled: vi.fn(),
  getQuotaPools: vi.fn(),
  getQuotaPool: vi.fn(),
  getQuotaPoolCandidates: vi.fn(),
  getSelfQuotaPool: vi.fn(),
  getQuotaPoolMembers: vi.fn(),
  getQuotaPoolTransactions: vi.fn(),
  getQuotaPoolStats: vi.fn(),
  getQuotaPoolOperationLogs: vi.fn(),
}))

vi.mock('../api', () => apiMocks)

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

const otherPool: QuotaPool = {
  ...pool,
  id: 8,
  name: '研发池',
  quota: 300_000_000,
}

const viewCapabilities: QuotaPoolCapabilities = {
  can_view: true,
  can_edit: false,
  can_edit_monthly_refill: false,
  can_refill: false,
  can_manage_members: false,
  can_remove_members: false,
  can_manage_admins: false,
  can_delete: false,
}

const adminContact = {
  id: 9,
  username: 'pool-admin',
  display_name: 'Alice Chen',
  email: 'alice@example.com',
}

function renderQuotaPools(user: AuthUser, capabilities = viewCapabilities) {
  useAuthStore.getState().auth.setUser(user)
  apiMocks.getSelfQuotaPool.mockResolvedValue({
    success: true,
    data: { pool, capabilities, admin_contacts: [adminContact] },
  })
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <QuotaPools />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  for (const mock of Object.values(apiMocks)) mock.mockReset()
})

afterEach(() => {
  useAuthStore.getState().auth.reset()
})

test('ordinary member opens the assigned pool directly without a list', async () => {
  renderQuotaPools({
    id: 1,
    username: 'member',
    role: 1,
    quota_pool_enabled: true,
    quota_pool_id: 7,
  })

  expect(await screen.findByText('平台保障部')).toBeInTheDocument()
  expect(
    screen.queryByRole('columnheader', { name: 'Quota pool' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Back to list' })
  ).not.toBeInTheDocument()
  expect(screen.getByText('Alice Chen')).toBeInTheDocument()
  expect(screen.getByText('alice@example.com')).toBeInTheDocument()
})

test('pool administrator navigates from list to detail and back', async () => {
  renderQuotaPools({
    id: 2,
    username: 'pool-admin',
    role: 1,
    quota_pool_enabled: true,
    quota_pool_id: 7,
    quota_pool_admin: { pool_id: 7 },
  })

  expect(
    await screen.findByRole('columnheader', { name: 'Quota pool' })
  ).toBeInTheDocument()
  expect(
    screen.queryByRole('tab', { name: 'Overview' })
  ).not.toBeInTheDocument()

  const poolRow = screen.getByText('平台保障部').closest('tr')
  if (!poolRow) throw new Error('Quota pool row was not rendered')
  poolRow.focus()
  fireEvent.click(poolRow)

  const backButton = await screen.findByRole('button', {
    name: 'Back to list',
  })
  expect(
    screen.queryByRole('button', {
      name: 'Switch quota pool: 平台保障部',
    })
  ).not.toBeInTheDocument()
  expect(backButton).toHaveFocus()
  expect(screen.getByRole('tab', { name: 'Overview' })).toBeInTheDocument()
  expect(
    screen.queryByRole('columnheader', { name: 'Quota pool' })
  ).not.toBeInTheDocument()

  fireEvent.click(backButton)

  await waitFor(() => {
    expect(
      screen.getByRole('columnheader', { name: 'Quota pool' })
    ).toBeInTheDocument()
    expect(screen.getByText('平台保障部').closest('tr')).toHaveFocus()
  })
})

test('global administrator searches and paginates quota pools on the server', async () => {
  apiMocks.getQuotaPools.mockResolvedValue({
    success: true,
    data: {
      items: [pool],
      total: 31,
      page: 1,
      page_size: 20,
      capabilities: viewCapabilities,
    },
  })
  renderQuotaPools({
    id: 3,
    username: 'admin',
    role: 10,
    quota_pool_enabled: true,
  })

  await waitFor(() =>
    expect(apiMocks.getQuotaPools).toHaveBeenCalledWith({
      page: 1,
      pageSize: 20,
      keyword: '',
    })
  )
  const search = await screen.findByRole('searchbox', {
    name: 'Search quota pools',
  })
  fireEvent.change(search, { target: { value: '保障' } })
  const searchButton = screen.getByRole('button', { name: 'Search' })
  searchButton.focus()
  fireEvent.click(searchButton)

  await waitFor(() =>
    expect(apiMocks.getQuotaPools).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      keyword: '保障',
    })
  )
  await screen.findByRole('searchbox', { name: 'Search quota pools' })
  expect(searchButton).toHaveFocus()
  fireEvent.click(await screen.findByRole('button', { name: 'Next page' }))
  await waitFor(() =>
    expect(apiMocks.getQuotaPools).toHaveBeenLastCalledWith({
      page: 2,
      pageSize: 20,
      keyword: '保障',
    })
  )

  const pageSize = await screen.findByRole('combobox', {
    name: 'Rows per page',
  })
  expect(pageSize).toHaveValue('20')
  expect(screen.getByRole('option', { name: '10 / page' })).toBeInTheDocument()
  expect(screen.getByRole('option', { name: '50 / page' })).toBeInTheDocument()
  fireEvent.change(pageSize, { target: { value: '50' } })
  await waitFor(() =>
    expect(apiMocks.getQuotaPools).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 50,
      keyword: '保障',
    })
  )
})

test('global administrator keeps search controls for an empty result', async () => {
  apiMocks.getQuotaPools.mockResolvedValue({
    success: true,
    data: {
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      capabilities: viewCapabilities,
    },
  })
  renderQuotaPools({
    id: 3,
    username: 'admin',
    role: 10,
    quota_pool_enabled: true,
  })

  expect(
    await screen.findByRole('searchbox', { name: 'Search quota pools' })
  ).toBeInTheDocument()
  expect(screen.getByText('No quota pools')).toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Previous page' })).toBeDisabled()
  expect(screen.getByRole('button', { name: 'Next page' })).toBeDisabled()
})

test('global administrator accepts a server-clamped page after totals shrink', async () => {
  let shrunk = false
  apiMocks.getQuotaPools.mockImplementation(
    async (options?: {
      page?: number
      pageSize?: number
      keyword?: string
    }) => {
      if (options?.page === 2) shrunk = true
      return {
        success: true,
        data: {
          items: [pool],
          total: shrunk ? 20 : 21,
          page: shrunk ? 1 : 1,
          page_size: 20,
          capabilities: viewCapabilities,
        },
      }
    }
  )
  renderQuotaPools({
    id: 3,
    username: 'admin',
    role: 10,
    quota_pool_enabled: true,
  })

  fireEvent.click(await screen.findByRole('button', { name: 'Next page' }))
  await waitFor(() =>
    expect(apiMocks.getQuotaPools).toHaveBeenCalledWith({
      page: 2,
      pageSize: 20,
      keyword: '',
    })
  )
  await waitFor(() =>
    expect(apiMocks.getQuotaPools).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      keyword: '',
    })
  )
  expect(screen.getByRole('button', { name: 'Next page' })).toBeDisabled()
})

test('global administrator searches and switches pools from the detail title', async () => {
  apiMocks.getQuotaPools.mockResolvedValueOnce({
    success: true,
    data: {
      items: [pool],
      total: 1,
      page: 1,
      page_size: 20,
      capabilities: viewCapabilities,
    },
  })
  apiMocks.getQuotaPools.mockResolvedValue({
    success: true,
    data: {
      items: [otherPool],
      total: 1,
      page: 1,
      page_size: 20,
      capabilities: viewCapabilities,
    },
  })
  apiMocks.getQuotaPool.mockImplementation(async (poolId: number) => ({
    success: true,
    data: {
      pool: poolId === otherPool.id ? otherPool : pool,
      capabilities: viewCapabilities,
    },
  }))
  renderQuotaPools({
    id: 3,
    username: 'admin',
    role: 10,
    quota_pool_enabled: true,
  })

  fireEvent.click(await screen.findByText('平台保障部'))
  const switcher = await screen.findByRole('button', {
    name: 'Switch quota pool: 平台保障部',
  })
  fireEvent.click(switcher)
  const search = await screen.findByPlaceholderText('Search by pool ID or name')
  fireEvent.change(search, { target: { value: '研发' } })
  await waitFor(() =>
    expect(apiMocks.getQuotaPools).toHaveBeenLastCalledWith({
      page: 1,
      pageSize: 20,
      keyword: '研发',
    })
  )
  fireEvent.click(await screen.findByRole('option', { name: /研发池/ }))

  await waitFor(() => expect(apiMocks.getQuotaPool).toHaveBeenCalledWith(8))
  expect(
    await screen.findByRole('button', {
      name: 'Switch quota pool: 研发池',
    })
  ).toHaveTextContent('研发池')

  fireEvent.click(screen.getByRole('button', { name: 'Back to list' }))
  await waitFor(() =>
    expect(
      screen.getByRole('searchbox', { name: 'Search quota pools' })
    ).toHaveFocus()
  )
})

test('global administrator cannot operate a pool when detail loading fails', async () => {
  apiMocks.getQuotaPools.mockResolvedValue({
    success: true,
    data: {
      items: [pool],
      total: 1,
      page: 1,
      page_size: 20,
      capabilities: {
        ...viewCapabilities,
        can_edit: true,
        can_manage_members: true,
      },
    },
  })
  apiMocks.getQuotaPool.mockResolvedValue({
    success: false,
    message: 'raw detail error',
  })
  renderQuotaPools({
    id: 3,
    username: 'admin',
    role: 10,
    quota_pool_enabled: true,
  })

  fireEvent.click(await screen.findByText('平台保障部'))

  expect(await screen.findByText('Failed to load')).toBeInTheDocument()
  expect(
    screen.queryByRole('tab', { name: 'Overview' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Add member' })
  ).not.toBeInTheDocument()
  expect(screen.queryByText('raw detail error')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
})

test('global administrator opens the add-member dialog from pool details', async () => {
  const manageCapabilities = {
    ...viewCapabilities,
    can_manage_members: true,
  }
  apiMocks.getQuotaPools.mockResolvedValue({
    success: true,
    data: {
      items: [pool],
      total: 1,
      page: 1,
      page_size: 20,
      capabilities: manageCapabilities,
    },
  })
  apiMocks.getQuotaPool.mockResolvedValue({
    success: true,
    data: { pool, capabilities: manageCapabilities },
  })
  apiMocks.getQuotaPoolCandidates.mockResolvedValue({
    success: true,
    data: { items: [], total: 0, page: 1, page_size: 20 },
  })
  renderQuotaPools({
    id: 3,
    username: 'admin',
    role: 10,
    quota_pool_enabled: true,
  })

  fireEvent.click(await screen.findByText('平台保障部'))
  fireEvent.click(await screen.findByRole('button', { name: 'Add member' }))

  expect(
    await screen.findByRole('dialog', { name: 'Add member' })
  ).toBeInTheDocument()
  expect(screen.getByRole('combobox', { name: 'User' })).toBeInTheDocument()
})

test.each([
  [100, 'normal', true],
  [10, 'normal', false],
  [2, 'normal', false],
  [100, 'default', false],
  [100, 'new_user', false],
] as const)(
  'role %s sees status action for %s pool: %s',
  async (role, poolType, visible) => {
    const selectedPool = { ...pool, pool_type: poolType }
    apiMocks.getQuotaPools.mockResolvedValue({
      success: true,
      data: { items: [selectedPool], total: 1, capabilities: viewCapabilities },
    })
    apiMocks.getQuotaPool.mockResolvedValue({
      success: true,
      data: { pool: selectedPool, capabilities: viewCapabilities },
    })
    renderQuotaPools({
      id: 3,
      username: 'operator',
      role,
      quota_pool_enabled: true,
    })
    fireEvent.click(await screen.findByText(pool.name))
    await screen.findByRole('tab', { name: 'Overview' })
    expect(Boolean(screen.queryByRole('button', { name: 'Disable' }))).toBe(
      visible
    )
  }
)

test('root confirms disable and restore, refreshing detail and list status', async () => {
  let enabled = true
  apiMocks.getQuotaPools.mockImplementation(async () => ({
    success: true,
    data: {
      items: [{ ...pool, enabled }],
      total: 1,
      capabilities: viewCapabilities,
    },
  }))
  apiMocks.getQuotaPool.mockImplementation(async () => ({
    success: true,
    data: { pool: { ...pool, enabled }, capabilities: viewCapabilities },
  }))
  apiMocks.setQuotaPoolEnabled.mockImplementation(async (_id, nextEnabled) => {
    enabled = nextEnabled
    return { success: true }
  })
  renderQuotaPools({
    id: 3,
    username: 'root',
    role: 100,
    quota_pool_enabled: true,
  })
  fireEvent.click(await screen.findByText(pool.name))
  fireEvent.click(await screen.findByRole('button', { name: 'Disable' }))
  fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
  expect(apiMocks.setQuotaPoolEnabled).not.toHaveBeenCalled()
  fireEvent.click(screen.getByRole('button', { name: 'Disable' }))
  fireEvent.click(screen.getByRole('button', { name: 'Confirm' }))
  await screen.findByRole('button', { name: 'Enable' })
  expect(apiMocks.setQuotaPoolEnabled).toHaveBeenCalledWith(pool.id, false)
  fireEvent.click(screen.getByRole('button', { name: 'Back to list' }))
  await screen.findByRole('cell', { name: 'Disabled' })
  fireEvent.click(screen.getByText(pool.name))
  fireEvent.click(await screen.findByRole('button', { name: 'Enable' }))
  fireEvent.click(screen.getByRole('button', { name: 'Confirm' }))
  await screen.findByRole('button', { name: 'Disable' })
  expect(apiMocks.setQuotaPoolEnabled).toHaveBeenLastCalledWith(pool.id, true)
})

test('failed status change keeps confirmation open and allows retry', async () => {
  apiMocks.getQuotaPools.mockResolvedValue({
    success: true,
    data: { items: [pool], total: 1, capabilities: viewCapabilities },
  })
  apiMocks.getQuotaPool.mockResolvedValue({
    success: true,
    data: { pool, capabilities: viewCapabilities },
  })
  let resolveRequest: (value: {
    success: boolean
    message: string
  }) => void = () => {}
  apiMocks.setQuotaPoolEnabled.mockImplementation(
    () =>
      new Promise((resolve) => {
        resolveRequest = resolve
      })
  )
  renderQuotaPools({
    id: 3,
    username: 'root',
    role: 100,
    quota_pool_enabled: true,
  })
  fireEvent.click(await screen.findByText(pool.name))
  fireEvent.click(await screen.findByRole('button', { name: 'Disable' }))
  fireEvent.click(screen.getByRole('button', { name: 'Confirm' }))
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Confirm' })).toBeDisabled()
  )
  expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled()
  resolveRequest({ success: false, message: 'Operation failed' })
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Confirm' })).toBeEnabled()
  )
  expect(screen.getByRole('alertdialog')).toBeInTheDocument()
  expect(apiMocks.setQuotaPoolEnabled).toHaveBeenCalledTimes(1)
  apiMocks.setQuotaPoolEnabled.mockResolvedValue({ success: true })
  fireEvent.click(screen.getByRole('button', { name: 'Confirm' }))
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
  expect(apiMocks.setQuotaPoolEnabled).toHaveBeenCalledTimes(2)
})
