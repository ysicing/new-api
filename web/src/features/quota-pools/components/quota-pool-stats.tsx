/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import type { UseQueryResult } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight, Download, RefreshCw } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DateTimePicker } from '@/components/datetime-picker'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ButtonGroup } from '@/components/ui/button-group'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import dayjs from '@/lib/dayjs'
import { formatPercent, formatQuota } from '@/lib/format'

import {
  beijingPickerDateToTimestamp,
  timestampToBeijingPickerDate,
} from '../lib/quota-pool-stats-time'
import type {
  ApiResponse,
  QuotaPoolMemberStat,
  QuotaPoolStats as QuotaPoolStatsData,
  QuotaPoolStatsRange,
  QuotaPoolUsageStat,
} from '../types'
import { LoadingOrEmpty } from './quota-pool-data'
import { QuotaPoolStatsCharts } from './quota-pool-stats-charts'
import { QuotaPoolStatsExportDialog } from './quota-pool-stats-export-dialog'
import { quotaPoolStatsPresets } from './quota-pool-stats-options'

type MemberFilter = 'all' | 'active' | 'inactive'
type MemberSort = 'usage' | 'requests' | 'active_days'
const MEMBER_PAGE_SIZE = 50

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
  range: QuotaPoolStatsRange
  actualStart?: number
  actualEnd?: number
  loading: boolean
  onRangeChange: (range: QuotaPoolStatsRange) => void
  onRefresh: () => void
  onExportOpen: () => void
}) {
  const { t } = useTranslation()
  const custom = props.range.preset === 'custom'
  const selectPreset = (preset: QuotaPoolStatsRange['preset']) => {
    if (preset !== 'custom') {
      props.onRangeChange({ preset })
      return
    }
    const end = props.actualEnd ?? Math.floor(Date.now() / 1000)
    props.onRangeChange({
      preset,
      start_timestamp: props.actualStart ?? end - 7 * 24 * 60 * 60,
      end_timestamp: end,
    })
  }

  return (
    <div className='flex flex-col gap-3 pt-4'>
      <div className='flex flex-wrap items-center gap-2'>
        <ButtonGroup className='flex-wrap' aria-label={t('Quick date ranges')}>
          {quotaPoolStatsPresets.map((preset) => (
            <Button
              key={preset.value}
              size='sm'
              disabled={props.loading}
              variant={
                props.range.preset === preset.value ? 'secondary' : 'outline'
              }
              aria-pressed={props.range.preset === preset.value}
              onClick={() => selectPreset(preset.value)}
            >
              {t(preset.label)}
            </Button>
          ))}
        </ButtonGroup>
        {custom ? (
          <div
            className='flex flex-wrap items-center gap-2'
            role='group'
            aria-label={t('Custom range')}
          >
            <span className='text-muted-foreground text-sm'>{t('From')}</span>
            <DateTimePicker
              dateAriaLabel={t('Start date')}
              timeAriaLabel={t('Start time')}
              clearAriaLabel={t('Clear start time')}
              utcFields
              value={
                props.range.start_timestamp
                  ? timestampToBeijingPickerDate(props.range.start_timestamp)
                  : undefined
              }
              onChange={(date) =>
                props.onRangeChange({
                  ...props.range,
                  start_timestamp: date
                    ? beijingPickerDateToTimestamp(date)
                    : undefined,
                })
              }
            />
            <span className='text-muted-foreground text-sm'>{t('To')}</span>
            <DateTimePicker
              dateAriaLabel={t('End date')}
              timeAriaLabel={t('End time')}
              clearAriaLabel={t('Clear end time')}
              utcFields
              value={
                props.range.end_timestamp
                  ? timestampToBeijingPickerDate(props.range.end_timestamp)
                  : undefined
              }
              onChange={(date) =>
                props.onRangeChange({
                  ...props.range,
                  end_timestamp: date
                    ? beijingPickerDateToTimestamp(date)
                    : undefined,
                })
              }
            />
          </div>
        ) : null}
        <div className='ml-auto flex gap-2'>
          <Button
            size='sm'
            variant='outline'
            disabled={props.loading}
            onClick={props.onRefresh}
          >
            <RefreshCw data-icon='inline-start' aria-hidden='true' />
            {t('Refresh Stats')}
          </Button>
          <Button
            size='sm'
            variant='outline'
            disabled={props.loading}
            onClick={props.onExportOpen}
          >
            <Download data-icon='inline-start' aria-hidden='true' />
            {t('Export')}
          </Button>
        </div>
      </div>
    </div>
  )
}

