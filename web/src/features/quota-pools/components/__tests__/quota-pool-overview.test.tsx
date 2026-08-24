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
import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'

import type { QuotaPool } from '../../types'
import { PoolOverview } from '../quota-pool-data'

const systemPool: QuotaPool = {
  id: 9,
  name: '新用户额度池',
  pool_type: 'new_user',
  enabled: true,
  is_default: false,
  base_quota: -1,
  quota: -1,
  auto_recharge_amount: 0,
  weekly_limit: 0,
  monthly_limit: 0,
  monthly_refill_enabled: false,
  monthly_refill_top_up: false,
  monthly_refill_amount: 0,
  monthly_refill_day: 1,
  last_refill_month: 0,
}

test('new-user pool marks pool-level quota values as not applicable', () => {
  render(<PoolOverview pool={systemPool} />)

  expect(screen.getAllByText('Not applicable')).toHaveLength(2)
  expect(screen.queryByText('Unlimited')).not.toBeInTheDocument()
})

test('default pool keeps unlimited pool-level quota values', () => {
  render(
    <PoolOverview
      pool={{ ...systemPool, pool_type: 'default', is_default: true }}
    />
  )

  expect(screen.getAllByText('Unlimited')).toHaveLength(2)
  expect(screen.queryByText('Not applicable')).not.toBeInTheDocument()
})
