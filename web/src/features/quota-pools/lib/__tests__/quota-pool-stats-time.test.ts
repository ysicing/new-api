/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { describe, expect, test } from 'vitest'

import {
  beijingPickerDateToTimestamp,
  isQuotaPoolStatsRangeReady,
  timestampToBeijingPickerDate,
} from '../quota-pool-stats-time'

describe('quota pool statistics Beijing time conversion', () => {
  test('shows an epoch using the business wall clock', () => {
    const pickerDate = timestampToBeijingPickerDate(
      Date.parse('2026-09-03T00:00:00Z') / 1000
    )

    expect([
      pickerDate.getUTCFullYear(),
      pickerDate.getUTCMonth() + 1,
      pickerDate.getUTCDate(),
      pickerDate.getUTCHours(),
    ]).toEqual([2026, 9, 3, 8])
  })

  test('converts the selected wall clock back to the business epoch', () => {
    const pickerDate = new Date(Date.UTC(2026, 8, 3, 0, 0, 0))

    const timestamp = beijingPickerDateToTimestamp(pickerDate)

    expect(new Date(timestamp * 1000).toISOString()).toBe(
      '2026-09-02T16:00:00.000Z'
    )
  })

  test('round trips a business wall clock that falls in the browser DST gap', () => {
    const original = Date.parse('2026-03-07T18:30:00Z') / 1000

    const pickerDate = timestampToBeijingPickerDate(original)
    const roundTrip = beijingPickerDateToTimestamp(pickerDate)

    expect(roundTrip).toBe(original)
  })

  test('rejects invalid custom editing states before querying', () => {
    const now = Date.parse('2026-09-03T00:00:00Z') / 1000

    expect(
      isQuotaPoolStatsRangeReady(
        { preset: 'custom', start_timestamp: now, end_timestamp: now - 1 },
        now
      )
    ).toBe(false)
    expect(
      isQuotaPoolStatsRangeReady(
        { preset: 'custom', start_timestamp: now - 1, end_timestamp: now + 1 },
        now
      )
    ).toBe(false)
    expect(
      isQuotaPoolStatsRangeReady(
        {
          preset: 'custom',
          start_timestamp: now - 366 * 24 * 60 * 60 - 1,
          end_timestamp: now,
        },
        now
      )
    ).toBe(false)
  })
})
