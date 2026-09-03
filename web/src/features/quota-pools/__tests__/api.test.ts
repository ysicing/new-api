/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { beforeEach, describe, expect, test, vi } from 'vitest'

import {
  exportQuotaPoolStats,
  getQuotaPoolCandidates,
  getQuotaPoolMembers,
  getQuotaPool,
  getQuotaPools,
  getQuotaPoolStats,
  moveUserQuotaPool,
  rechargeQuotaPoolMember,
  removeQuotaPoolMember,
  reclaimQuotaPoolMember,
  setQuotaPoolAdmin,
} from '../api'

const apiMocks = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  delete: vi.fn(),
}))

vi.mock('@/lib/api', () => ({ api: apiMocks }))

describe('quota pool members API', () => {
  beforeEach(() => {
    apiMocks.get.mockReset()
    apiMocks.post.mockReset()
    apiMocks.put.mockReset()
    apiMocks.delete.mockReset()
    apiMocks.get.mockResolvedValue({ data: { success: true, data: {} } })
    apiMocks.post.mockResolvedValue({ data: { success: true, data: {} } })
    apiMocks.put.mockResolvedValue({ data: { success: true, data: {} } })
    apiMocks.delete.mockResolvedValue({ data: { success: true, data: {} } })
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

  test('passes pagination and search parameters to the pool list endpoint', async () => {
    await getQuotaPools({ page: 2, pageSize: 20, keyword: '保障' })

    expect(apiMocks.get).toHaveBeenCalledWith('/api/quota_pool/', {
      params: { p: 2, page_size: 20, keyword: '保障' },
    })
  })

  test('keeps the unpaginated pool list call backward compatible', async () => {
    await getQuotaPools()

    expect(apiMocks.get).toHaveBeenCalledWith('/api/quota_pool/')
  })

  test('loads a single quota pool for detail switching', async () => {
    await getQuotaPool(8)

    expect(apiMocks.get).toHaveBeenCalledWith('/api/quota_pool/8')
  })

  test('passes the selected reclaim amount to the backend', async () => {
    await reclaimQuotaPoolMember(7, 3, 250, false)

    expect(apiMocks.post).toHaveBeenCalledWith(
      '/api/quota_pool/7/members/3/reclaim',
      { amount: 250 }
    )
  })

  test('passes the selected recharge amount to the backend', async () => {
    await rechargeQuotaPoolMember(7, 3, 10_000_000, false)

    expect(apiMocks.post).toHaveBeenCalledWith(
      '/api/quota_pool/7/members/3/recharge',
      { amount: 10_000_000 }
    )
  })

  test('moves a user to the selected quota pool', async () => {
    await moveUserQuotaPool(12, 7)

    expect(apiMocks.put).toHaveBeenCalledWith('/api/quota_pool/users/12', {
      pool_id: 7,
    })
  })

  test('removes members through global and self endpoints', async () => {
    await removeQuotaPoolMember(7, 3, false)
    await removeQuotaPoolMember(7, 4, true)

    expect(apiMocks.delete).toHaveBeenNthCalledWith(
      1,
      '/api/quota_pool/7/members/3'
    )
    expect(apiMocks.delete).toHaveBeenNthCalledWith(
      2,
      '/api/quota_pool/self/members/4'
    )
  })

  test('sets a unified pool administrator without a level', async () => {
    await setQuotaPoolAdmin(7, 3)

    expect(apiMocks.post).toHaveBeenCalledWith('/api/quota_pool/7/admins', {
      user_id: 3,
    })
  })

  test('passes custom statistics range and export format to matching endpoints', async () => {
    const range = {
      range_type: 'custom' as const,
      start_date: '2026-08-01',
      end_date: '2026-08-31',
    }
    await getQuotaPoolStats(7, false, range)
    apiMocks.get.mockResolvedValueOnce({
      data: new Blob(['report']),
      headers: {
        'content-disposition':
          "attachment; filename*=UTF-8''%E4%BA%A7%E7%A0%94.xlsx",
      },
    })

    const exported = await exportQuotaPoolStats(7, true, range, 'xlsx')

    expect(apiMocks.get).toHaveBeenNthCalledWith(1, '/api/quota_pool/7/stats', {
      params: range,
    })
    expect(apiMocks.get).toHaveBeenNthCalledWith(
      2,
      '/api/quota_pool/self/stats/export',
      { params: { ...range, format: 'xlsx' }, responseType: 'blob' }
    )
    expect(exported.filename).toBe('产研.xlsx')
  })
})
