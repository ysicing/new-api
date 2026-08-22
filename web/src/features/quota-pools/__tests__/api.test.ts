/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { beforeEach, describe, expect, test, vi } from 'vitest'

import {
  getQuotaPoolCandidates,
  getQuotaPoolMembers,
  moveUserQuotaPool,
  reclaimQuotaPoolMember,
} from '../api'

const apiMocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}))

vi.mock('@/lib/api', () => ({ api: apiMocks }))

describe('quota pool members API', () => {
  beforeEach(() => {
    apiMocks.get.mockReset()
    apiMocks.post.mockReset()
    apiMocks.put.mockReset()
    apiMocks.get.mockResolvedValue({ data: { success: true, data: {} } })
    apiMocks.post.mockResolvedValue({ data: { success: true, data: {} } })
    apiMocks.put.mockResolvedValue({ data: { success: true, data: {} } })
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

  test('searches eligible member candidates through the matching endpoint', async () => {
    await getQuotaPoolCandidates(false, {
      page: 1,
      pageSize: 20,
      keyword: 'alice',
    })
    await getQuotaPoolCandidates(true, {
      page: 1,
      pageSize: 20,
      keyword: '平台部',
    })

    expect(apiMocks.get).toHaveBeenNthCalledWith(
      1,
      '/api/quota_pool/candidates',
      { params: { p: 1, page_size: 20, keyword: 'alice' } }
    )
    expect(apiMocks.get).toHaveBeenNthCalledWith(
      2,
      '/api/quota_pool/self/candidates',
      { params: { p: 1, page_size: 20, keyword: '平台部' } }
    )
  })

  test('passes the selected reclaim amount to the backend', async () => {
    await reclaimQuotaPoolMember(7, 3, 250, false)

    expect(apiMocks.post).toHaveBeenCalledWith(
      '/api/quota_pool/7/members/3/reclaim',
      { amount: 250 }
    )
  })

  test('moves a user to the selected quota pool', async () => {
    await moveUserQuotaPool(12, 7)

    expect(apiMocks.put).toHaveBeenCalledWith('/api/quota_pool/users/12', {
      pool_id: 7,
    })
  })
})
