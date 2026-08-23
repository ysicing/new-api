/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { beforeEach, expect, test, vi } from 'vitest'

import { searchLDAPUsers, syncLDAPCandidate } from '../api'
import type { LDAPSyncCandidate } from '../types'

const apiMocks = vi.hoisted(() => ({
  post: vi.fn(),
}))

vi.mock('@/lib/api', () => ({ api: apiMocks }))

const candidate: LDAPSyncCandidate = {
  key: 'alice@example.com',
  username: 'alice',
  email: 'alice@example.com',
  display_name: 'Alice Chen',
  department: '平台保障部',
  ldap_id: 'alice@example.com',
  signature: 'signed-candidate',
}

beforeEach(() => {
  apiMocks.post.mockReset()
  apiMocks.post.mockResolvedValue({ data: { success: true, data: {} } })
})

test('searches LDAP through the admin sync endpoint', async () => {
  await searchLDAPUsers('alice@example.com')

  expect(apiMocks.post).toHaveBeenCalledWith('/api/user/ldap/sync', {
    action: 'search',
    username: 'alice@example.com',
  })
})

test('submits the signed LDAP candidate unchanged', async () => {
  await syncLDAPCandidate(candidate)

  expect(apiMocks.post).toHaveBeenCalledWith('/api/user/ldap/sync', {
    action: 'sync',
    user: candidate,
  })
})
