/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { QuotaPoolStatsExportDialog } from '../quota-pool-stats-export-dialog'

const apiMocks = vi.hoisted(() => ({ exportQuotaPoolStats: vi.fn() }))
vi.mock('../../api', () => apiMocks)
vi.mock('@/components/datetime-picker', () => ({
  DateTimePicker: (props: { value?: Date; utcFields?: boolean }) => (
    <span>
      {props.value
        ? `${props.utcFields ? props.value.getUTCFullYear() : props.value.getFullYear()}-${String((props.utcFields ? props.value.getUTCMonth() : props.value.getMonth()) + 1).padStart(2, '0')}-${String(props.utcFields ? props.value.getUTCDate() : props.value.getDate()).padStart(2, '0')} ${String(props.utcFields ? props.value.getUTCHours() : props.value.getHours()).padStart(2, '0')}:00`
        : 'empty date'}
    </span>
  ),
}))

describe('quota pool statistics export dialog', () => {
  beforeEach(() => {
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
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(
      () => undefined
    )
  })

  test('inherits the page range and exports with selected granularity and format', async () => {
    render(
      <QuotaPoolStatsExportDialog
        open
        onOpenChange={vi.fn()}
        poolId={7}
        selfMode
        initialRange={{
          start_timestamp: 1787587200,
          end_timestamp: 1788192000,
          granularity: 'day',
        }}
      />
    )

    expect(screen.getByText('2026-08-25 00:00')).toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('Week granularity'))
    fireEvent.click(screen.getByLabelText('Excel format'))
    fireEvent.click(screen.getByRole('button', { name: 'Export file' }))

    await waitFor(() =>
      expect(apiMocks.exportQuotaPoolStats).toHaveBeenCalledWith(
        7,
        true,
        {
          preset: 'custom',
          start_timestamp: 1787587200,
          end_timestamp: 1788192000,
          granularity: 'week',
        },
        'xlsx'
      )
    )
  })

  test('does not export when cancelled', () => {
    const onOpenChange = vi.fn()
    render(
      <QuotaPoolStatsExportDialog
        open
        onOpenChange={onOpenChange}
        poolId={7}
        initialRange={{
          start_timestamp: 1787587200,
          end_timestamp: 1788192000,
          granularity: 'day',
        }}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(apiMocks.exportQuotaPoolStats).not.toHaveBeenCalled()
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  test('does not export a future or normalized-overlimit custom range', () => {
    const now = Math.floor(Date.now() / 1000)
    const view = render(
      <QuotaPoolStatsExportDialog
        open
        onOpenChange={vi.fn()}
        poolId={7}
        initialRange={{
          start_timestamp: now - 60,
          end_timestamp: now + 60,
          granularity: 'hour',
        }}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Export file' }))
    expect(apiMocks.exportQuotaPoolStats).not.toHaveBeenCalled()

    view.unmount()
    render(
      <QuotaPoolStatsExportDialog
        open
        onOpenChange={vi.fn()}
        poolId={7}
        initialRange={{
          start_timestamp: now - 366 * 24 * 60 * 60,
          end_timestamp: now,
          granularity: 'day',
        }}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Export file' }))
    expect(apiMocks.exportQuotaPoolStats).not.toHaveBeenCalled()
  })
})
