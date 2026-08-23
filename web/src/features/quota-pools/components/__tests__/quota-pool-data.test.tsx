/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import type { UseQueryResult } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'

import type { ApiResponse, PageData, QuotaPoolTransaction } from '../../types'
import { PoolTransactions } from '../quota-pool-data'

const typeLabels = [
  ['initial_fund', 'Initial funding'],
  ['manual_refill', 'Temporary refill'],
  ['monthly_refill', 'Monthly automatic refill'],
  ['allocate_auto', 'Automatic allocation'],
  ['allocate_manual', 'Manual allocation'],
  ['reclaim_user', 'Reclaimed user quota'],
  ['adjust_base_quota', 'Base quota adjustment'],
] as const

test('renders localized labels for every quota pool transaction type', () => {
  const items = typeLabels.map(([type], index) => ({
    id: index + 1,
    pool_id: 7,
    type,
    amount: 100,
    quota_before: 100,
    quota_after: 200,
    user_id: 1,
    operator_id: 9,
    user_name: 'alice',
    operator_name: 'admin',
    created_at: 1_700_000_000,
  }))
  const query = {
    isLoading: false,
    isError: false,
    data: {
      success: true,
      data: { items, total: items.length, page: 1, page_size: 20 },
    },
  } as unknown as UseQueryResult<ApiResponse<PageData<QuotaPoolTransaction>>>

  render(<PoolTransactions query={query} />)

  for (const [type, label] of typeLabels) {
    expect(screen.getByText(label)).toBeInTheDocument()
    expect(screen.queryByText(type)).not.toBeInTheDocument()
  }
})
