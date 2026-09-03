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
import { toast } from 'sonner'

import { DatePicker } from '@/components/date-picker'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ButtonGroup } from '@/components/ui/button-group'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
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

import { exportQuotaPoolStats } from '../api'
import type {
  ApiResponse,
  QuotaPoolMemberStat,
  QuotaPoolStats as QuotaPoolStatsData,
  QuotaPoolStatsRange,
  QuotaPoolUsageStat,
} from '../types'
import { LoadingOrEmpty } from './quota-pool-data'
import { QuotaPoolStatsCharts } from './quota-pool-stats-charts'

type MemberFilter = 'all' | 'active' | 'inactive'
type MemberSort = 'usage' | 'requests' | 'active_days'
const MEMBER_PAGE_SIZE = 50

function localDate(date: Date) {
  return dayjs(date).format('YYYY-MM-DD')
}

function currentWeekStart() {
  const today = dayjs()
  return today.subtract((today.day() + 6) % 7, 'day').format('YYYY-MM-DD')
}

function customRangeFromStart(
  range: Extract<QuotaPoolStatsRange, { range_type: 'custom' }>,
  startDate: string
): QuotaPoolStatsRange {
  const start = dayjs(startDate)
  const currentEnd = dayjs(range.end_date)
  const latestEnd = start.add(365, 'day')
  let end = currentEnd
  if (currentEnd.isBefore(start) || currentEnd.isAfter(latestEnd)) {
    const today = dayjs()
    end = latestEnd.isAfter(today) ? today : latestEnd
  }
  return {
    range_type: 'custom',
    start_date: startDate,
    end_date: end.format('YYYY-MM-DD'),
  }
}

