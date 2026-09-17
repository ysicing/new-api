import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { formatQuota } from '@/lib/format'

import type { QuotaPool } from '../types'

export function QuotaPoolRechargeRules(props: { pool: QuotaPool }) {
  const { t } = useTranslation()
  const pool = props.pool
  const system = pool.system_auto_recharge
  const title = t('Current pool recharge rules')
  if (pool.pool_type === 'new_user' || !system) {
    return (
      <Card aria-label={title}>
        <CardHeader>
          <CardTitle>{title}</CardTitle>
          <CardDescription>
            {pool.pool_type === 'new_user'
              ? t('New-user pools do not support automatic recharge.')
              : t('No data')}
          </CardDescription>
        </CardHeader>
      </Card>
    )
  }

  // 与后端 resolveAutoRechargePolicy 一致：存量默认池全部使用系统配置。
  const systemPool = pool.pool_type === 'default'
  const inheritAmount = systemPool || pool.auto_recharge_amount < 0
  const inheritWeekly = systemPool || pool.weekly_limit < 0
  const inheritMonthly = systemPool || pool.monthly_limit < 0
  const amount = inheritAmount ? system.amount : pool.auto_recharge_amount
  const weekly = inheritWeekly ? system.weekly_limit : pool.weekly_limit
  const monthly = inheritMonthly ? system.monthly_limit : pool.monthly_limit
  let reason: string | undefined
  if (!system.enabled) reason = t('System disabled')
  else if (!pool.enabled) reason = t('Pool disabled')
  else if (amount <= 0) {
    reason = inheritAmount
      ? t('Amount not configured')
      : t('Pool-level disabled')
  }
  const insufficient = !reason && !systemPool && pool.quota < amount
  let status = reason ? t('Disabled') : t('Enabled')
  if (insufficient) {
    status = t('Temporarily unavailable')
    reason = t('Pool quota is insufficient for one recharge.')
  }
  const rows = [
    {
      label: t('Recharge threshold'),
      value: t('Balance at or below {{amount}}', {
        amount: formatQuota(system.threshold),
      }),
      source: t('System setting'),
    },
    {
      label: t('Recharge amount'),
      value: formatQuota(amount),
      source: inheritAmount
        ? t('Inherit system setting')
        : t('Pool custom setting'),
    },
    {
      label: t('Weekly limit'),
      value: weekly === 0 ? t('Unlimited') : String(weekly),
      source: inheritWeekly
        ? t('Inherit system setting')
        : t('Pool custom setting'),
    },
    {
      label: t('Monthly limit'),
      value: monthly === 0 ? t('Unlimited') : String(monthly),
      source: inheritMonthly
        ? t('Inherit system setting')
        : t('Pool custom setting'),
    },
    {
      label: t('Interval in minutes'),
      value: String(system.interval),
      source: t('System setting'),
    },
  ]
  return (
    <Card aria-label={title}>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>
          {t(
            'Limits apply to each member. Automatic recharge also requires sufficient pool quota.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='flex flex-col gap-4'>
        <div className='flex flex-wrap items-center gap-2'>
          <span className='text-sm'>{t('Automatic recharge')}</span>
          <Badge variant={reason ? 'secondary' : 'default'}>{status}</Badge>
          {reason ? (
            <span className='text-muted-foreground text-sm'>{reason}</span>
          ) : null}
        </div>
        <dl className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
          {rows.map((row) => (
            <div key={row.label}>
              <dt className='text-muted-foreground text-sm'>{row.label}</dt>
              <dd className='mt-1 font-medium tabular-nums'>{row.value}</dd>
              <dd className='text-muted-foreground mt-1 text-xs'>
                {row.source}
              </dd>
            </div>
          ))}
        </dl>
      </CardContent>
    </Card>
  )
}
