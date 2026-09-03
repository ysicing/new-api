/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import type { QuotaPoolStatsGranularity, QuotaPoolStatsPreset } from '../types'

export const quotaPoolStatsPresets: Array<{
  value: QuotaPoolStatsPreset
  label: string
}> = [
  { value: 'rolling_1d', label: '1 day' },
  { value: 'rolling_7d', label: '7 days' },
  { value: 'rolling_14d', label: '14 days' },
  { value: 'rolling_29d', label: '29 days' },
  { value: 'today', label: 'Current day' },
  { value: 'this_week', label: 'This week' },
  { value: 'this_month', label: 'This month' },
  { value: 'custom', label: 'Custom range' },
]

export function defaultQuotaPoolStatsGranularity(
  preset: QuotaPoolStatsPreset
): QuotaPoolStatsGranularity {
  return preset === 'rolling_1d' || preset === 'today' ? 'hour' : 'day'
}
