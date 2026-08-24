/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { formatQuota } from '@/lib/format'

import type { QuotaPoolOperationLog } from '../types'

type Translate = (key: string, options?: Record<string, unknown>) => string

type OperationDescriptor = {
  action: string
  params: Record<string, unknown>
}

const operationTemplates: Record<string, string> = {
  'quota_pool.create':
    'Created quota pool {{pool}} with initial quota {{amount}}',
  'quota_pool.sync_system': 'Synchronized system quota pools',
  'quota_pool.update': 'Updated {{fields}} settings for quota pool {{pool}}',
  'quota_pool.delete': 'Deleted quota pool {{pool}}',
  'quota_pool.refill': 'Added {{amount}} temporary quota to {{pool}}',
  'quota_pool.self_update':
    'Updated {{fields}} auto-recharge settings for {{pool}}',
  'quota_pool.member_add': 'Added member {{user}} to {{pool}}',
  'quota_pool.member_move': 'Moved member {{user}} into {{pool}}',
  'quota_pool.member_remove':
    'Removed member {{user}} to {{targetPool}} and reclaimed {{amount}}',
  'quota_pool.member_recharge': 'Recharged member {{user}} by {{amount}}',
  'quota_pool.member_reclaim': 'Reclaimed {{amount}} from member {{user}}',
  'quota_pool.admin_grant': 'Set member {{user}} as a pool administrator',
  'quota_pool.admin_revoke':
    "Removed member {{user}}'s pool administrator role",
}

export function renderQuotaPoolOperation(
  log: QuotaPoolOperationLog,
  t: Translate
): string {
  const descriptor = parseOperationDescriptor(log.other)
  if (!descriptor) return log.content
  const template = operationTemplate(descriptor)
  if (!template) return log.content
  const params = descriptor.params
  return t(template, {
    user: identifierLabel(params.user_name, params.user_id, true),
    pool: identifierLabel(params.quota_pool_name, params.quota_pool_id, false),
    targetPool: identifierLabel(
      params.target_pool_name,
      params.target_pool_id,
      false
    ),
    amount: quotaLabel(params.amount),
    fields: numberLabel(params.fields),
  })
}

function operationTemplate(descriptor: OperationDescriptor): string | null {
  if (descriptor.action === 'quota_pool.enabled') {
    return descriptor.params.enabled === false
      ? 'Disabled quota pool {{pool}}'
      : 'Enabled quota pool {{pool}}'
  }
  return operationTemplates[descriptor.action] ?? null
}

function parseOperationDescriptor(other: string): OperationDescriptor | null {
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

function identifierLabel(
  name: unknown,
  id: unknown,
  includeId: boolean
): string {
  const cleanName = typeof name === 'string' ? name.trim() : ''
  const numericId = Number(id)
  const hasId = Number.isInteger(numericId) && numericId > 0
  if (cleanName && includeId && hasId) return `${cleanName} (ID: ${numericId})`
  if (cleanName) return cleanName
  return hasId ? `#${numericId}` : '—'
}

function quotaLabel(value: unknown): string {
  const amount = Number(value)
  return Number.isFinite(amount) ? formatQuota(amount) : '—'
}

function numberLabel(value: unknown): string {
  const number = Number(value)
  return Number.isFinite(number) ? String(number) : '—'
}
