/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import type { SystemTask } from '@/features/system-settings/types'

import { SystemTasksPanel } from '../system-tasks-panel'

const { listSystemTasks } = vi.hoisted(() => ({
  listSystemTasks: vi.fn(),
}))

vi.mock('@/features/system-settings/api', () => ({ listSystemTasks }))

function maintenanceTask(overrides: Partial<SystemTask> = {}): SystemTask {
  return {
    id: 1,
    task_id: 'task-quota-pool',
    type: 'quota_pool_maintenance',
    status: 'succeeded',
    result: {
      monthly_refilled: 0,
      users_recharged: 2,
      users_skipped: 543,
    },
    locked_by: 'node-a',
    created_at: 1_780_000_000,
    updated_at: 1_780_000_600,
    ...overrides,
  }
}

async function renderPanel(tasks: SystemTask[]) {
  listSystemTasks.mockResolvedValue({
    success: true,
    message: '',
    data: tasks,
  })
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <SystemTasksPanel />
    </QueryClientProvider>
  )
  return within(await screen.findByRole('table'))
}

test('labels the quota pool maintenance task instead of showing its raw type alone', async () => {
  const table = await renderPanel([maintenanceTask()])

  expect(table.getByText('Quota pool maintenance')).toBeInTheDocument()
  expect(table.getByText('quota_pool_maintenance')).toBeInTheDocument()
})

test('summarizes a succeeded maintenance result including zero-valued counts', async () => {
  const table = await renderPanel([maintenanceTask()])

  expect(
    table.getByText(
      'Pools refilled: 0 · Users recharged: 2 · Users skipped: 543'
    )
  ).toBeInTheDocument()
})

test('prefers the error text over the result summary when a task failed', async () => {
  const table = await renderPanel([
    maintenanceTask({
      status: 'failed',
      error: 'quota pool insufficient quota',
      result: { users_recharged: 1, users_skipped: 2 },
    }),
  ])

  expect(table.getByText('quota pool insufficient quota')).toBeInTheDocument()
  expect(table.queryByText(/Users recharged/)).not.toBeInTheDocument()
})

test('renders a placeholder when a succeeded task reports no recognized counters', async () => {
  const table = await renderPanel([
    maintenanceTask({ type: 'channel_test', result: {} }),
  ])

  expect(table.queryByText(/Users recharged/)).not.toBeInTheDocument()
  expect(table.getAllByText('-').length).toBeGreaterThan(0)
})
