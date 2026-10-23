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
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import type { User } from '../../types'
import { UsersMutateDrawer } from '../users-mutate-drawer'
import { UsersProvider } from '../users-provider'

const target: User = {
  id: 40,
  username: 'alice',
  display_name: 'Alice',
  role: 1,
  status: 1,
  quota: 5645000,
  used_quota: 0,
  request_count: 0,
  group: 'default',
  quota_pool_id: 7,
}

const clients: QueryClient[] = []
afterEach(() => {
  clients.forEach((client) => client.clear())
  clients.length = 0
  useAuthStore.getState().auth.reset()
})

function renderDrawer(detail: Promise<unknown>) {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'root', role: 100 })
  vi.spyOn(api, 'get').mockImplementation((url) => {
    if (url === '/api/user/40') return detail as ReturnType<typeof api.get>
    if (url === '/api/group/') {
      return Promise.resolve({ data: { success: true, data: ['default'] } })
    }
    return Promise.resolve({
      data: { success: true, data: { resources: [], actions: [] } },
    })
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  clients.push(client)
  render(
    <QueryClientProvider client={client}>
      <UsersProvider>
        <UsersMutateDrawer open onOpenChange={vi.fn()} currentRow={target} />
      </UsersProvider>
    </QueryClientProvider>
  )
  return client
}

test('allowed user detail exposes adjustment and sends subtract request', async () => {
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: true } })
  renderDrawer(
    Promise.resolve({
      data: { success: true, data: { ...target, can_adjust_quota: true } },
    })
  )
  const interaction = userEvent.setup()
  await interaction.click(
    await screen.findByRole('button', { name: 'Adjust Quota' })
  )
  await interaction.click(screen.getByRole('button', { name: 'Subtract' }))
  await interaction.type(screen.getByRole('spinbutton'), '10')
  await interaction.click(screen.getByRole('button', { name: 'Confirm' }))
  await waitFor(() =>
    expect(post).toHaveBeenCalledWith('/api/user/manage', {
      id: 40,
      action: 'add_quota',
      mode: 'subtract',
      value: 5000000,
    })
  )
})

test.each([false, undefined])(
  'denied or absent capability %s hides quota adjustment',
  async (canAdjustQuota) => {
    renderDrawer(
      Promise.resolve({
        data: {
          success: true,
          data: { ...target, can_adjust_quota: canAdjustQuota },
        },
      })
    )
    await waitFor(() =>
      expect(screen.getByRole('textbox', { name: 'Username' })).toHaveValue(
        'alice'
      )
    )
    expect(
      screen.queryByRole('button', { name: 'Adjust Quota' })
    ).not.toBeInTheDocument()
  }
)

test('pending detail cannot expose adjustment until permission is resolved', async () => {
  let resolve!: (value: unknown) => void
  renderDrawer(
    new Promise((done) => {
      resolve = done
    })
  )
  expect(
    screen.queryByRole('button', { name: 'Adjust Quota' })
  ).not.toBeInTheDocument()
  resolve({
    data: { success: true, data: { ...target, can_adjust_quota: true } },
  })
  expect(
    await screen.findByRole('button', { name: 'Adjust Quota' })
  ).toBeEnabled()
})

test('failed user detail keeps quota adjustment unavailable', async () => {
  const get = vi.spyOn(api, 'get')
  renderDrawer(Promise.reject(new Error('network unavailable')))
  await waitFor(() => expect(get).toHaveBeenCalledWith('/api/user/40'))
  expect(
    screen.queryByRole('button', { name: 'Adjust Quota' })
  ).not.toBeInTheDocument()
})

test('failed refresh hides a previously allowed adjustment', async () => {
  const client = renderDrawer(
    Promise.resolve({
      data: { success: true, data: { ...target, can_adjust_quota: true } },
    })
  )
  await screen.findByRole('button', { name: 'Adjust Quota' })
  vi.spyOn(api, 'get').mockRejectedValue(new Error('network unavailable'))
  await client.refetchQueries({ queryKey: ['users', 'detail'] })
  await waitFor(() =>
    expect(
      screen.queryByRole('button', { name: 'Adjust Quota' })
    ).not.toBeInTheDocument()
  )
})
