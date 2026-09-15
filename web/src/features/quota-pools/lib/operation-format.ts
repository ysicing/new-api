/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { formatQuota } from '@/lib/format'

export type QuotaPoolOperationDescriptor = {
  action: string
  params: Record<string, unknown>
}

type Translate = (key: string, options?: Record<string, unknown>) => string

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

export function renderQuotaPoolOperationDescriptor(
  descriptor: QuotaPoolOperationDescriptor,
  t: Translate
): string | null {
  const template = operationTemplate(descriptor)
  if (!template) return null
  const params = descriptor.params
  const summary = t(template, {
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
  if (
    !['quota_pool.update', 'quota_pool.self_update'].includes(
      descriptor.action
    ) ||
    !Array.isArray(params.changes)
  ) {
    return summary
  }

  const labels: Record<string, string> = {
    name: t('Name'),
    base_quota: t('Base quota'),
    auto_recharge_amount: t('Recharge amount'),
    weekly_limit: t('Weekly limit'),
    monthly_limit: t('Monthly limit'),
    monthly_refill_enabled: t('Monthly refill'),
    monthly_refill_top_up: t('Top up to target quota'),
    monthly_refill_amount: t('Monthly refill amount'),
    monthly_refill_day: t('Monthly refill day'),
  }
  const details: string[] = []
  for (const change of params.changes) {
    if (
      !change ||
      typeof change !== 'object' ||
      typeof change.field !== 'string'
    ) {
      continue
    }
    if (!Object.hasOwn(labels, change.field)) continue
    const label = labels[change.field]
    details.push(
      `${label}: ${formatQuotaPoolConfigValue(change.field, change.before, t)} → ${formatQuotaPoolConfigValue(change.field, change.after, t)}`
    )
  }
  return [summary, ...details].join('\n')
}

function formatQuotaPoolConfigValue(
  field: string,
  value: unknown,
  t: Translate
): string {
  if (field === 'name') return typeof value === 'string' ? value : '—'
  if (field === 'monthly_refill_enabled' || field === 'monthly_refill_top_up') {
    if (typeof value !== 'boolean') return '—'
    return value ? t('Enabled') : t('Disabled')
  }
  if (typeof value !== 'number' || !Number.isFinite(value)) return '—'
  if (
    ['auto_recharge_amount', 'weekly_limit', 'monthly_limit'].includes(field)
  ) {
    if (value === -1) return t('Inherit system setting')
    if (value === 0) {
      return field === 'auto_recharge_amount' ? t('Disabled') : t('Unlimited')
    }
  }
  if (
    ['base_quota', 'auto_recharge_amount', 'monthly_refill_amount'].includes(
      field
    )
  ) {
    return formatQuota(value)
  }
  return String(value)
}

function operationTemplate(
  descriptor: QuotaPoolOperationDescriptor
): string | null {
  if (descriptor.action === 'quota_pool.enabled') {
    return descriptor.params.enabled === false
      ? 'Disabled quota pool {{pool}}'
      : 'Enabled quota pool {{pool}}'
  }
  return operationTemplates[descriptor.action] ?? null
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
