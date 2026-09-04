/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { beforeEach, expect, test, vi } from 'vitest'

import { getSelfAutoRechargeEligibility } from '../api'

const apiMocks = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/lib/api', () => ({ api: apiMocks }))

beforeEach(() => {
  apiMocks.get.mockReset()
})

test('requests the authenticated user automatic recharge eligibility snapshot', async () => {
  const response = {
    success: true,
    data: {
      status: 'eligible',
      eligible: true,
      user_quota: 100_000,
      threshold: 1_000_000,
      amount: 500_000,
      pool_name: 'Platform pool',
      pool_type: 'normal',
      weekly: { used: 0, limit: 0 },
      monthly: { used: 0, limit: 0 },
    },
  }
  apiMocks.get.mockResolvedValue({ data: response })

  await expect(getSelfAutoRechargeEligibility()).resolves.toEqual(response)
  expect(apiMocks.get).toHaveBeenCalledWith(
    '/api/user/auto_recharge/eligibility',
    { skipBusinessError: true, skipErrorHandler: true }
  )
})
