/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import type { UseQueryResult } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import type { ApiResponse, QuotaPoolStats } from '../../types'
import { PoolStats } from '../quota-pool-stats'

function statsQuery(
  overrides: Partial<UseQueryResult<ApiResponse<QuotaPoolStats>>> = {}
): UseQueryResult<ApiResponse<QuotaPoolStats>> {
  return {
    isLoading: false,
    isError: false,
    data: {
      success: true,
      data: {
        usage: [
          {
            user_id: 7,
            username: 'alice',
            used_quota: 100,
            gpt_quota: 75,
            claude_quota: 25,
            deepseek_quota: 0,
            gemini_quota: 0,
            qwen_quota: 0,
            other_quota: 0,
          },
        ],
        recharge: [],
        total_usage: 100,
        total_refill: 0,
        total_allocate: 0,
      },
    },
    refetch: vi.fn(),
    ...overrides,
  } as unknown as UseQueryResult<ApiResponse<QuotaPoolStats>>
}

describe('quota pool statistics', () => {
  test('shows period controls and per-user model usage share', () => {
    const onPeriodChange = vi.fn()

    render(
      <PoolStats
        query={statsQuery()}
        period='week'
        onPeriodChange={onPeriodChange}
      />
    )

    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.getByText('GPT 75%')).toBeInTheDocument()
    expect(screen.getByText('Claude 25%')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Past month' }))
    expect(onPeriodChange).toHaveBeenCalledWith('month')
  })

  test('shows an explicit error state when statistics fail to load', () => {
    render(
      <PoolStats
        query={statsQuery({ isError: true, data: undefined })}
        period='week'
        onPeriodChange={vi.fn()}
      />
    )

    expect(screen.getByText('Loading failed')).toBeInTheDocument()
  })
})
