/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { AxiosError, type AxiosResponse } from 'axios'
import i18next from 'i18next'
import { toast } from 'sonner'
import { beforeAll, expect, test, vi } from 'vitest'

import type { SystemTask } from '@/features/system-settings/types'

import { SystemTasksPanel } from '../system-tasks-panel'
import { SystemTasksTable } from '../system-tasks-table'

const { listSystemTasks, startMagpieMonitorTask } = vi.hoisted(() => ({
  listSystemTasks: vi.fn(),
  startMagpieMonitorTask: vi.fn(),
}))

vi.mock('@/features/system-settings/api', () => ({
  listSystemTasks,
  startMagpieMonitorTask,
}))

// 与生产环境一致，由 React 转义插值；否则日期中的 / 会显示为 HTML 实体。
beforeAll(async () => {
  await i18next.init({
    lng: 'en',
    fallbackLng: 'en',
    nsSeparator: false,
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

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
  listSystemTasks.mockImplementation(async (_limit, filters) => ({
    success: true,
    message: '',
    data: tasks.filter((task) =>
      filters?.scope === 'active'
        ? task.status === 'running' || task.status === 'pending'
        : task.status !== 'running' && task.status !== 'pending'
    ),
    total: tasks.length,
  }))
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <SystemTasksPanel />
    </QueryClientProvider>
  )
  await screen.findByText('Active Tasks')
}

async function renderTable(tasks: SystemTask[]) {
  render(<SystemTasksTable tasks={tasks} />)
  return within(await screen.findByRole('table'))
}

test('labels the quota pool recharge maintenance task instead of showing its raw type alone', async () => {
  const table = await renderTable([maintenanceTask()])

  expect(table.getByText('Quota pool recharge maintenance')).toBeInTheDocument()
  expect(table.getByText('quota_pool_maintenance')).toBeInTheDocument()
})

test('summarizes a succeeded maintenance result including zero-valued counts', async () => {
  const table = await renderTable([maintenanceTask()])

  const summary = table.getByText(
    'Pools refilled: 0 · Users checked: 545 · Users recharged: 2 · Users not eligible: 543'
  )
  expect(summary).toBeInTheDocument()
  expect(
    table.getByText(
      'Balance above threshold: 520 · Weekly limit reached: 20 · Quota pool insufficient: 3'
    )
  ).toBeInTheDocument()
  expect(summary.closest('td')).not.toHaveClass('truncate')
})

test('shows configuration, wallet limit and disabled account skip counts separately', async () => {
  const table = await renderTable([
    maintenanceTask({
      result: {
        users_checked: 6,
        users_recharged: 0,
        users_skipped: 6,
        skip_reasons: {
          invalid_configuration: 1,
          wallet_quota_limit: 2,
          user_disabled: 3,
        },
      },
    }),
  ])

  expect(
    table.getByText(
      'Invalid automatic recharge configuration (1) · Wallet quota limit exceeded (2) · The user account is disabled. (3)'
    )
  ).toBeInTheDocument()
})

test('prefers the error text over the result summary when a task failed', async () => {
  const table = await renderTable([
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
  const table = await renderTable([
    maintenanceTask({ type: 'channel_test', result: {} }),
  ])

  expect(table.queryByText(/Users recharged/)).not.toBeInTheDocument()
  expect(table.getAllByText('-').length).toBeGreaterThan(0)
})

test('shows Magpie counters and expandable channel outcomes including notification errors', async () => {
  const table = await renderTable([
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
  const table = await renderTable([
    maintenanceTask({ type: 'magpie_channel_monitor', result: { changed: 0 } }),
  ])

  expect(table.getByText('Channels changed: 0')).toBeInTheDocument()
  expect(
    table.queryByRole('button', { name: 'Channel results' })
  ).not.toBeInTheDocument()
})

test('translates structured Magpie errors and every diagnostic before legacy fallbacks', async () => {
  i18next.addResourceBundle('zh', 'translation', {
    'Magpie Codex account availability is unknown; channel status preserved':
      'Magpie Codex 账号可用性未知；保留渠道状态',
    'Codex accounts missing from the refresh response: {{count}}':
      '刷新响应缺少 Codex 账号：{{count}}',
    'No enabled Magpie gateway keys: {{count}}':
      '没有已启用的 Magpie 网关键：{{count}}',
    'Failed to send Magpie channel notification: {{error}}':
      '发送 Magpie 渠道通知失败：{{error}}',
  })
  const table = await renderTable([
    maintenanceTask({
      type: 'magpie_channel_monitor',
      result: {
        checked: 1,
        channels: [
          {
            channel_id: 1,
            channel_name: 'Unknown pool',
            action: 'query_failed',
            error: 'legacy availability error',
            error_message: {
              key: 'Magpie Codex account availability is unknown; channel status preserved',
            },
            diagnostics: [
              {
                key: 'Codex accounts missing from the refresh response: {{count}}',
                params: { count: 2 },
              },
              {
                key: 'No enabled Magpie gateway keys: {{count}}',
                params: { count: 3 },
              },
              null,
              { key: 123 },
            ],
            notification_error: 'legacy notification error',
            notification_error_message: {
              key: 'Failed to send Magpie channel notification: {{error}}',
              params: { error: 'connection refused' },
            },
          },
        ],
      },
    }),
  ])
  await userEvent
    .setup()
    .click(table.getByRole('button', { name: 'Channel results' }))

  expect(
    table.getByText(
      'Magpie Codex account availability is unknown; channel status preserved'
    )
  ).toBeInTheDocument()
  expect(
    table.getByText('Codex accounts missing from the refresh response: 2')
  ).toBeInTheDocument()
  expect(
    table.getByText('No enabled Magpie gateway keys: 3')
  ).toBeInTheDocument()
  expect(
    table.getByText(
      'Notification failed: Failed to send Magpie channel notification: connection refused'
    )
  ).toBeInTheDocument()
  expect(table.queryByText('legacy availability error')).not.toBeInTheDocument()
  expect(table.queryByText(/legacy notification error/)).not.toBeInTheDocument()

  try {
    await act(async () => {
      await i18next.changeLanguage('zh')
    })
    expect(
      await table.findByText('Magpie Codex 账号可用性未知；保留渠道状态')
    ).toBeInTheDocument()
    expect(table.getByText('刷新响应缺少 Codex 账号：2')).toBeInTheDocument()
    expect(table.getByText('没有已启用的 Magpie 网关键：3')).toBeInTheDocument()
    expect(
      table.getByText(
        'Notification failed: 发送 Magpie 渠道通知失败：connection refused'
      )
    ).toBeInTheDocument()
  } finally {
    await act(async () => {
      await i18next.changeLanguage('en')
    })
  }
})

test('shows deferred probes and the next check time without claiming fresh availability', async () => {
  const nextCheck = '2026-10-12T14:27:12Z'
  const table = await renderTable([
    maintenanceTask({
      type: 'magpie_channel_monitor',
      result: {
        checked: 0,
        skipped: 1,
        channels: [
          {
            channel_id: 59,
            channel_name: 'Cached pool',
            action: 'cached',
            next_check_at: nextCheck,
          },
          {
            channel_id: 60,
            channel_name: 'Old pool',
            action: 'unchanged',
            next_check_at: 'invalid-date',
          },
        ],
      },
    }),
  ])

  expect(
    table.getByText('Channels checked: 0 · Channel probes deferred: 1')
  ).toBeInTheDocument()
  await userEvent
    .setup()
    .click(table.getByRole('button', { name: 'Channel results' }))
  expect(
    table.getByText('Waiting for the next availability check')
  ).toBeInTheDocument()
  expect(
    table.getByText(
      `Next availability check no earlier than: ${new Date(nextCheck).toLocaleString('en-US')}`
    )
  ).toBeInTheDocument()
  expect(table.queryByText(/Codex accounts available/)).not.toBeInTheDocument()
  expect(table.queryByText(/Invalid Date/)).not.toBeInTheDocument()
})

test('retains partial Magpie results when a task fails', async () => {
  const table = await renderTable([
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

test('manual Magpie check waits for enqueueing and then refreshes task history', async () => {
  let finish!: (value: { success: boolean; message: string }) => void
  startMagpieMonitorTask.mockReturnValue(
    new Promise((resolve) => {
      finish = resolve
    })
  )
  await renderPanel([maintenanceTask()])
  const initialCalls = listSystemTasks.mock.calls.length
  const user = userEvent.setup()
  await user.click(screen.getByRole('button', { name: 'Check Magpie now' }))
  expect(startMagpieMonitorTask).toHaveBeenCalledTimes(1)
  expect(screen.getByRole('button', { name: 'Submitting...' })).toBeDisabled()
  finish({ success: true, message: '' })
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Check Magpie now' })
    ).toBeEnabled()
  )
  await waitFor(() =>
    expect(listSystemTasks.mock.calls.length).toBeGreaterThan(initialCalls)
  )
  expect(listSystemTasks).toHaveBeenCalledWith(100, { scope: 'active' })
  expect(listSystemTasks).toHaveBeenCalledWith(
    20,
    expect.objectContaining({ scope: 'history' })
  )
})

test('manual Magpie check shows enqueue failure and permits retry', async () => {
  const error = vi.spyOn(toast, 'error')
  startMagpieMonitorTask.mockResolvedValue({
    success: false,
    message: 'Task already active',
  })
  await renderPanel([maintenanceTask()])
  await userEvent
    .setup()
    .click(screen.getByRole('button', { name: 'Check Magpie now' }))
  await waitFor(() => expect(error).toHaveBeenCalledWith('Task already active'))
  expect(screen.getByRole('button', { name: 'Check Magpie now' })).toBeEnabled()
})

test('manual Magpie check is disabled while another Magpie task is active', async () => {
  await renderPanel([
    maintenanceTask({ type: 'magpie_channel_monitor', status: 'running' }),
  ])
  expect(
    screen.getByRole('button', { name: 'Check Magpie now' })
  ).toBeDisabled()
  expect(startMagpieMonitorTask).not.toHaveBeenCalled()
})

test('manual Magpie check displays the server reason when enqueueing conflicts', async () => {
  const error = vi.spyOn(toast, 'error')
  const failure = new AxiosError('Request failed with status code 409')
  failure.response = {
    status: 409,
    data: { message: 'Magpie task already active' },
  } as AxiosResponse
  startMagpieMonitorTask.mockRejectedValue(failure)
  await renderPanel([maintenanceTask()])
  await userEvent
    .setup()
    .click(screen.getByRole('button', { name: 'Check Magpie now' }))
  await waitFor(() =>
    expect(error).toHaveBeenCalledWith('Magpie task already active')
  )
  expect(screen.getByRole('button', { name: 'Check Magpie now' })).toBeEnabled()
})
