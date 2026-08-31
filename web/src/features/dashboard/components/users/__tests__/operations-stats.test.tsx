/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { OperationsStats } from '../operations-stats'

const apiMocks = vi.hoisted(() => ({
  getTopUsers: vi.fn(),
  getRechargeLeaderboard: vi.fn(),
}))

vi.mock('../../../api', () => apiMocks)

function renderOperationsStats(
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
) {
  return render(
    <QueryClientProvider client={queryClient}>
      <OperationsStats />
    </QueryClientProvider>
  )
}

describe('operations statistics', () => {
  beforeEach(() => {
    apiMocks.getTopUsers.mockResolvedValue({
      success: true,
      data: [{ user_id: 1, username: 'alice', used_quota: 100 }],
      refreshing: false,
      generated_at: 1_700_000_000,
      refresh_schedule: 'every_5_minutes',
    })
    apiMocks.getRechargeLeaderboard.mockResolvedValue({
      success: true,
      data: {
        list: [
          {
            user_id: 2,
            username: 'bob',
            used_quota: 20,
            total_count: 3,
          },
        ],
      },
      refreshing: false,
      generated_at: 1_700_000_100,
      refresh_schedule: 'every_5_minutes',
    })
  })

  test('uses one shared period for both leaderboards', async () => {
    renderOperationsStats()

    expect(await screen.findByText('alice')).toBeInTheDocument()
    expect(apiMocks.getTopUsers).toHaveBeenCalledWith(10, 'week')
    expect(apiMocks.getRechargeLeaderboard).toHaveBeenCalledWith(10, 'week')

    fireEvent.click(screen.getByRole('button', { name: 'Past month' }))

    await waitFor(() => {
      expect(apiMocks.getTopUsers).toHaveBeenCalledWith(10, 'month')
      expect(apiMocks.getRechargeLeaderboard).toHaveBeenCalledWith(10, 'month')
    })
  })

  test('uses one shared Top limit for both leaderboards', async () => {
    renderOperationsStats()

    expect(await screen.findByText('alice')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Top 20' }))

    await waitFor(() => {
      expect(apiMocks.getTopUsers).toHaveBeenCalledWith(20, 'week')
      expect(apiMocks.getRechargeLeaderboard).toHaveBeenCalledWith(20, 'week')
    })
  })

  test('shows no-data state for an empty direct result', async () => {
    apiMocks.getTopUsers.mockResolvedValue({
      success: true,
      data: [],
      refreshing: false,
      generated_at: 1_700_000_000,
      refresh_schedule: 'every_5_minutes',
    })
    apiMocks.getRechargeLeaderboard.mockResolvedValue({
      success: true,
      data: { list: [] },
      refreshing: false,
      generated_at: 1_700_000_000,
      refresh_schedule: 'every_5_minutes',
    })

    renderOperationsStats()

    expect(await screen.findAllByText('No data')).toHaveLength(2)
    expect(screen.queryByText('Thinking...')).not.toBeInTheDocument()
  })

  test('shows snapshot time and refresh schedule', async () => {
    renderOperationsStats()

    expect(await screen.findByText('alice')).toBeInTheDocument()
    expect(screen.getAllByText(/Last updated:/)).toHaveLength(2)
    expect(screen.getAllByText('Refreshes every 5 minutes')).toHaveLength(2)

    apiMocks.getTopUsers.mockResolvedValue({
      success: true,
      data: [{ user_id: 1, username: 'alice', used_quota: 100 }],
      refreshing: false,
      generated_at: 1_700_000_200,
      refresh_schedule: 'every_5_minutes',
    })
    fireEvent.click(screen.getByRole('button', { name: 'Past month' }))

    expect(
      await screen.findAllByText('Refreshes every 5 minutes')
    ).toHaveLength(2)
  })

  test('keeps the recharge leaderboard visible when top users fail', async () => {
    apiMocks.getTopUsers.mockRejectedValue(new Error('failed'))

    renderOperationsStats()

    expect(await screen.findByText('bob')).toBeInTheDocument()
    expect(screen.getByText('Loading failed')).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('Loading failed')
  })

  test('does not retry failed operations requests', async () => {
    apiMocks.getTopUsers.mockRejectedValue(new Error('failed'))
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: 2, retryDelay: 0 } },
    })

    renderOperationsStats(queryClient)

    expect(await screen.findByText('Loading failed')).toBeInTheDocument()
    expect(apiMocks.getTopUsers).toHaveBeenCalledTimes(1)
  })
})