function PoolMemberStats(props: { items: QuotaPoolMemberStat[] }) {
  const { t } = useTranslation()
  const [filter, setFilter] = useState<MemberFilter>('all')
  const [sort, setSort] = useState<MemberSort>('usage')
  const [page, setPage] = useState(1)
  const items = useMemo(() => {
    const filtered = props.items.filter((item) => {
      if (filter === 'active') return item.active
      if (filter === 'inactive') return !item.active
      return true
    })
    return [...filtered].sort((a, b) => {
      if (sort === 'requests') return b.request_count - a.request_count
      if (sort === 'active_days') return b.active_days - a.active_days
      return b.used_quota - a.used_quota
    })
  }, [filter, props.items, sort])
  const pageCount = Math.max(1, Math.ceil(items.length / MEMBER_PAGE_SIZE))
  const currentPage = Math.min(page, pageCount)
  const visibleItems = items.slice(
    (currentPage - 1) * MEMBER_PAGE_SIZE,
    currentPage * MEMBER_PAGE_SIZE
  )

  return (
    <Card>
      <CardHeader className='flex-row items-center justify-between gap-3'>
        <CardTitle>{t('Member usage details')}</CardTitle>
        <div className='flex flex-wrap gap-2'>
          <NativeSelect
            aria-label={t('Activity status')}
            value={filter}
            onChange={(event) => {
              setFilter(event.target.value as MemberFilter)
              setPage(1)
            }}
          >
            <NativeSelectOption value='all'>
              {t('All members')}
            </NativeSelectOption>
            <NativeSelectOption value='active'>
              {t('Active member')}
            </NativeSelectOption>
            <NativeSelectOption value='inactive'>
              {t('Inactive')}
            </NativeSelectOption>
          </NativeSelect>
          <NativeSelect
            aria-label={t('Sort members by')}
            value={sort}
            onChange={(event) => {
              setSort(event.target.value as MemberSort)
              setPage(1)
            }}
          >
            <NativeSelectOption value='usage'>{t('Usage')}</NativeSelectOption>
            <NativeSelectOption value='requests'>
              {t('Requests')}
            </NativeSelectOption>
            <NativeSelectOption value='active_days'>
              {t('Active days')}
            </NativeSelectOption>
          </NativeSelect>
        </div>
      </CardHeader>
      <CardContent className='overflow-x-auto'>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('User')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead className='text-right'>{t('Active days')}</TableHead>
              <TableHead>{t('Last active')}</TableHead>
              <TableHead className='text-right'>{t('Requests')}</TableHead>
              <TableHead className='text-right'>{t('Usage')}</TableHead>
              <TableHead className='text-right'>{t('Usage share')}</TableHead>
              <TableHead className='text-right'>
                {t('Average daily usage')}
              </TableHead>
              <TableHead>{t('Model usage share')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {visibleItems.map((item) => (
              <TableRow key={item.user_id}>
                <TableCell>{item.username || `#${item.user_id}`}</TableCell>
                <TableCell>
                  <Badge variant={item.active ? 'default' : 'outline'}>
                    {item.active ? t('Active member') : t('Inactive')}
                  </Badge>
                </TableCell>
                <TableCell className='text-right'>{item.active_days}</TableCell>
                <TableCell>
                  {item.last_active_time ||
                    (item.last_active_at > 0
                      ? dayjs
                          .unix(item.last_active_at)
                          .format('YYYY-MM-DD HH:mm')
                      : '-')}
                </TableCell>
                <TableCell className='text-right tabular-nums'>
                  {item.request_count.toLocaleString()}
                </TableCell>
                <TableCell className='text-right tabular-nums'>
                  {formatQuota(item.used_quota)}
                </TableCell>
                <TableCell className='text-right tabular-nums'>
                  {formatPercent(item.usage_share)}
                </TableCell>
                <TableCell className='text-right tabular-nums'>
                  {formatQuota(item.average_daily_usage)}
                </TableCell>
                <TableCell>
                  <div className='flex min-w-48 flex-wrap gap-1'>
                    {quotaPoolModelShares(item).map((share) => (
                      <Badge key={share.label} variant='outline'>
                        {share.label === 'Other' ? t('Other') : share.label}{' '}
                        {share.percent}%
                      </Badge>
                    ))}
                    {item.used_quota === 0 ? '-' : null}
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
        {pageCount > 1 ? (
          <div className='flex items-center justify-end gap-2 pt-3'>
            <span className='text-muted-foreground text-sm tabular-nums'>
              {currentPage} / {pageCount}
            </span>
            <Button
              size='icon-sm'
              variant='outline'
              aria-label={t('Previous')}
              disabled={currentPage <= 1}
              onClick={() => setPage((value) => Math.max(1, value - 1))}
            >
              <ChevronLeft aria-hidden='true' />
            </Button>
            <Button
              size='icon-sm'
              variant='outline'
              aria-label={t('Next')}
              disabled={currentPage >= pageCount}
              onClick={() => setPage((value) => Math.min(pageCount, value + 1))}
            >
              <ChevronRight aria-hidden='true' />
            </Button>
          </div>
        ) : null}
      </CardContent>
    </Card>
  )
}

export function PoolStats(props: {
  query: UseQueryResult<ApiResponse<QuotaPoolStatsData>>
  range: QuotaPoolStatsRange
  onRangeChange: (range: QuotaPoolStatsRange) => void
  poolId: number
  selfMode?: boolean
}) {
  const { t } = useTranslation()
  const [exportOpen, setExportOpen] = useState(false)
  const stats = props.query.data?.data
  const summary = stats?.summary
  const cards = [
    [t('Members'), summary?.member_count.toLocaleString() ?? '0'],
    [t('Active members'), summary?.active_members.toLocaleString() ?? '0'],
    [t('Activity rate'), formatPercent(summary?.active_rate ?? 0)],
    [t('Requests'), summary?.request_count.toLocaleString() ?? '0'],
    [t('Total usage'), formatQuota(summary?.total_usage ?? 0)],
    [
      t('Average usage per active member'),
      formatQuota(summary?.average_usage_per_active_member ?? 0),
    ],
  ]
  return (
    <div className='flex flex-col gap-3'>
      <PoolStatsControls
        range={props.range}
        actualStart={stats?.start_timestamp}
        actualEnd={stats?.end_timestamp}
        loading={props.query.isFetching}
        onRangeChange={props.onRangeChange}
        onRefresh={() => void props.query.refetch()}
        onExportOpen={() => setExportOpen(true)}
      />
      <LoadingOrEmpty query={props.query} empty={!stats}>
        {stats ? (
          <div className='flex flex-col gap-3'>
            <p className='text-muted-foreground text-xs'>
              {t('Statistics period')}: {stats.start_time ?? '-'} —{' '}
              {stats.end_time ?? '-'} · {t('Granularity')}:{' '}
              {t(stats.granularity)} · {t('Generated at')}:{' '}
              {stats.generated_time ||
                (stats.generated_at
                  ? dayjs.unix(stats.generated_at).format('YYYY-MM-DD HH:mm')
                  : '-')}{' '}
              {stats.time_zone ? `(${stats.time_zone}) ` : ''}·{' '}
              {t('Cached for about 5 minutes')}
            </p>
            <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-3'>
              {cards.map(([label, value]) => (
                <Card key={label}>
                  <CardHeader>
                    <CardTitle className='text-sm'>{label}</CardTitle>
                  </CardHeader>
                  <CardContent className='text-xl font-semibold tabular-nums'>
                    {value}
                  </CardContent>
                </Card>
              ))}
            </div>
            <QuotaPoolStatsCharts trend={stats.trend ?? []} />
            <PoolMemberStats items={stats.members ?? []} />
            <div className='grid gap-3 sm:grid-cols-3'>
              <Card>
                <CardHeader>
                  <CardTitle className='text-sm'>
                    {t('Total allocated')}
                  </CardTitle>
                </CardHeader>
                <CardContent className='text-xl font-semibold tabular-nums'>
                  {formatQuota(stats.total_allocate ?? 0)}
                </CardContent>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle className='text-sm'>
                    {t('Total refilled')}
                  </CardTitle>
                </CardHeader>
                <CardContent className='text-xl font-semibold tabular-nums'>
                  {formatQuota(stats.total_refill ?? 0)}
                </CardContent>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle className='text-sm'>
                    {t('Total reclaimed')}
                  </CardTitle>
                </CardHeader>
                <CardContent className='text-xl font-semibold tabular-nums'>
                  {formatQuota(stats.total_reclaim ?? 0)}
                </CardContent>
              </Card>
            </div>
          </div>
        ) : null}
      </LoadingOrEmpty>
      {exportOpen && stats ? (
        <QuotaPoolStatsExportDialog
          open={exportOpen}
          onOpenChange={setExportOpen}
          poolId={props.poolId}
          selfMode={props.selfMode}
          initialRange={{
            start_timestamp: stats.start_timestamp,
            end_timestamp: stats.end_timestamp,
            granularity: stats.granularity,
          }}
        />
      ) : null}
    </div>
  )
}
