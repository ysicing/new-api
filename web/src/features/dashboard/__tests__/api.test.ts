/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { getModelStatistics, getRechargeLeaderboard, getTopUsers } from '../api'

const apiMocks = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/lib/api', () => ({ api: apiMocks }))

describe('operations statistics API', () => {
  beforeEach(() => {
    apiMocks.get.mockReset()
  })

  test('rejects an unsuccessful top users business response', async () => {
    apiMocks.get.mockResolvedValue({
      data: { success: false, message: 'top users failed' },
    })

    await expect(getTopUsers()).rejects.toThrow('top users failed')
  })

  test('rejects an unsuccessful recharge leaderboard business response', async () => {
    apiMocks.get.mockResolvedValue({
      data: { success: false, message: 'recharge leaderboard failed' },
    })

    await expect(getRechargeLeaderboard()).rejects.toThrow(
      'recharge leaderboard failed'
    )
  })

  test('requests model statistics with period and scope', async () => {
    apiMocks.get.mockResolvedValue({
      data: { success: true, data: [] },
    })

    await getModelStatistics('month', 'self')

    expect(apiMocks.get).toHaveBeenCalledWith('/api/data/model-statistics', {
      params: { period: 'month', scope: 'self' },
    })
  })
})
