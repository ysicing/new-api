/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { renderHook, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'

import { useAffiliate } from '../use-affiliate'

const apiMocks = vi.hoisted(() => ({
  getAffiliateCode: vi.fn(),
  transferAffiliateQuota: vi.fn(),
}))

vi.mock('../../api', () => apiMocks)
vi.mock('@/lib/api', () => ({ getSelf: vi.fn() }))
vi.mock('@/hooks/use-copy-to-clipboard', () => ({
  useCopyToClipboard: () => ({ copyToClipboard: vi.fn() }),
}))

beforeEach(() => {
  apiMocks.getAffiliateCode.mockReset()
  apiMocks.getAffiliateCode.mockResolvedValue({
    success: true,
    data: 'invite-code',
  })
  apiMocks.transferAffiliateQuota.mockReset()
})

test('does not request an affiliate code when referrals are disabled', async () => {
  const { result } = renderHook(() => useAffiliate(false))

  await waitFor(() => expect(result.current.loading).toBe(false))
  expect(apiMocks.getAffiliateCode).not.toHaveBeenCalled()
  expect(result.current.affiliateLink).toBe('')
})

test('loads the affiliate link when referrals are enabled', async () => {
  const { result } = renderHook(() => useAffiliate(true))

  await waitFor(() => expect(result.current.loading).toBe(false))
  expect(apiMocks.getAffiliateCode).toHaveBeenCalledOnce()
  expect(result.current.affiliateLink).toContain('/sign-up?aff=invite-code')
})
