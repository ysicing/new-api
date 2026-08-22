/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import type { UseQueryResult } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ButtonGroup } from '@/components/ui/button-group'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatQuota } from '@/lib/format'

import type {
  ApiResponse,
  QuotaPoolStats as QuotaPoolStatsData,
  QuotaPoolStatsPeriod,
  QuotaPoolUsageStat,
} from '../types'
import { LoadingOrEmpty } from './quota-pool-data'

function quotaPoolModelShares(stat: QuotaPoolUsageStat) {
  return [
    { label: 'GPT', quota: stat.gpt_quota },
    { label: 'Claude', quota: stat.claude_quota },
    { label: 'DeepSeek', quota: stat.deepseek_quota },
    { label: 'Gemini', quota: stat.gemini_quota },
    { label: 'Qwen', quota: stat.qwen_quota },
    { label: 'Other', quota: stat.other_quota },
  ]
    .map((item) => ({
      label: item.label,
      percent:
        stat.used_quota > 0
          ? Math.round((item.quota / stat.used_quota) * 100)
          : 0,
    }))
    .filter((item) => item.percent > 0)
    .sort((a, b) => b.percent - a.percent || a.label.localeCompare(b.label))
}

function PoolStatsControls(props: {
  period: QuotaPoolStatsPeriod
  loading: boolean
  onPeriodChange: (period: QuotaPoolStatsPeriod) => void
  onRefresh: () => void
}) {
  const { t } = useTranslation()
  return (
    <div className='flex flex-col gap-2 pt-4 sm:flex-row sm:items-center sm:justify-between'>
      <ButtonGroup aria-label={t('Statistics')}>
        <Button
          size='sm'
          variant={props.period === 'week' ? 'secondary' : 'outline'}
          aria-pressed={props.period === 'week'}
          onClick={() => props.onPeriodChange('week')}
        >
          {t('This week')}
        </Button>
        <Button
          size='sm'
          variant={props.period === 'month' ? 'secondary' : 'outline'}
          aria-pressed={props.period === 'month'}
          onClick={() => props.onPeriodChange('month')}
        >
          {t('Past month')}
        </Button>
      </ButtonGroup>
      <Button
        size='sm'
        variant='outline'
        disabled={props.loading}
        onClick={props.onRefresh}
      >
        <RefreshCw data-icon='inline-start' aria-hidden='true' />
        {t('Refresh Stats')}
      </Button>
    </div>
  )
}

function PoolUsageStats({ items }: { items: QuotaPoolUsageStat[] }) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <CardTitle className='text-sm'>{t('Total usage')}</CardTitle>
      </CardHeader>
      <CardContent className='overflow-x-auto'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('User')}</TableHead>
              <TableHead className='text-right'>{t('Total usage')}</TableHead>
              <TableHead>{t('Model usage share')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((item) => (
              <TableRow key={item.user_id}>
                <TableCell>{item.username || `#${item.user_id}`}</TableCell>
                <TableCell className='text-right'>
                  {formatQuota(item.used_quota)}
                </TableCell>
                <TableCell>
                  <div className='flex min-w-48 flex-wrap gap-1'>
                    {quotaPoolModelShares(item).map((share) => (
                      <Badge key={share.label} variant='outline'>
                        {share.label === 'Other' ? t('Other') : share.label}{' '}
                        {share.percent}%
                      </Badge>
                    ))}
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}

export function PoolStats(props: {
  query: UseQueryResult<ApiResponse<QuotaPoolStatsData>>
  period: QuotaPoolStatsPeriod
  onPeriodChange: (period: QuotaPoolStatsPeriod) => void
}) {
  const { t } = useTranslation()
  const stats = props.query.data?.data
  const cards = [
    ['Total usage', stats?.total_usage],
    ['Total allocated', stats?.total_allocate],
    ['Total refilled', stats?.total_refill],
  ]
  return (
    <div className='flex flex-col gap-3'>
      <PoolStatsControls
        period={props.period}
        loading={props.query.isFetching}
        onPeriodChange={props.onPeriodChange}
        onRefresh={() => void props.query.refetch()}
      />
      <LoadingOrEmpty query={props.query} empty={!stats}>
        <div className='flex flex-col gap-3'>
          <div className='grid gap-3 sm:grid-cols-3'>
            {cards.map(([label, value]) => (
              <Card key={String(label)}>
                <CardHeader>
                  <CardTitle className='text-sm'>{t(String(label))}</CardTitle>
                </CardHeader>
                <CardContent className='text-xl font-semibold tabular-nums'>
                  {formatQuota(Number(value ?? 0))}
                </CardContent>
              </Card>
            ))}
          </div>
          {stats?.usage.length ? (
            <PoolUsageStats items={stats.usage} />
          ) : (
            <Empty>
              <EmptyHeader>
                <EmptyTitle>{t('No data')}</EmptyTitle>
                <EmptyDescription>
                  {t('No records are available for this quota pool.')}
                </EmptyDescription>
              </EmptyHeader>
            </Empty>
          )}
        </div>
      </LoadingOrEmpty>
    </div>
  )
}
