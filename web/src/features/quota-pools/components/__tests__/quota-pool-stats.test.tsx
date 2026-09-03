/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import type { UseQueryResult } from '@tanstack/react-query'
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import type { ApiResponse, QuotaPoolStats } from '../../types'
import { PoolStats } from '../quota-pool-stats'

const apiMocks = vi.hoisted(() => ({ exportQuotaPoolStats: vi.fn() }))
const datePickerMock = vi.hoisted(() => ({ selections: [] as Date[] }))

vi.mock('../../api', () => apiMocks)
vi.mock('@/components/date-picker', () => ({
  DatePicker: (props: { onSelect: (date: Date) => void }) => (
    <button
      type='button'
      onClick={() => {
        const date = datePickerMock.selections.shift()
        if (date) props.onSelect(date)
      }}
    >
      Choose date
    </button>
  ),
}))

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
    data: {
      success: true,
      data: {
        range_type: 'week',
        start_date: '2026-08-17',
        end_date: '2026-08-19',
        start_timestamp: 1,
        end_timestamp: 2,
        generated_at: 3,
        usage: [
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
          },
        ],
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
            last_active_at: 2,
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
          {
            user_id: 9,
            username: 'charlie',
            request_count: 20,
            used_quota: 50,
            gpt_quota: 50,
            claude_quota: 0,
            deepseek_quota: 0,
            gemini_quota: 0,
            qwen_quota: 0,
            other_quota: 0,
            active: true,
            active_days: 3,
            last_active_at: 2,
            usage_share: 33.33,
            average_daily_usage: 16.67,
          },
        ],
        daily: [
          {
            date: '2026-08-17',
            active_members: 1,
            active_rate: 50,
            request_count: 8,
            used_quota: 100,
          },
          {
            date: '2026-08-18',
            active_members: 0,
            active_rate: 0,
            request_count: 0,
            used_quota: 0,
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
  beforeEach(() => {
    datePickerMock.selections = []
    apiMocks.exportQuotaPoolStats.mockReset()
    apiMocks.exportQuotaPoolStats.mockResolvedValue({
      blob: new Blob(['report']),
      filename: 'report.xlsx',
    })
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: vi.fn(() => 'blob:report'),
    })
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: vi.fn(),
    })
  })

  test('shows summary, trends, inactive members and calendar range controls', () => {
    const onRangeChange = vi.fn()

    render(
      <PoolStats
        query={statsQuery()}
        range={{ range_type: 'week', anchor: '2026-08-19' }}
        onRangeChange={onRangeChange}
        poolId={7}
      />
    )

    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.getByText('bob')).toBeInTheDocument()
    expect(screen.getByText('GPT 75%')).toBeInTheDocument()
    expect(screen.getByText('Claude 25%')).toBeInTheDocument()
    expect(screen.getAllByText('Active members')).not.toHaveLength(0)
    expect(screen.getAllByTestId('quota-pool-chart')).toHaveLength(2)
    expect(
      screen.getByRole('table', { name: 'Daily member activity data' })
    ).toHaveTextContent('2026-08-17')
    fireEvent.click(screen.getByRole('button', { name: 'This month' }))
    expect(onRangeChange).toHaveBeenCalledWith(
      expect.objectContaining({ range_type: 'month' })
    )
    fireEvent.change(screen.getByLabelText('Statistics range type'), {
      target: { value: 'custom' },
    })
    expect(onRangeChange).toHaveBeenCalledWith(
      expect.objectContaining({ range_type: 'custom' })
    )
    fireEvent.change(screen.getByLabelText('Activity status'), {
      target: { value: 'inactive' },
    })
    expect(screen.queryByText('alice')).not.toBeInTheDocument()
    expect(screen.getByText('bob')).toBeInTheDocument()
  })

  test('shows an explicit error state when statistics fail to load', () => {
    render(
      <PoolStats
        query={statsQuery({ isError: true, data: undefined })}
        range={{ range_type: 'week', anchor: '2026-08-19' }}
        onRangeChange={vi.fn()}
        poolId={7}
      />
    )

    expect(screen.getByText('Loading failed')).toBeInTheDocument()
  })

  test('downloads both supported export formats for the current range', async () => {
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(() => undefined)
    render(
      <PoolStats
        query={statsQuery()}
        range={{ range_type: 'week', anchor: '2026-08-19' }}
        onRangeChange={vi.fn()}
        poolId={7}
        selfMode
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Export' }))
    fireEvent.click(await screen.findByText('Export Markdown'))
    await waitFor(() =>
      expect(apiMocks.exportQuotaPoolStats).toHaveBeenCalledWith(
        7,
        true,
        { range_type: 'week', anchor: '2026-08-19' },
        'markdown'
      )
    )
    fireEvent.click(screen.getByRole('button', { name: 'Export' }))
    fireEvent.click(await screen.findByText('Export Excel'))
    await waitFor(() =>
      expect(apiMocks.exportQuotaPoolStats).toHaveBeenCalledTimes(2)
    )
    expect(apiMocks.exportQuotaPoolStats).toHaveBeenLastCalledWith(
      7,
      true,
      { range_type: 'week', anchor: '2026-08-19' },
      'xlsx'
    )
    expect(click).toHaveBeenCalledTimes(2)
  })

  test('emits historical week and month anchors', () => {
    const onRangeChange = vi.fn()
    datePickerMock.selections = [new Date('2026-07-09T00:00:00')]
    const view = render(
      <PoolStats
        query={statsQuery()}
        range={{ range_type: 'week', anchor: '2026-08-19' }}
        onRangeChange={onRangeChange}
        poolId={7}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Choose date' }))
    expect(onRangeChange).toHaveBeenCalledWith({
      range_type: 'week',
      anchor: '2026-07-09',
    })

    view.rerender(
      <PoolStats
        query={statsQuery()}
        range={{ range_type: 'month', anchor: '2026-07' }}
        onRangeChange={onRangeChange}
        poolId={7}
      />
    )
    fireEvent.change(screen.getByLabelText('Select a month'), {
      target: { value: '2026-06' },
    })
    expect(onRangeChange).toHaveBeenLastCalledWith({
      range_type: 'month',
      anchor: '2026-06',
    })
  })

  test('clamps a custom range to 366 inclusive days', () => {
    const onRangeChange = vi.fn()
    datePickerMock.selections = [new Date('2025-02-01T00:00:00')]
    render(
      <PoolStats
        query={statsQuery()}
        range={{
          range_type: 'custom',
          start_date: '2025-01-01',
          end_date: '2026-08-01',
        }}
        onRangeChange={onRangeChange}
        poolId={7}
      />
    )

    fireEvent.click(screen.getAllByRole('button', { name: 'Choose date' })[0])

    expect(onRangeChange).toHaveBeenCalledWith({
      range_type: 'custom',
      start_date: '2025-02-01',
      end_date: '2026-02-01',
    })
  })

  test('sorts member rows by request count', () => {
    render(
      <PoolStats
        query={statsQuery()}
        range={{ range_type: 'week' }}
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

    const rows = within(card).getAllByRole('row')
    expect(rows[1]).toHaveTextContent('charlie')
    expect(rows[2]).toHaveTextContent('alice')
  })

  test('paginates large member lists and keeps zero-value trends readable', () => {
    const query = statsQuery()
    const data = query.data?.data
    if (!data) throw new Error('statistics fixture missing')
    data.members = Array.from({ length: 51 }, (_, index) => ({
      ...data.members[1],
      user_id: index + 1,
      username: `member-${index + 1}`,
    }))
    data.daily = [
      {
        date: '2026-08-19',
        active_members: 0,
        active_rate: 0,
        request_count: 0,
        used_quota: 0,
      },
    ]
    render(
      <PoolStats
        query={query}
        range={{ range_type: 'week' }}
        onRangeChange={vi.fn()}
        poolId={7}
      />
    )

    expect(screen.getByText('1 / 2')).toBeInTheDocument()
    expect(screen.getByText('member-1')).toBeInTheDocument()
    expect(screen.queryByText('member-51')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Next' }))
    expect(screen.getByText('member-51')).toBeInTheDocument()
    expect(
      screen.getByRole('table', { name: 'Daily request and usage data' })
    ).toHaveTextContent('2026-08-19')
  })
})