function customRangeFromEnd(
  range: Extract<QuotaPoolStatsRange, { range_type: 'custom' }>,
  endDate: string
): QuotaPoolStatsRange {
  const end = dayjs(endDate)
  const currentStart = dayjs(range.start_date)
  const earliestStart = end.subtract(365, 'day')
  const start =
    currentStart.isAfter(end) || currentStart.isBefore(earliestStart)
      ? earliestStart
      : currentStart
  return {
    range_type: 'custom',
    start_date: start.format('YYYY-MM-DD'),
    end_date: endDate,
  }
}

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
  normalizedStartDate?: string
  normalizedEndDate?: string
  loading: boolean
  exporting: boolean
  onRangeChange: (range: QuotaPoolStatsRange) => void
  onRefresh: () => void
  onExport: (format: 'markdown' | 'xlsx') => void
}) {
  const { t } = useTranslation()
  const today = dayjs().format('YYYY-MM-DD')
  const month = dayjs().format('YYYY-MM')
  const customRange =
    props.range.range_type === 'custom' ? props.range : undefined
  const setRangeType = (rangeType: string) => {
    if (rangeType === 'month') {
      props.onRangeChange({ range_type: 'month' })
      return
    }
    if (rangeType === 'custom') {
      props.onRangeChange({
        range_type: 'custom',
        start_date: props.normalizedStartDate ?? currentWeekStart(),
        end_date: props.normalizedEndDate ?? today,
      })
      return
    }
    props.onRangeChange({ range_type: 'week' })
  }

  return (
    <div className='flex flex-col gap-3 pt-4'>
      <div className='flex flex-wrap items-center gap-2'>
        <ButtonGroup aria-label={t('Quick date ranges')}>
          <Button
            size='sm'
            disabled={props.loading}
            variant={
              props.range.range_type === 'week' && !props.range.anchor
                ? 'secondary'
                : 'outline'
            }
            aria-pressed={
              props.range.range_type === 'week' && !props.range.anchor
            }
            onClick={() => props.onRangeChange({ range_type: 'week' })}
          >
            {t('This week')}
          </Button>
          <Button
            size='sm'
            disabled={props.loading}
            variant={
              props.range.range_type === 'month' && !props.range.anchor
                ? 'secondary'
                : 'outline'
            }
            aria-pressed={
              props.range.range_type === 'month' && !props.range.anchor
            }
            onClick={() => props.onRangeChange({ range_type: 'month' })}
          >
            {t('This month')}
          </Button>
        </ButtonGroup>
        <NativeSelect
          className='w-32'
          disabled={props.loading}
          aria-label={t('Statistics range type')}
          value={props.range.range_type}
          onChange={(event) => setRangeType(event.target.value)}
        >
          <NativeSelectOption value='week'>{t('By week')}</NativeSelectOption>
          <NativeSelectOption value='month'>{t('By month')}</NativeSelectOption>
          <NativeSelectOption value='custom'>
            {t('Custom range')}
          </NativeSelectOption>
        </NativeSelect>
        {props.range.range_type === 'week' ? (
          <DatePicker
            selected={
              props.range.anchor
                ? new Date(`${props.range.anchor}T00:00:00`)
                : undefined
            }
            onSelect={(date) =>
              date &&
              props.onRangeChange({
                range_type: 'week',
                anchor: localDate(date),
              })
            }
            placeholder={t('Select a week')}
          />
        ) : null}
        {props.range.range_type === 'month' ? (
          <Input
            className='w-40'
            type='month'
            disabled={props.loading}
            max={month}
            aria-label={t('Select a month')}
            value={props.range.anchor ?? ''}
            onChange={(event) => {
              if (event.target.value) {
                props.onRangeChange({
                  range_type: 'month',
                  anchor: event.target.value,
                })
              }
            }}
          />
        ) : null}
        {customRange ? (
          <div className='flex flex-wrap items-center gap-2'>
            <span className='text-muted-foreground text-sm'>{t('From')}</span>
            <DatePicker
              selected={new Date(`${customRange.start_date}T00:00:00`)}
              onSelect={(date) =>
                date &&
                props.onRangeChange(
                  customRangeFromStart(customRange, localDate(date))
                )
              }
            />
            <span className='text-muted-foreground text-sm'>{t('To')}</span>
            <DatePicker
              selected={new Date(`${customRange.end_date}T00:00:00`)}
              onSelect={(date) =>
                date &&
                props.onRangeChange(
                  customRangeFromEnd(customRange, localDate(date))
                )
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
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  size='sm'
                  variant='outline'
                  disabled={props.loading || props.exporting}
                />
              }
            >
              <Download data-icon='inline-start' aria-hidden='true' />
              {props.exporting ? t('Exporting...') : t('Export')}
            </DropdownMenuTrigger>
            <DropdownMenuContent align='end'>
              <DropdownMenuItem onClick={() => props.onExport('markdown')}>
                {t('Export Markdown')}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => props.onExport('xlsx')}>
                {t('Export Excel')}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
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
  const [exporting, setExporting] = useState(false)
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
  const handleExport = async (format: 'markdown' | 'xlsx') => {
    setExporting(true)
    try {
      const exported = await exportQuotaPoolStats(
        props.poolId,
        props.selfMode === true,
        props.range,
        format
      )
      const href = URL.createObjectURL(exported.blob)
      const link = document.createElement('a')
      link.href = href
      link.download = exported.filename
      document.body.append(link)
      link.click()
      link.remove()
      URL.revokeObjectURL(href)
      toast.success(t('Statistics exported successfully'))
    } catch {
      toast.error(t('Failed to export statistics'))
    } finally {
      setExporting(false)
    }
  }

  return (
    <div className='flex flex-col gap-3'>
      <PoolStatsControls
        range={props.range}
        normalizedStartDate={stats?.start_date}
        normalizedEndDate={stats?.end_date}
        loading={props.query.isFetching}
        exporting={exporting}
        onRangeChange={props.onRangeChange}
        onRefresh={() => void props.query.refetch()}
        onExport={(format) => void handleExport(format)}
      />
      <LoadingOrEmpty query={props.query} empty={!stats}>
        {stats ? (
          <div className='flex flex-col gap-3'>
            <p className='text-muted-foreground text-xs'>
              {t('Statistics period')}: {stats.start_date ?? '-'} —{' '}
              {stats.end_date ?? '-'} · {t('Generated at')}:{' '}
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
            <QuotaPoolStatsCharts daily={stats.daily ?? []} />
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
    </div>
  )
}
