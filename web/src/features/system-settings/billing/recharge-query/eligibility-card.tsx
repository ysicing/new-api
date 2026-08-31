/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useQuery } from '@tanstack/react-query'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import { formatQuota } from '@/lib/format'

import { SettingsCard } from '../../components/settings-card'
import { getAutoRechargeEligibility } from './api'
import { rechargeQueryErrorMessage } from './error-message'
import type { AutoRechargeEligibility, AutoRechargeLimitUsage } from './types'

const REASON_LABELS: Record<string, string> = {
  disabled: 'Automatic recharge is disabled globally.',
  quota_above_threshold: 'The user balance is above the recharge threshold.',
  quota_pool_not_found: 'The user quota pool no longer exists.',
  new_user_pool_disabled:
    'The new-user quota pool does not support automatic recharge.',
  quota_pool_disabled: 'The user quota pool is disabled.',
  amount_not_configured: 'No valid recharge amount is configured.',
  weekly_count_failed: 'The weekly recharge count could not be read.',
  weekly_limited: 'The weekly recharge limit has been reached.',
  monthly_count_failed: 'The monthly recharge count could not be read.',
  monthly_limited: 'The monthly recharge limit has been reached.',
  quota_pool_insufficient: 'The quota pool balance is insufficient.',
  user_disabled: 'The user account is disabled.',
}

function formatLimit(usage: AutoRechargeLimitUsage, notLimited: string) {
  if (usage.limit <= 0) return `${usage.used} / ${notLimited}`
  return `${usage.used} / ${usage.limit}`
}

function EligibilityResult(props: { result: AutoRechargeEligibility }) {
  const { t } = useTranslation()
  const result = props.result
  const metrics = [
    [t('Current balance'), formatQuota(result.user_quota)],
    [t('Recharge threshold'), formatQuota(result.threshold)],
    [
      t('Quota pool'),
      result.pool_name ||
        (result.pool_id > 0 ? `#${result.pool_id}` : t('Not applicable')),
    ],
    [
      t('Pool balance'),
      result.pool_quota == null
        ? t('Not applicable')
        : formatQuota(result.pool_quota),
    ],
    [t('Recharge amount'), formatQuota(result.amount)],
    [t('Weekly recharge count'), formatLimit(result.weekly, t('Not limited'))],
    [
      t('Monthly recharge count'),
      formatLimit(result.monthly, t('Not limited')),
    ],
  ]
  const reasonKey = result.reason ?? ''
  const reason = REASON_LABELS[reasonKey]
    ? t(REASON_LABELS[reasonKey])
    : t('Unknown blocking rule: {{reason}}', { reason: reasonKey })

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
        <div>
          <p className='font-medium'>
            {result.username || `#${result.user_id}`}
          </p>
          <p className='text-muted-foreground text-sm'>
            {result.email || `ID ${result.user_id}`}
          </p>
        </div>
        {result.eligible ? (
          <Badge variant='secondary' className='bg-success/10 text-success'>
            {t('Eligible for automatic recharge')}
          </Badge>
        ) : (
          <Badge variant='destructive'>
            {t('Not eligible for automatic recharge')}
          </Badge>
        )}
      </div>
      {!result.eligible ? (
        <Alert variant='destructive'>
          <AlertTitle>{t('Blocking rule')}</AlertTitle>
          <AlertDescription>{reason}</AlertDescription>
        </Alert>
      ) : null}
      <dl className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        {metrics.map(([label, value]) => (
          <div key={label} className='bg-muted/30 rounded-lg border px-3 py-2'>
            <dt className='text-muted-foreground text-xs'>{label}</dt>
            <dd className='mt-1 font-medium tabular-nums'>{value}</dd>
          </div>
        ))}
      </dl>
      <p className='text-muted-foreground text-xs'>
        {t(
          'This result is a read-only snapshot. Actual recharge still uses the latest balance and limits.'
        )}
      </p>
    </div>
  )
}

export function EligibilityCard() {
  const { t } = useTranslation()
  const [identifier, setIdentifier] = useState('')
  const [submittedIdentifier, setSubmittedIdentifier] = useState('')
  const [validationError, setValidationError] = useState('')
  const eligibilityQuery = useQuery({
    queryKey: ['auto-recharge-eligibility', submittedIdentifier],
    queryFn: async () => {
      const response = await getAutoRechargeEligibility(submittedIdentifier)
      if (!response.success || !response.data) {
        throw new Error(
          response.message || t('We could not check recharge eligibility.')
        )
      }
      return response.data
    },
    enabled: submittedIdentifier !== '',
    retry: false,
  })
  let resultContent: ReactNode
  if (eligibilityQuery.isLoading || eligibilityQuery.isFetching) {
    resultContent = (
      <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        {['a', 'b', 'c', 'd'].map((key) => (
          <Skeleton key={key} className='h-16 w-full rounded-lg' />
        ))}
      </div>
    )
  } else if (eligibilityQuery.isError) {
    resultContent = (
      <ErrorState
        title={t('We could not check recharge eligibility.')}
        description={rechargeQueryErrorMessage(
          eligibilityQuery.error,
          t,
          t('Request failed')
        )}
        onRetry={() => void eligibilityQuery.refetch()}
        className='min-h-44'
      />
    )
  } else if (eligibilityQuery.data) {
    resultContent = <EligibilityResult result={eligibilityQuery.data} />
  } else {
    resultContent = (
      <Empty className='min-h-36 border'>
        <EmptyHeader>
          <EmptyTitle>{t('No user selected')}</EmptyTitle>
          <EmptyDescription>
            {t('Enter a user to inspect the current automatic recharge rules.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  return (
    <SettingsCard
      title={t('Automatic recharge eligibility')}
      description={t(
        'Look up one user by exact ID, username, or email. This query never performs a recharge.'
      )}
    >
      <form
        className='mb-4 flex flex-col gap-3 sm:flex-row sm:items-end'
        onSubmit={(event) => {
          event.preventDefault()
          const value = identifier.trim()
          if (!value) {
            setValidationError(t('Enter a user ID, username, or email.'))
            return
          }
          setValidationError('')
          if (value === submittedIdentifier) {
            void eligibilityQuery.refetch()
          } else {
            setSubmittedIdentifier(value)
          }
        }}
      >
        <FieldGroup className='sm:flex-row sm:items-end'>
          <Field className='flex-1' data-invalid={validationError !== ''}>
            <FieldLabel htmlFor='recharge-query-identifier'>
              {t('User ID, username, or email')}
            </FieldLabel>
            <Input
              id='recharge-query-identifier'
              type='search'
              value={identifier}
              aria-invalid={validationError !== ''}
              onChange={(event) => setIdentifier(event.target.value)}
            />
            <FieldDescription>{t('Exact match only')}</FieldDescription>
            <FieldError>{validationError}</FieldError>
          </Field>
          <Button type='submit' disabled={eligibilityQuery.isFetching}>
            {eligibilityQuery.isFetching ? (
              <>
                <Spinner data-icon='inline-start' />
                {t('Checking...')}
              </>
            ) : (
              t('Check eligibility')
            )}
          </Button>
        </FieldGroup>
      </form>

      <div
        aria-live='polite'
        aria-busy={eligibilityQuery.isLoading || eligibilityQuery.isFetching}
      >
        {eligibilityQuery.isFetching ? (
          <span className='sr-only' role='status'>
            {t('Loading...')}
          </span>
        ) : null}
        {resultContent}
      </div>
    </SettingsCard>
  )
}
