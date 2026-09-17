import { render, screen, within } from '@testing-library/react'
import { expect, test } from 'vitest'

import { formatQuota } from '@/lib/format'

import type { QuotaPool } from '../../types'
import { QuotaPoolRechargeRules } from '../quota-pool-recharge-rules'

const unit = 500_000
const system = {
  enabled: true,
  threshold: 30 * unit,
  amount: 100 * unit,
  weekly_limit: 2,
  monthly_limit: 8,
  interval: 5,
}
const pool: QuotaPool = {
  id: 7,
  name: 'Team pool',
  pool_type: 'normal',
  enabled: true,
  is_default: false,
  base_quota: 1000 * unit,
  quota: 500 * unit,
  auto_recharge_amount: 35 * unit,
  weekly_limit: -1,
  monthly_limit: -1,
  monthly_refill_enabled: false,
  monthly_refill_top_up: false,
  monthly_refill_amount: 0,
  monthly_refill_day: 1,
  last_refill_month: 0,
  system_auto_recharge: system,
}

function rule(label: string) {
  const row = screen.getByText(label).parentElement
  if (!row) throw new Error('Recharge rule row not found')
  return within(row)
}

test('shows the pool amount instead of the system default, with inherited limits and an inclusive threshold', () => {
  render(<QuotaPoolRechargeRules pool={pool} />)
  expect(
    rule('Recharge amount').getByText(formatQuota(35 * unit))
  ).toBeInTheDocument()
  expect(
    rule('Recharge amount').getByText('Pool custom setting')
  ).toBeInTheDocument()
  expect(screen.queryByText(formatQuota(100 * unit))).not.toBeInTheDocument()
  expect(rule('Weekly limit').getByText('2')).toBeInTheDocument()
  expect(
    rule('Weekly limit').getByText('Inherit system setting')
  ).toBeInTheDocument()
  expect(rule('Monthly limit').getByText('8')).toBeInTheDocument()
  expect(
    rule('Recharge threshold').getByText(
      `Balance at or below ${formatQuota(30 * unit)}`
    )
  ).toBeInTheDocument()
  expect(rule('Interval in minutes').getByText('5')).toBeInTheDocument()
  expect(screen.queryByRole('button')).not.toBeInTheDocument()
  expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument()
})

test('normal pools can inherit amount while overriding unlimited weekly and finite monthly limits', () => {
  render(
    <QuotaPoolRechargeRules
      pool={{
        ...pool,
        auto_recharge_amount: -1,
        weekly_limit: 0,
        monthly_limit: 4,
      }}
    />
  )
  expect(
    rule('Recharge amount').getByText(formatQuota(100 * unit))
  ).toBeInTheDocument()
  expect(rule('Weekly limit').getByText('Unlimited')).toBeInTheDocument()
  expect(rule('Monthly limit').getByText('4')).toBeInTheDocument()
})

test('system default pools use all system rules and unlimited pool balance', () => {
  render(
    <QuotaPoolRechargeRules
      pool={{
        ...pool,
        pool_type: 'default',
        quota: -1,
        weekly_limit: 0,
        monthly_limit: 4,
      }}
    />
  )
  expect(
    rule('Recharge amount').getByText(formatQuota(100 * unit))
  ).toBeInTheDocument()
  expect(rule('Weekly limit').getByText('2')).toBeInTheDocument()
  expect(rule('Monthly limit').getByText('8')).toBeInTheDocument()
  expect(screen.getByText('Enabled')).toBeInTheDocument()
})

test.each([
  [{ enabled: false }, 'Pool disabled'],
  [{ system_auto_recharge: { ...system, enabled: false } }, 'System disabled'],
  [{ auto_recharge_amount: 0 }, 'Pool-level disabled'],
  [
    {
      auto_recharge_amount: -1,
      system_auto_recharge: { ...system, amount: 0 },
    },
    'Amount not configured',
  ],
  [{ quota: 34 * unit }, 'Pool quota is insufficient for one recharge.'],
] satisfies [Partial<QuotaPool>, string][])(
  'explains inactive automatic recharge for %j',
  (overrides, reason) => {
    render(<QuotaPoolRechargeRules pool={{ ...pool, ...overrides }} />)
    expect(screen.getByText(reason)).toBeInTheDocument()
    expect(screen.queryByText('Enabled')).not.toBeInTheDocument()
  }
)

test('new-user pools explain that automatic recharge is not available', () => {
  render(<QuotaPoolRechargeRules pool={{ ...pool, pool_type: 'new_user' }} />)
  expect(
    screen.getByText('New-user pools do not support automatic recharge.')
  ).toBeInTheDocument()
  expect(screen.queryByText('Recharge amount')).not.toBeInTheDocument()
})

test('missing system settings do not display fabricated effective values', () => {
  render(
    <QuotaPoolRechargeRules
      pool={{ ...pool, system_auto_recharge: undefined }}
    />
  )
  expect(screen.getByText('No data')).toBeInTheDocument()
  expect(screen.queryByText('Recharge amount')).not.toBeInTheDocument()
})
