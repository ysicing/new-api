/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import type { QuotaPoolOperationLog } from '../types'
import {
  renderQuotaPoolOperationDescriptor,
  type QuotaPoolOperationDescriptor,
} from './operation-format'

type Translate = (key: string, options?: Record<string, unknown>) => string

export function renderQuotaPoolOperation(
  log: QuotaPoolOperationLog,
  t: Translate
): string {
  const descriptor = parseOperationDescriptor(log.other)
  if (!descriptor) return log.content
  return renderQuotaPoolOperationDescriptor(descriptor, t) ?? log.content
}

function parseOperationDescriptor(
  other: string
): QuotaPoolOperationDescriptor | null {
  try {
    const parsed: unknown = JSON.parse(other)
    if (!isRecord(parsed) || !isRecord(parsed.op)) return null
    const action = parsed.op.action
    if (typeof action !== 'string' || action === '') return null
    const params = isRecord(parsed.op.params) ? parsed.op.params : {}
    return { action, params }
  } catch {
    return null
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
