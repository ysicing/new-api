/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { Row } from '@tanstack/react-table'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

import type { User } from '../../types'
import { DataTableRowActions } from '../data-table-row-actions'
import { UsersProvider } from '../users-provider'

const quotaPoolApiMocks = vi.hoisted(() => ({
  getQuotaPools: vi.fn(),
  moveUserQuotaPool: vi.fn(),
}))

vi.mock('@/features/quota-pools/api', () => quotaPoolApiMocks)

const targetUser = {
  id: 12,
  username: 'alice',
  display_name: 'Alice Chen',
  quota: 100,
  quota_pool_id: 7,
  quota_pool_name: '平台保障部',
  used_quota: 0,
  request_count: 0,
  group: 'default',
  status: 1,
  role: 1,
  DeletedAt: null,
} as User

beforeEach(() => {
  quotaPoolApiMocks.getQuotaPools.mockReset()
  quotaPoolApiMocks.moveUserQuotaPool.mockReset()
  quotaPoolApiMocks.getQuotaPools.mockResolvedValue({
    success: true,
    data: {
      items: [
        {
          id: 1,
          name: '默认额度池',
          pool_type: 'default',
          enabled: true,
          is_default: true,
        },
        {
          id: 7,
          name: '平台保障部',
          pool_type: 'normal',
          enabled: true,
          is_default: false,
        },
        {
          id: 8,
          name: '研发池',
          pool_type: 'normal',
          enabled: true,
          is_default: false,
        },
        {
          id: 9,
          name: '停用池',
          pool_type: 'normal',
          enabled: false,
          is_default: false,
        },
      ],
      capabilities: {},
    },
  })
  quotaPoolApiMocks.moveUserQuotaPool.mockResolvedValue({ success: true })
  useAuthStore.getState().auth.setUser({
    id: 2,
    username: 'admin',
    role: 10,
    quota_pool_enabled: true,
  })
})

afterEach(() => {
  useAuthStore.getState().auth.reset()
})

function renderRowActions(user = targetUser) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <UsersProvider>
        <DataTableRowActions row={{ original: user } as Row<User>} />
      </UsersProvider>
    </QueryClientProvider>
  )
}

test('system admin moves a user to another enabled quota pool', async () => {
  const user = userEvent.setup()
  renderRowActions()

  await user.click(
    await screen.findByRole('button', { name: 'Move quota pool' })
  )

  const targetPool = await screen.findByRole('combobox', {
    name: 'Target quota pool',
  })
  await user.click(targetPool)
  expect(
    screen.queryByRole('option', { name: '平台保障部' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('option', { name: '停用池' })
  ).not.toBeInTheDocument()
  await user.click(await screen.findByRole('option', { name: '研发池' }))
  await user.click(screen.getByRole('button', { name: 'Confirm move' }))

  await waitFor(() =>
    expect(quotaPoolApiMocks.moveUserQuotaPool).toHaveBeenCalledWith(12, 8)
  )
})

test('maps the displayed default pool to the backend default pool ID', async () => {
  const user = userEvent.setup()
  renderRowActions()

  await user.click(screen.getByRole('button', { name: 'Move quota pool' }))
  await user.click(
    await screen.findByRole('combobox', { name: 'Target quota pool' })
  )
  await user.click(await screen.findByRole('option', { name: '默认额度池' }))
  await user.click(screen.getByRole('button', { name: 'Confirm move' }))

  await waitFor(() =>
    expect(quotaPoolApiMocks.moveUserQuotaPool).toHaveBeenCalledWith(12, 0)
  )
})

test('hides migration when disabled or the target role cannot join a pool', () => {
  useAuthStore.getState().auth.setUser({
    id: 2,
    username: 'admin',
    role: 10,
    quota_pool_enabled: false,
  })
  const disabledFeature = renderRowActions()
  expect(
    screen.queryByRole('button', { name: 'Move quota pool' })
  ).not.toBeInTheDocument()

  disabledFeature.unmount()
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'root',
    role: 100,
    quota_pool_enabled: true,
  })
  const rootTarget = renderRowActions({ ...targetUser, role: 100 })
  expect(
    screen.queryByRole('button', { name: 'Move quota pool' })
  ).not.toBeInTheDocument()

  rootTarget.unmount()
  renderRowActions({ ...targetUser, role: 0 })
  expect(
    screen.queryByRole('button', { name: 'Move quota pool' })
  ).not.toBeInTheDocument()
})

test('announces quota pool loading failures', async () => {
  quotaPoolApiMocks.getQuotaPools.mockResolvedValue({
    success: false,
    message: 'raw server error',
  })
  const user = userEvent.setup()
  renderRowActions()

  await user.click(screen.getByRole('button', { name: 'Move quota pool' }))

  expect(
    await screen.findByText('Failed to load quota pools.')
  ).toHaveAttribute('role', 'alert')
  expect(screen.queryByText('raw server error')).not.toBeInTheDocument()
})
