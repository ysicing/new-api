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
import { expect, test, vi } from 'vitest'

import { getAutoRechargeEligibility } from '../api'

const apiMocks = vi.hoisted(() => ({ post: vi.fn() }))

vi.mock('@/lib/api', () => ({ api: apiMocks }))

test('sends the user identifier in the request body instead of the URL', async () => {
  apiMocks.post.mockResolvedValue({ data: { success: true, data: {} } })

  await getAutoRechargeEligibility('alice@example.com')

  expect(apiMocks.post).toHaveBeenCalledWith(
    '/api/quota_pool/recharge_query/eligibility',
    { identifier: 'alice@example.com' },
    { skipErrorHandler: true }
  )
})
