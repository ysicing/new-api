/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'

import { Users } from '../../index'

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: { ldap_login: true }, loading: false }),
}))
vi.mock('../users-table', () => ({ UsersTable: () => <div>User table</div> }))
vi.mock('../users-mutate-drawer', () => ({
  UsersMutateDrawer: () => null,
}))
vi.mock('../users-delete-dialog', () => ({ UsersDeleteDialog: () => null }))
vi.mock('../../api', async () => {
  const actual = await vi.importActual<typeof import('../../api')>('../../api')
  return {
    ...actual,
    searchLDAPUsers: vi.fn(),
    syncLDAPCandidate: vi.fn(),
  }
})

test('mounts the LDAP sync sheet from the user page action', async () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const user = userEvent.setup()
  render(
    <QueryClientProvider client={queryClient}>
      <Users />
    </QueryClientProvider>
  )

  await user.click(screen.getByRole('button', { name: 'Sync LDAP User' }))

  expect(
    await screen.findByRole('dialog', { name: 'Sync LDAP User' })
  ).toBeInTheDocument()
})
