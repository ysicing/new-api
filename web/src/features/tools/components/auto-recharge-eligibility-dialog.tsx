/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Alert02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import { formatQuota } from '@/lib/format'

import { getSelfAutoRechargeEligibility } from '../api'
import type {
  AutoRechargeLimitUsage,
  SelfAutoRechargeEligibility,
} from '../types'

const AUTO_RECHARGE_REASON_KEYS = new Map<string, string>([
  ['disabled', 'Automatic recharge is disabled globally.'],
  [
    'quota_above_threshold',
    'The user balance is above the recharge threshold.',
  ],
  ['quota_pool_not_found', 'The user quota pool no longer exists.'],
  [
    'new_user_pool_disabled',
    'The new-user quota pool does not support automatic recharge.',
  ],
  ['quota_pool_disabled', 'The user quota pool is disabled.'],
  ['amount_not_configured', 'No valid recharge amount is configured.'],
  ['weekly_count_failed', 'The weekly recharge count could not be read.'],
  ['weekly_limited', 'The weekly recharge limit has been reached.'],
  ['monthly_count_failed', 'The monthly recharge count could not be read.'],
  ['monthly_limited', 'The monthly recharge limit has been reached.'],
  ['quota_pool_insufficient', 'The quota pool balance is insufficient.'],
  ['user_disabled', 'The user account is disabled.'],
])

function formatLimit(
  usage: AutoRechargeLimitUsage,
  notLimited: string
): string {
  return usage.limit > 0
    ? `${usage.used} / ${usage.limit}`
    : `${usage.used} / ${notLimited}`
}

function EligibilityResult(props: { result: SelfAutoRechargeEligibility }) {
  const { t } = useTranslation()
  const result = props.result
  let statusLabel = t('Not eligible for automatic recharge')
  let statusVariant: 'secondary' | 'destructive' = 'destructive'
  let statusClassName: string | undefined
  if (result.status === 'eligible') {
    statusLabel = t('Eligible for automatic recharge')
    statusVariant = 'secondary'
    statusClassName = 'bg-success/10 text-success'
  } else if (result.status === 'not_needed') {
    statusLabel = t('Automatic recharge is not needed.')
    statusVariant = 'secondary'
  }
  let guidance: string | null = null
  if (result.guidance === 'quota_pool_admin') {
    guidance = t('Contact your quota pool administrator.')
  } else if (result.guidance === 'department_quota_pool_admin') {
    guidance = t("Contact your department's quota pool administrator.")
  } else if (result.guidance === 'operations_oa') {
    guidance = t('Please submit an operations OA work-order approval request.')
  }
  const reasonKey = result.reason
    ? AUTO_RECHARGE_REASON_KEYS.get(result.reason)
    : undefined
  const reason = reasonKey
    ? t(reasonKey)
    : t(
        'Automatic recharge is currently unavailable. Please follow the guidance below and contact the appropriate person.'
      )
  const metrics = [
    [t('Current balance'), formatQuota(result.user_quota)],
    [t('Recharge threshold'), formatQuota(result.threshold)],
    [t('Recharge amount'), formatQuota(result.amount)],
    [t('Quota pool'), result.pool_name || t('Not applicable')],
    [t('Weekly recharge count'), formatLimit(result.weekly, t('Not limited'))],
    [
      t('Monthly recharge count'),
      formatLimit(result.monthly, t('Not limited')),
    ],
  ]

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <Badge variant={statusVariant} className={statusClassName}>
          {statusLabel}
        </Badge>
      </div>
      {result.status !== 'eligible' ? (
        <Alert
          variant={result.status === 'blocked' ? 'destructive' : 'default'}
        >
          <HugeiconsIcon
            icon={Alert02Icon}
            strokeWidth={2}
            aria-hidden='true'
          />
          <AlertTitle>
            {result.status === 'blocked'
              ? t('Blocking rule')
              : t('Automatic recharge is not needed.')}
          </AlertTitle>
          <AlertDescription className='flex flex-col gap-2'>
            <p>{reason}</p>
            {result.status === 'blocked' && guidance ? <p>{guidance}</p> : null}
          </AlertDescription>
        </Alert>
      ) : null}
      <dl className='grid gap-3 sm:grid-cols-2'>
        {metrics.map(([label, value]) => (
          <div key={label} className='bg-muted/30 rounded-lg border px-3 py-2'>
            <dt className='text-muted-foreground text-xs'>{label}</dt>
            <dd className='mt-1 font-medium tabular-nums'>{value}</dd>
          </div>
        ))}
      </dl>
      <p className='text-muted-foreground text-xs'>
        {t(
          'This is a read-only snapshot. Automatic recharge always checks the latest balance and limits.'
        )}
      </p>
    </div>
  )
}

function EligibilityLoading() {
  const { t } = useTranslation()
  return (
    <div className='flex flex-col gap-4' aria-busy='true'>
      <div className='text-muted-foreground flex items-center gap-2 text-sm'>
        <Spinner />
        <span>{t('Loading...')}</span>
      </div>
      <div className='grid gap-3 sm:grid-cols-2'>
        {['balance', 'threshold', 'amount', 'pool'].map((key) => (
          <Skeleton key={key} className='h-16 w-full' />
        ))}
      </div>
    </div>
  )
}

export function AutoRechargeEligibilityDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const eligibilityQuery = useQuery({
    queryKey: ['self-auto-recharge-eligibility'],
    queryFn: async () => {
      const response = await getSelfAutoRechargeEligibility()
      if (!response.success || !response.data) {
        throw new Error('self auto recharge eligibility request failed')
      }
      return response.data
    },
    enabled: props.open,
    staleTime: 0,
    refetchOnMount: 'always',
    refetchOnWindowFocus: false,
    retry: false,
  })

  let content
  if (eligibilityQuery.isLoading || eligibilityQuery.isFetching) {
    content = <EligibilityLoading />
  } else if (eligibilityQuery.isError) {
    content = (
      <Alert variant='destructive'>
        <HugeiconsIcon icon={Alert02Icon} strokeWidth={2} aria-hidden='true' />
        <AlertTitle>{t('We could not check recharge eligibility.')}</AlertTitle>
        <AlertDescription>
          {t('Please try again in a moment.')}
        </AlertDescription>
      </Alert>
    )
  } else if (eligibilityQuery.data) {
    content = <EligibilityResult result={eligibilityQuery.data} />
  } else {
    content = null
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-lg' showCloseButton={false}>
        <DialogHeader>
          <DialogTitle>{t('Automatic recharge eligibility')}</DialogTitle>
          <DialogDescription>
            {t(
              'Review your current automatic recharge eligibility without making any changes.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div
          aria-live='polite'
          aria-busy={eligibilityQuery.isLoading || eligibilityQuery.isFetching}
        >
          {content}
        </div>
        <DialogFooter>
          {eligibilityQuery.isError ? (
            <Button
              type='button'
              variant='outline'
              onClick={() => void eligibilityQuery.refetch()}
            >
              {t('Retry')}
            </Button>
          ) : null}
          <DialogClose render={<Button variant='outline' />}>
            {t('Close')}
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
