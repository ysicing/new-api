/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { render, screen, within } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { formatQuota } from '@/lib/format'

import type { QuotaPool } from '../../types'
import { QuotaPoolList } from '../quota-pool-list'

const systemAutoRecharge = {
  enabled: true,
  interval: 30,
  threshold: 25_000_000,
  amount: 100_000_000,
  weekly_limit: 3,
  monthly_limit: 10,
}

const basePool: QuotaPool = {
  id: 1,
  name: '全部继承池',
  pool_type: 'normal',
  enabled: true,
  is_default: false,
  base_quota: 1_000_000_000,
  quota: 800_000_000,
  auto_recharge_amount: -1,
  weekly_limit: -1,
  monthly_limit: -1,
  monthly_refill_enabled: false,
  monthly_refill_top_up: false,
  monthly_refill_amount: 0,
  monthly_refill_day: 1,
  last_refill_month: 0,
  member_count: 1,
  system_auto_recharge: systemAutoRecharge,
}

function renderList(pools: QuotaPool[]) {
  render(<QuotaPoolList pools={pools} onSelect={vi.fn()} />)
}

function row(name: string) {
  const element = screen.getByText(name).closest('tr')
  if (!element) throw new Error(`Missing row for ${name}`)
  return within(element)
}

test('shows effective automatic recharge state and override source', () => {
  renderList([
    basePool,
    {
      ...basePool,
      id: 2,
      name: '部分自定义池',
      auto_recharge_amount: 50_000_000,
      monthly_limit: 0,
    },
    {
      ...basePool,
      id: 3,
      name: '全部自定义池',
      auto_recharge_amount: 50_000_000,
      weekly_limit: 0,
      monthly_limit: 0,
    },
    {
      ...basePool,
      id: 4,
      name: '池级关闭池',
      auto_recharge_amount: 0,
    },
  ])

  expect(
    screen.getByRole('columnheader', { name: 'Automatic recharge' })
  ).toBeInTheDocument()
  expect(
    row('全部继承池').getByRole('cell', {
      name: 'Automatic recharge: Enabled',
    })
  ).toHaveTextContent('All inherited')
  expect(
    row('全部继承池').getByText(formatQuota(100_000_000))
  ).toBeInTheDocument()
  expect(
    row('部分自定义池').getByText('Partially customized')
  ).toBeInTheDocument()
  expect(row('全部自定义池').getByText('Fully customized')).toBeInTheDocument()
  expect(
    row('池级关闭池').getByRole('cell', {
      name: 'Automatic recharge: Disabled',
    })
  ).toHaveTextContent('Pool-level disabled')
})

test('explains system, pool, default-pool and new-user-pool states', () => {
  renderList([
    {
      ...basePool,
      id: 5,
      name: '系统关闭池',
      auto_recharge_amount: 50_000_000,
      system_auto_recharge: { ...systemAutoRecharge, enabled: false },
    },
    { ...basePool, id: 6, name: '额度池停用', enabled: false },
    {
      ...basePool,
      id: 7,
      name: '系统默认池',
      pool_type: 'default',
      is_default: true,
    },
    {
      ...basePool,
      id: 8,
      name: '新用户池',
      pool_type: 'new_user',
      auto_recharge_amount: 0,
    },
  ])

  expect(row('系统关闭池').getByText('System disabled')).toBeInTheDocument()
  expect(
    row('系统关闭池').getByText('Partially customized')
  ).toBeInTheDocument()
  expect(row('额度池停用').getByText('Pool disabled')).toBeInTheDocument()
  expect(row('系统默认池').getByText('System setting')).toBeInTheDocument()
  expect(
    row('新用户池').getByRole('cell', {
      name: 'Automatic recharge: Not applicable',
    })
  ).toHaveTextContent('New-user pool')
})
