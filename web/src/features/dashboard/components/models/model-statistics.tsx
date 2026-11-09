/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useQuery } from '@tanstack/react-query'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { ButtonGroup } from '@/components/ui/button-group'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatQuota, formatTimestamp } from '@/lib/format'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import {
  getModelStatistics,
  type ModelStatisticItem,
  type ModelStatisticsPeriod,
  type ModelStatisticsScope,
} from '../../api'

const MODEL_STATISTICS_STALE_TIME = 5 * 60_000

export function ModelStatistics() {
  const { t, i18n } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const isAdmin = Boolean(user?.role && user.role >= ROLE.ADMIN)
  const [period, setPeriod] = useState<ModelStatisticsPeriod>('week')
  const [scope, setScope] = useState<ModelStatisticsScope>('all')
  const effectiveScope: ModelStatisticsScope = isAdmin ? scope : 'self'
  const scopeUserId = effectiveScope === 'self' ? (user?.id ?? 0) : 0
  const query = useQuery({
    queryKey: [
      'dashboard',
      'model-statistics',
      period,
      effectiveScope,
      scopeUserId,
    ],
    queryFn: () => getModelStatistics(period, effectiveScope),
    retry: false,
    staleTime: MODEL_STATISTICS_STALE_TIME,
  })
  const rows = query.data?.data ?? []
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const percentFormatter = useMemo(
    () =>
      new Intl.NumberFormat(locale, {
        style: 'percent',
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      }),
    [locale]
  )

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex flex-wrap items-center justify-between gap-2 border-b px-3 py-2 sm:px-5 sm:py-3'>
        <ButtonGroup aria-label={t('Statistics')}>
          <Button
            size='sm'
            variant={period === 'week' ? 'secondary' : 'outline'}
            aria-pressed={period === 'week'}
            onClick={() => setPeriod('week')}
          >
            {t('This week')}
          </Button>
          <Button
            size='sm'
            variant={period === 'month' ? 'secondary' : 'outline'}
            aria-pressed={period === 'month'}
            onClick={() => setPeriod('month')}
          >
            {t('Past month')}
          </Button>
        </ButtonGroup>
        {isAdmin ? (
          <ButtonGroup aria-label={t('Scope')}>
            <Button
              size='sm'
              variant={scope === 'all' ? 'secondary' : 'outline'}
              aria-pressed={scope === 'all'}
              onClick={() => setScope('all')}
            >
              {t('All users')}
            </Button>
            <Button
              size='sm'
              variant={scope === 'self' ? 'secondary' : 'outline'}
              aria-pressed={scope === 'self'}
              onClick={() => setScope('self')}
            >
              {t('Only Mine')}
            </Button>
          </ButtonGroup>
        ) : null}
      </div>
      <ModelStatisticsContent
        loading={query.isLoading}
        error={query.isError}
        rows={rows}
        locale={locale}
        formatShare={(share) => percentFormatter.format(share)}
      />
      <div className='text-muted-foreground flex flex-wrap gap-x-3 gap-y-1 border-t px-3 py-2 text-xs sm:px-5'>
        {query.data?.generated_at ? (
          <span>
            {t('Last updated:')} {formatTimestamp(query.data.generated_at)}
          </span>
        ) : null}
        <span>{t('Refreshes every 5 minutes')}</span>
      </div>
    </div>
  )
}

function ModelStatisticsContent(props: {
  loading: boolean
  error: boolean
  rows: ModelStatisticItem[]
  locale: Intl.LocalesArgument
  formatShare: (share: number) => string
}) {
  const { t } = useTranslation()
  if (props.loading) return <Skeleton className='m-4 h-72 w-auto' />
  if (props.error) {
    return (
      <Empty role='alert' className='min-h-72'>
        <EmptyHeader>
          <EmptyTitle>{t('Loading failed')}</EmptyTitle>
          <EmptyDescription>{t('Request failed')}</EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }
  if (props.rows.length === 0) {
    return (
      <Empty className='min-h-72'>
        <EmptyHeader>
          <EmptyTitle>{t('No data')}</EmptyTitle>
        </EmptyHeader>
      </Empty>
    )
  }
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className='w-16'>#</TableHead>
          <TableHead>{t('Model')}</TableHead>
          <TableHead className='text-right'>{t('Calls')}</TableHead>
          <TableHead className='text-right'>{t('Cost')}</TableHead>
          <TableHead className='text-right'>{t('Share')}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.rows.map((row, index) => (
          <TableRow key={row.model_name}>
            <TableCell className='text-muted-foreground'>{index + 1}</TableCell>
            <TableCell className='font-medium'>{row.model_name}</TableCell>
            <TableCell className='text-right'>
              {formatNumber(row.count, props.locale)}
            </TableCell>
            <TableCell className='text-right'>
              {formatQuota(row.quota)}
            </TableCell>
            <TableCell className='text-right'>
              {props.formatShare(row.share)}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
