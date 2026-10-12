/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
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
      users_checked: 545,
      users_recharged: 2,
      users_skipped: 543,
      skip_reasons: {
        quota_above_threshold: 520,
        weekly_limited: 20,
        quota_pool_insufficient: 3,
      },
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

test('labels the quota pool recharge maintenance task instead of showing its raw type alone', async () => {
  const table = await renderPanel([maintenanceTask()])

  expect(table.getByText('Quota pool recharge maintenance')).toBeInTheDocument()
  expect(table.getByText('quota_pool_maintenance')).toBeInTheDocument()
})

test('summarizes a succeeded maintenance result including zero-valued counts', async () => {
  const table = await renderPanel([maintenanceTask()])

  const summary = table.getByText(
    'Pools refilled: 0 · Users checked: 545 · Users recharged: 2 · Users not eligible: 543'
  )
  expect(summary).toBeInTheDocument()
  expect(
    table.getByText(
      'Balance above threshold: 520 · Weekly limit reached: 20 · Quota pool insufficient: 3'
    )
  ).toBeInTheDocument()
  expect(summary.closest('td')).toHaveClass('min-w-[480px]')
  expect(summary.closest('td')).not.toHaveClass('truncate')

  expect(table.getByRole('columnheader', { name: 'Executor' })).toHaveClass(
    'w-[220px]'
  )
  expect(table.getByRole('columnheader', { name: 'Detail' })).toHaveClass(
    'min-w-[480px]'
  )
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

test('shows Magpie counters and expandable channel outcomes including notification errors', async () => {
  const table = await renderPanel([
    maintenanceTask({
      type: 'magpie_channel_monitor',
      result: {
        checked: 3,
        changed: 2,
        enabled: 1,
        disabled: 1,
        unchanged: 0,
        failed: 1,
        notification_failed: 1,
        channels: [
          {
            channel_id: 1,
            channel_name: 'Recovered pool',
            available: true,
            action: 'enabled',
            notification_error: 'send failed',
          },
          {
            channel_id: 2,
            channel_name: 'Exhausted pool',
            available: false,
            action: 'disabled',
          },
          {
            channel_id: 3,
            channel_name: 'Unknown pool',
            action: 'query_failed',
            error: 'availability is unknown',
          },
        ],
      },
    }),
  ])

  expect(table.getByText('Magpie channel monitor')).toBeInTheDocument()
  expect(
    table.getByText(
      'Channels checked: 3 · Channels enabled: 1 · Channels disabled: 1 · Channels unchanged: 0 · Channel checks failed: 1 · Notifications failed: 1'
    )
  ).toBeInTheDocument()
  expect(table.queryByText('Recovered pool (#1)')).not.toBeInTheDocument()
  await userEvent
    .setup()
    .click(table.getByRole('button', { name: 'Channel results' }))
  expect(table.getByText('Recovered pool (#1)')).toBeInTheDocument()
  expect(
    table.getByText('Automatically enabled · Codex accounts available')
  ).toBeInTheDocument()
  expect(
    table.getByText('Automatically disabled · No available Codex accounts')
  ).toBeInTheDocument()
  expect(table.getByText('Availability query failed')).toBeInTheDocument()
  expect(table.getByText('availability is unknown')).toBeInTheDocument()
  expect(
    table.getByText('Notification failed: send failed')
  ).toBeInTheDocument()
})

test('shows zero changes for legacy Magpie history without inventing channel details', async () => {
  const table = await renderPanel([
    maintenanceTask({ type: 'magpie_channel_monitor', result: { changed: 0 } }),
  ])

  expect(table.getByText('Channels changed: 0')).toBeInTheDocument()
  expect(
    table.queryByRole('button', { name: 'Channel results' })
  ).not.toBeInTheDocument()
})

test('retains partial Magpie results when a task fails', async () => {
  const table = await renderPanel([
    maintenanceTask({
      type: 'magpie_channel_monitor',
      status: 'failed',
      error: 'context canceled',
      result: {
        checked: 1,
        enabled: 0,
        disabled: 0,
        unchanged: 1,
        failed: 0,
        notification_failed: 0,
        channels: [
          {
            channel_id: 1,
            channel_name: 'Ready pool',
            available: true,
            action: 'unchanged',
          },
        ],
      },
    }),
  ])

  expect(table.getByText('context canceled')).toBeInTheDocument()
  expect(table.getByText(/Channels checked: 1/)).toBeInTheDocument()
  await userEvent
    .setup()
    .click(table.getByRole('button', { name: 'Channel results' }))
  expect(
    table.getByText('Channel status preserved · Codex accounts available')
  ).toBeInTheDocument()
})
