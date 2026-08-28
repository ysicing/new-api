/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

import { ModelStatistics } from '../model-statistics'

const apiMocks = vi.hoisted(() => ({ getModelStatistics: vi.fn() }))

vi.mock('../../../api', () => apiMocks)

function renderModelStatistics() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <ModelStatistics />
    </QueryClientProvider>
  )
}

describe('model statistics', () => {
  beforeEach(() => {
    apiMocks.getModelStatistics.mockResolvedValue({
      success: true,
      generated_at: 1_700_000_000,
      refresh_schedule: 'every_5_minutes',
      data: [
        { model_name: 'gpt-5', count: 3, quota: 70, share: 0.7 },
        { model_name: 'claude-4', count: 2, quota: 30, share: 0.3 },
      ],
    })
  })

  afterEach(() => {
    useAuthStore.getState().auth.reset()
  })

  test('administrator switches between all users and only mine', async () => {
    useAuthStore.getState().auth.setUser({
      id: 1,
      username: 'admin',
      role: 10,
    })
    renderModelStatistics()

    expect(await screen.findByText('gpt-5')).toBeInTheDocument()
    expect(apiMocks.getModelStatistics).toHaveBeenCalledWith('week', 'all')
    expect(screen.getByText('70.00%')).toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'Calls' })
    ).toBeInTheDocument()
    expect(screen.getByText(/Last updated:/)).toBeInTheDocument()
    expect(screen.getByText('Refreshes every 5 minutes')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Only Mine' }))

    await waitFor(() => {
      expect(apiMocks.getModelStatistics).toHaveBeenLastCalledWith(
        'week',
        'self'
      )
    })
  })

  test('ordinary user is fixed to personal data', async () => {
    useAuthStore.getState().auth.setUser({
      id: 2,
      username: 'member',
      role: 1,
    })
    renderModelStatistics()

    expect(await screen.findByText('gpt-5')).toBeInTheDocument()
    expect(apiMocks.getModelStatistics).toHaveBeenCalledWith('week', 'self')
    expect(
      screen.queryByRole('button', { name: 'All users' })
    ).not.toBeInTheDocument()
  })

  test('reloads personal statistics when the signed-in user changes', async () => {
    useAuthStore.getState().auth.setUser({
      id: 2,
      username: 'member-a',
      role: 1,
    })
    renderModelStatistics()
    expect(await screen.findByText('gpt-5')).toBeInTheDocument()
    expect(apiMocks.getModelStatistics).toHaveBeenCalledTimes(1)

    act(() => {
      useAuthStore.getState().auth.setUser({
        id: 3,
        username: 'member-b',
        role: 1,
      })
    })

    await waitFor(() => {
      expect(apiMocks.getModelStatistics).toHaveBeenCalledTimes(2)
    })
  })

  test('switches between this week and past month', async () => {
    useAuthStore.getState().auth.setUser({
      id: 1,
      username: 'admin',
      role: 10,
    })
    renderModelStatistics()
    expect(await screen.findByText('gpt-5')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Past month' }))

    await waitFor(() => {
      expect(apiMocks.getModelStatistics).toHaveBeenCalledTimes(2)
      expect(apiMocks.getModelStatistics).toHaveBeenLastCalledWith(
        'month',
        'all'
      )
    })
  })
})
