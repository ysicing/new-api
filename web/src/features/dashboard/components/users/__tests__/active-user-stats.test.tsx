/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'

import { ActiveUserStats } from '../active-user-stats'

const apiMocks = vi.hoisted(() => ({ getActiveUserStats: vi.fn() }))
const chartMocks = vi.hoisted(() => ({ render: vi.fn() }))

vi.mock('../../../api', () => apiMocks)
vi.mock('@/context/theme-provider', () => ({
  useTheme: () => ({ resolvedTheme: 'light' }),
}))
vi.mock('@visactor/react-vchart', () => ({
  VChart: (props: { spec: unknown }) => {
    chartMocks.render(props.spec)
    return <div data-testid='active-user-chart' />
  },
}))

beforeEach(() => {
  apiMocks.getActiveUserStats.mockReset()
  chartMocks.render.mockReset()
  apiMocks.getActiveUserStats.mockResolvedValue({
    success: true,
    generated_at: 1_788_300_000,
    data: {
      total_active_users: 3,
      daily: [
        { date: '2026-09-01', active_users: 2 },
        { date: '2026-09-02', active_users: 3 },
      ],
    },
  })
})

function renderStats() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <ActiveUserStats startTimestamp={100} endTimestamp={200} />
    </QueryClientProvider>
  )
}

test('reuses fresh active-user data across remounts with the same range filter', async () => {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const first = render(
    <QueryClientProvider client={queryClient}>
      <ActiveUserStats startTimestamp={100} endTimestamp={200} rangeKey={7} />
    </QueryClientProvider>
  )
  expect(await screen.findByText('3')).toBeInTheDocument()
  first.unmount()

  render(
    <QueryClientProvider client={queryClient}>
      <ActiveUserStats startTimestamp={160} endTimestamp={260} rangeKey={7} />
    </QueryClientProvider>
  )
  expect(await screen.findByText('3')).toBeInTheDocument()

  expect(apiMocks.getActiveUserStats).toHaveBeenCalledTimes(1)
})

test('shows range total and daily active-user trend from the independent API', async () => {
  renderStats()

  expect(screen.getByText('Active users in range')).toBeInTheDocument()
  expect(await screen.findByText('3')).toBeInTheDocument()
  expect(screen.getByText('Daily active users')).toBeInTheDocument()
  expect(apiMocks.getActiveUserStats).toHaveBeenCalledWith({
    start_timestamp: 100,
    end_timestamp: 200,
  })
  expect(
    screen.getByRole('list', { name: 'Daily active users' })
  ).toHaveTextContent('2026-09-01: 2')
  await waitFor(() => expect(chartMocks.render).toHaveBeenCalled())
  expect(chartMocks.render.mock.calls[0][0]).toEqual(
    expect.objectContaining({
      data: [
        {
          id: 'activeUsers',
          values: [
            { date: '2026-09-01', activeUsers: 2 },
            { date: '2026-09-02', activeUsers: 3 },
          ],
        },
      ],
    })
  )
})

test('announces active-user loading state', () => {
  apiMocks.getActiveUserStats.mockReturnValue(new Promise(() => {}))

  renderStats()

  expect(screen.getByRole('status')).toHaveTextContent('Loading...')
})
