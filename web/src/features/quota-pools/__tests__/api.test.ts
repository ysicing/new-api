/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { getQuotaPoolMembers, reclaimQuotaPoolMember } from '../api'

const apiMocks = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))

vi.mock('@/lib/api', () => ({ api: apiMocks }))

describe('quota pool members API', () => {
  beforeEach(() => {
    apiMocks.get.mockReset()
    apiMocks.post.mockReset()
    apiMocks.get.mockResolvedValue({ data: { success: true, data: {} } })
    apiMocks.post.mockResolvedValue({ data: { success: true, data: {} } })
  })

  test('passes pagination and search parameters to the global endpoint', async () => {
    await getQuotaPoolMembers(7, false, {
      page: 2,
      pageSize: 20,
      keyword: 'alice',
    })

    expect(apiMocks.get).toHaveBeenCalledWith('/api/quota_pool/7/members', {
      params: { p: 2, page_size: 20, keyword: 'alice' },
    })
  })

  test('uses the same parameters for the self endpoint', async () => {
    await getQuotaPoolMembers(7, true, {
      page: 1,
      pageSize: 10,
      keyword: '',
    })

    expect(apiMocks.get).toHaveBeenCalledWith('/api/quota_pool/self/members', {
      params: { p: 1, page_size: 10, keyword: '' },
    })
  })

  test('passes the selected reclaim amount to the backend', async () => {
    await reclaimQuotaPoolMember(7, 3, 250, false)

    expect(apiMocks.post).toHaveBeenCalledWith(
      '/api/quota_pool/7/members/3/reclaim',
      { amount: 250 }
    )
  })
})
