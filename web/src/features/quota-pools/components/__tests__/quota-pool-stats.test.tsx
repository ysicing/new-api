/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import type { UseQueryResult } from '@tanstack/react-query'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import type { ApiResponse, QuotaPoolStats } from '../../types'
import { PoolStats } from '../quota-pool-stats'

vi.mock('@visactor/react-vchart', () => ({
  VChart: () => <div data-testid='quota-pool-chart' />,
}))
vi.mock('@/context/theme-provider', () => ({
  useTheme: () => ({ resolvedTheme: 'light' }),
}))

function statsQuery(
  overrides: Partial<UseQueryResult<ApiResponse<QuotaPoolStats>>> = {}
): UseQueryResult<ApiResponse<QuotaPoolStats>> {
  return {
    isLoading: false,
    isError: false,
    isFetching: false,
    data: {
      success: true,
      data: {
        preset: 'rolling_7d',
        granularity: 'day',
        start_timestamp: 1787587200,
        end_timestamp: 1788192000,
        start_time: '2026-08-25 00:00:00 +08:00 CST',
        end_time: '2026-09-01 00:00:00 +08:00 CST',
        generated_at: 1788192000,
        generated_time: '2026-09-01 00:00:00 +08:00 CST',
        time_zone: 'Asia/Shanghai',
        usage: [],
        members: [
          {
            user_id: 7,
            username: 'alice',
            request_count: 8,
            used_quota: 100,
            gpt_quota: 75,
            claude_quota: 25,
            deepseek_quota: 0,
            gemini_quota: 0,
            qwen_quota: 0,
            other_quota: 0,
            active: true,
            active_days: 2,
            last_active_at: 1788192000,
            last_active_time: '2026-09-01 00:00:00 +08:00 CST',
            usage_share: 100,
            average_daily_usage: 50,
          },
          {
            user_id: 8,
            username: 'bob',
            request_count: 0,
            used_quota: 0,
            gpt_quota: 0,
            claude_quota: 0,
            deepseek_quota: 0,
            gemini_quota: 0,
            qwen_quota: 0,
            other_quota: 0,
            active: false,
            active_days: 0,
            last_active_at: 0,
            usage_share: 0,
            average_daily_usage: 0,
          },
        ],
        trend: [
          {
            bucket_start: 1787587200,
            bucket_end: 1787673599,
            label: '2026-08-25',
            active_members: 1,
            active_rate: 50,
            request_count: 8,
            used_quota: 100,
          },
        ],
        summary: {
          member_count: 2,
          active_members: 1,
          active_rate: 50,
          request_count: 8,
          total_usage: 100,
          average_usage_per_active_member: 100,
        },
        recharge: [],
        total_usage: 100,
        total_refill: 0,
        total_allocate: 0,
        total_reclaim: 0,
      },
    },
    refetch: vi.fn(),
    ...overrides,
  } as unknown as UseQueryResult<ApiResponse<QuotaPoolStats>>
}

describe('quota pool statistics', () => {
  test('shows all page presets and emits the selected preset', () => {
    const onRangeChange = vi.fn()
    render(
      <PoolStats
        query={statsQuery()}
        range={{ preset: 'rolling_7d' }}
        onRangeChange={onRangeChange}
        poolId={7}
      />
    )

    for (const label of [
      '1 day',
      '7 days',
      '14 days',
      '29 days',
      'Current day',
      'This week',
      'This month',
      'Custom range',
    ]) {
      expect(screen.getByRole('button', { name: label })).toBeInTheDocument()
    }
    expect(screen.getByRole('button', { name: '7 days' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    fireEvent.click(screen.getByRole('button', { name: '1 day' }))
    expect(onRangeChange).toHaveBeenCalledWith({ preset: 'rolling_1d' })
    fireEvent.click(screen.getByRole('button', { name: 'Custom range' }))
    expect(onRangeChange).toHaveBeenLastCalledWith({
      preset: 'custom',
      start_timestamp: 1787587200,
      end_timestamp: 1788192000,
    })
    fireEvent.click(screen.getByRole('button', { name: 'Export' }))
    expect(
      screen.getByRole('dialog', { name: 'Export quota pool statistics' })
    ).toBeInTheDocument()
  })

  test('renders trend metadata, charts and member filtering', () => {
    render(
      <PoolStats
        query={statsQuery()}
        range={{ preset: 'rolling_7d' }}
        onRangeChange={vi.fn()}
        poolId={7}
      />
    )

    expect(screen.getByText(/2026-08-25 00:00:00/)).toBeInTheDocument()
    expect(screen.getByText(/Asia\/Shanghai/)).toBeInTheDocument()
    expect(screen.getAllByTestId('quota-pool-chart')).toHaveLength(2)
    expect(
      screen.getByRole('table', { name: 'Trend member activity data' })
    ).toHaveTextContent('2026-08-25')
    fireEvent.change(screen.getByLabelText('Activity status'), {
      target: { value: 'inactive' },
    })
    expect(screen.queryByText('alice')).not.toBeInTheDocument()
    expect(screen.getByText('bob')).toBeInTheDocument()
  })

  test('sorts member rows by request count', () => {
    const query = statsQuery()
    const data = query.data?.data
    if (!data) throw new Error('statistics fixture missing')
    data.members.push({
      ...data.members[0],
      user_id: 9,
      username: 'charlie',
      request_count: 20,
      used_quota: 50,
    })
    render(
      <PoolStats
        query={query}
        range={{ preset: 'rolling_7d' }}
        onRangeChange={vi.fn()}
        poolId={7}
      />
    )
    const card = screen
      .getByText('Member usage details')
      .closest<HTMLElement>('[data-slot="card"]')
    if (!card) throw new Error('member details card not found')

    fireEvent.change(within(card).getByLabelText('Sort members by'), {
      target: { value: 'requests' },
    })

    expect(within(card).getAllByRole('row')[1]).toHaveTextContent('charlie')
  })

  test('shows an explicit error state when statistics fail to load', () => {
    render(
      <PoolStats
        query={statsQuery({ isError: true, data: undefined })}
        range={{ preset: 'rolling_7d' }}
        onRangeChange={vi.fn()}
        poolId={7}
      />
    )

    expect(screen.getByText('Loading failed')).toBeInTheDocument()
  })

  test('paginates large member lists', () => {
    const query = statsQuery()
    const data = query.data?.data
    if (!data) throw new Error('statistics fixture missing')
    data.members = Array.from({ length: 51 }, (_, index) => ({
      ...data.members[1],
      user_id: index + 1,
      username: `member-${index + 1}`,
    }))
    render(
      <PoolStats
        query={query}
        range={{ preset: 'rolling_7d' }}
        onRangeChange={vi.fn()}
        poolId={7}
      />
    )

    expect(screen.getByText('1 / 2')).toBeInTheDocument()
    expect(screen.queryByText('member-51')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(screen.getByText('member-51')).toBeInTheDocument()
  })
})
