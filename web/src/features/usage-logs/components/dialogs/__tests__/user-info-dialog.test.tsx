/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { render, screen } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

import { UserInfoDialog } from '../user-info-dialog'

const apiMocks = vi.hoisted(() => ({ getUserInfo: vi.fn() }))
const statusMocks = vi.hoisted(() => ({ useStatus: vi.fn() }))

vi.mock('../../../api', () => apiMocks)
vi.mock('@/hooks/use-status', () => statusMocks)

const userInfo = {
  id: 12,
  username: 'alice',
  display_name: 'Alice Chen',
  quota: 100,
  used_quota: 50,
  request_count: 10,
  group: 'default',
  quota_pool_id: 7,
  quota_pool_name: '平台保障部',
  quota_pool_enabled: true,
}

beforeEach(() => {
  apiMocks.getUserInfo.mockReset()
  apiMocks.getUserInfo.mockResolvedValue({ success: true, data: userInfo })
  statusMocks.useStatus.mockReset()
  statusMocks.useStatus.mockReturnValue({
    status: { self_use_mode_enabled: false },
    loading: false,
  })
})

test('hides invitation details in self-use mode', async () => {
  statusMocks.useStatus.mockReturnValue({
    status: { self_use_mode_enabled: true },
    loading: false,
  })
  apiMocks.getUserInfo.mockResolvedValue({
    success: true,
    data: {
      ...userInfo,
      aff_code: 'invite-code',
      aff_count: 3,
      aff_quota: 1000,
    },
  })

  render(<UserInfoDialog userId={12} open onOpenChange={() => undefined} />)

  expect(await screen.findByText('alice')).toBeInTheDocument()
  expect(screen.queryByText('Invitation Code')).not.toBeInTheDocument()
  expect(screen.queryByText('Invited Users')).not.toBeInTheDocument()
  expect(screen.queryByText('Invitation Quota')).not.toBeInTheDocument()
  expect(
    screen.queryByText(
      'View detailed information about this user including balance, usage statistics, and invitation details.'
    )
  ).not.toBeInTheDocument()
})

afterEach(() => {
  useAuthStore.getState().auth.reset()
})

test('shows the pool name and ID when quota pools are enabled', async () => {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'admin',
    role: 10,
    quota_pool_enabled: false,
  })

  render(<UserInfoDialog userId={12} open onOpenChange={() => undefined} />)

  expect(await screen.findByText('Quota pool')).toBeInTheDocument()
  expect(screen.getByText('平台保障部 (#7)')).toBeInTheDocument()
})

test('shows the default pool without a synthetic ID', async () => {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'admin',
    role: 10,
    quota_pool_enabled: false,
  })
  apiMocks.getUserInfo.mockResolvedValue({
    success: true,
    data: { ...userInfo, quota_pool_id: 0, quota_pool_name: '' },
  })

  render(<UserInfoDialog userId={12} open onOpenChange={() => undefined} />)

  expect(await screen.findByText('Quota pool')).toBeInTheDocument()
  expect(screen.getByText('Default pool')).toBeInTheDocument()
})

test('hides quota pool information when the feature is disabled', async () => {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'admin',
    role: 10,
    quota_pool_enabled: true,
  })
  apiMocks.getUserInfo.mockResolvedValue({
    success: true,
    data: { ...userInfo, quota_pool_enabled: false },
  })

  render(<UserInfoDialog userId={12} open onOpenChange={() => undefined} />)

  expect(await screen.findByText('alice')).toBeInTheDocument()
  expect(screen.queryByText('Quota pool')).not.toBeInTheDocument()
  expect(screen.queryByText('平台保障部 (#7)')).not.toBeInTheDocument()
})

test('hides a non-default pool when its name is unavailable', async () => {
  apiMocks.getUserInfo.mockResolvedValue({
    success: true,
    data: { ...userInfo, quota_pool_name: '' },
  })

  render(<UserInfoDialog userId={12} open onOpenChange={() => undefined} />)

  expect(await screen.findByText('alice')).toBeInTheDocument()
  expect(screen.queryByText('Quota pool')).not.toBeInTheDocument()
  expect(screen.queryByText('Quota pool (#7)')).not.toBeInTheDocument()
})

test('wraps a long quota pool name inside the information grid', async () => {
  const longName = '平台保障部'.repeat(12)
  apiMocks.getUserInfo.mockResolvedValue({
    success: true,
    data: { ...userInfo, quota_pool_name: longName },
  })

  render(<UserInfoDialog userId={12} open onOpenChange={() => undefined} />)

  const value = await screen.findByText(`${longName} (#7)`)
  expect(value).toHaveClass('break-words')
  expect(value.parentElement).toHaveClass('min-w-0')
})
