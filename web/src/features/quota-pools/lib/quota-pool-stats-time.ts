/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
const BEIJING_TIME_ZONE = 'Asia/Shanghai'

function zonedParts(value: Date) {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: BEIJING_TIME_ZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(value)
  const values = Object.fromEntries(
    parts
      .filter((part) => part.type !== 'literal')
      .map((part) => [part.type, Number(part.value)])
  )
  return {
    year: values.year,
    month: values.month,
    day: values.day,
    hour: values.hour,
    minute: values.minute,
    second: values.second,
  }
}

export function timestampToBeijingPickerDate(timestamp: number) {
  const parts = zonedParts(new Date(timestamp * 1000))
  return new Date(
    Date.UTC(
      parts.year,
      parts.month - 1,
      parts.day,
      parts.hour,
      parts.minute,
      parts.second
    )
  )
}

export function beijingPickerDateToTimestamp(value: Date) {
  const desiredWallTime = Date.UTC(
    value.getUTCFullYear(),
    value.getUTCMonth(),
    value.getUTCDate(),
    value.getUTCHours(),
    value.getUTCMinutes(),
    value.getUTCSeconds()
  )
  let candidate = desiredWallTime
  for (let attempt = 0; attempt < 3; attempt++) {
    const parts = zonedParts(new Date(candidate))
    const actualWallTime = Date.UTC(
      parts.year,
      parts.month - 1,
      parts.day,
      parts.hour,
      parts.minute,
      parts.second
    )
    candidate += desiredWallTime - actualWallTime
  }
  return Math.floor(candidate / 1000)
}

export function isQuotaPoolStatsRangeReady(
  range: {
    preset: string
    start_timestamp?: number
    end_timestamp?: number
  },
  nowTimestamp = Math.floor(Date.now() / 1000)
) {
  if (range.preset !== 'custom') return true
  const start = range.start_timestamp
  const end = range.end_timestamp
  if (!start || !end || start > end || end > nowTimestamp) return false
  const normalizedStart = start - (start % 3600)
  const normalizedEnd = Math.min(nowTimestamp, end - (end % 3600) + 3600 - 1)
  return normalizedEnd - normalizedStart <= 366 * 24 * 60 * 60
}
