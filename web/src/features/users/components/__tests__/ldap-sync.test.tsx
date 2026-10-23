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

import type { LDAPSyncCandidate } from '../../types'
import { LDAPSyncDrawer } from '../dialogs/ldap-sync-drawer'
import { UsersPrimaryButtons } from '../users-primary-buttons'
import { UsersProvider, useUsers } from '../users-provider'

const statusMocks = vi.hoisted(() => ({
  useStatus: vi.fn(),
}))
const apiMocks = vi.hoisted(() => ({
  searchLDAPUsers: vi.fn(),
  syncLDAPCandidate: vi.fn(),
}))
const toastMocks = vi.hoisted(() => ({
  error: vi.fn(),
  success: vi.fn(),
}))

vi.mock('@/hooks/use-status', () => statusMocks)
vi.mock('../../api', () => apiMocks)
vi.mock('sonner', () => ({ toast: toastMocks }))

const ldapCandidate: LDAPSyncCandidate = {
  key: 'alice@example.com',
  username: 'alice',
  email: 'alice@example.com',
  display_name: 'Alice Chen',
  department: '平台保障部',
  ldap_id: 'alice@example.com',
  signature: 'signed-candidate',
}

function OpenState() {
  const { open } = useUsers()
  return <output aria-label='Open user dialog'>{open ?? 'closed'}</output>
}

function RefreshState() {
  const { refreshTrigger } = useUsers()
  return <output aria-label='User refresh count'>{refreshTrigger}</output>
}

function renderPrimaryButtons() {
  return render(
    <UsersProvider>
      <UsersPrimaryButtons />
      <OpenState />
    </UsersProvider>
  )
}

beforeEach(() => {
  statusMocks.useStatus.mockReset()
  apiMocks.searchLDAPUsers.mockReset()
  apiMocks.syncLDAPCandidate.mockReset()
  toastMocks.error.mockReset()
  toastMocks.success.mockReset()
})

test('opens LDAP sync from user management when LDAP login is enabled', async () => {
  statusMocks.useStatus.mockReturnValue({
    status: { ldap_login: true },
    loading: false,
  })
  const user = userEvent.setup()
  renderPrimaryButtons()

  await user.click(screen.getByRole('button', { name: 'Sync LDAP User' }))

  expect(screen.getByLabelText('Open user dialog')).toHaveTextContent(
    'ldap-sync'
  )
})

test('hides LDAP sync when LDAP login is disabled', () => {
  statusMocks.useStatus.mockReturnValue({
    status: { ldap_login: false },
    loading: false,
  })
  renderPrimaryButtons()

  expect(
    screen.queryByRole('button', { name: 'Sync LDAP User' })
  ).not.toBeInTheDocument()
})

test('searches, selects and synchronizes a signed LDAP candidate', async () => {
  apiMocks.searchLDAPUsers.mockResolvedValue({
    success: true,
    data: { users: [ldapCandidate], total: 1 },
  })
  apiMocks.syncLDAPCandidate.mockResolvedValue({
    success: true,
    data: { id: 12, username: 'alice' },
  })
  const onOpenChange = vi.fn()
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const user = userEvent.setup()
  render(
    <QueryClientProvider client={queryClient}>
      <UsersProvider>
        <LDAPSyncDrawer open onOpenChange={onOpenChange} />
        <RefreshState />
      </UsersProvider>
    </QueryClientProvider>
  )

  await user.type(
    screen.getByRole('textbox', { name: 'LDAP username or email' }),
    'alice'
  )
  await user.click(screen.getByRole('button', { name: 'Search LDAP' }))
  await user.click(
    await screen.findByRole('radio', {
      name: /alice.*Alice Chen.*alice@example.com.*平台保障部/,
    })
  )
  await user.click(screen.getByRole('button', { name: 'Sync selected' }))

  expect(apiMocks.searchLDAPUsers).toHaveBeenCalledWith('alice')
  expect(apiMocks.syncLDAPCandidate).toHaveBeenCalledWith(ldapCandidate)
  await waitFor(() => expect(onOpenChange).toHaveBeenCalledWith(false))
  expect(screen.getByLabelText('User refresh count')).toHaveTextContent('1')
  expect(toastMocks.success).toHaveBeenCalledWith('LDAP user synchronized')
})
