import { useQuery } from '@tanstack/react-query'
import { type ReactNode, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { ButtonGroup } from '@/components/ui/button-group'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
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
import { formatQuota, formatTimestamp } from '@/lib/format'

import {
  getRechargeLeaderboard,
  getTopUsers,
  type OperationsStatsPeriod,
  type OperationsStatsUserStat,
} from '../../api'

const TOP_LIMIT_OPTIONS = [10, 20, 30] as const
const OPERATIONS_STATS_STALE_TIME = 60_000
const OPERATIONS_STATS_PENDING_POLL_INTERVAL = 5_000

export function OperationsStats() {
  const { t } = useTranslation()
  const [period, setPeriod] = useState<OperationsStatsPeriod>('week')
  const [limit, setLimit] = useState<(typeof TOP_LIMIT_OPTIONS)[number]>(10)
  const topUsers = useQuery({
    queryKey: ['operations', 'top-users', period, limit],
    queryFn: () => getTopUsers(limit, period),
    retry: false,
    staleTime: OPERATIONS_STATS_STALE_TIME,
    refetchInterval: (query) =>
      query.state.data?.refreshing
        ? OPERATIONS_STATS_PENDING_POLL_INTERVAL
        : false,
  })
  const recharge = useQuery({
    queryKey: ['operations', 'recharge', limit],
    queryFn: () => getRechargeLeaderboard(limit),
    retry: false,
    staleTime: OPERATIONS_STATS_STALE_TIME,
    refetchInterval: (query) =>
      query.state.data?.refreshing
        ? OPERATIONS_STATS_PENDING_POLL_INTERVAL
        : false,
  })
  const topItems = topUsers.data?.data ?? []
  const rechargeItems = recharge.data?.data.list ?? []
  const topRefreshSchedule =
    topUsers.data?.refresh_schedule === 'daily_after_midnight'
      ? t('Refreshes daily after midnight')
      : t('Refreshes every 30 minutes')
  const rechargeRefreshSchedule = t('Refreshes every 30 minutes')
  const periodControls = (
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
  )
  const limitControls = (
    <ButtonGroup aria-label={t('Top Users')}>
      {TOP_LIMIT_OPTIONS.map((option) => (
        <Button
          key={option}
          size='sm'
          variant={limit === option ? 'secondary' : 'outline'}
          aria-pressed={limit === option}
          onClick={() => setLimit(option)}
        >
          {t('Top {{count}}', { count: option })}
        </Button>
      ))}
    </ButtonGroup>
  )
  return (
    <div className='space-y-3'>
      <div className='flex justify-end'>{limitControls}</div>
      <div className='grid gap-4 xl:grid-cols-2'>
        <StatTable
          title={t('Top users')}
          action={periodControls}
          items={topItems}
          loading={topUsers.isLoading}
          error={topUsers.isError}
          refreshing={topUsers.data?.refreshing ?? false}
          generatedAt={topUsers.data?.generated_at ?? 0}
          refreshSchedule={topRefreshSchedule}
          value={(item) => formatQuota(item.used_quota)}
          valueTitle={t('Usage')}
        />
        <StatTable
          title={t('Recharge leaderboard')}
          description={t('This week')}
          items={rechargeItems}
          loading={recharge.isLoading}
          error={recharge.isError}
          refreshing={recharge.data?.refreshing ?? false}
          generatedAt={recharge.data?.generated_at ?? 0}
          refreshSchedule={rechargeRefreshSchedule}
          value={(item) => String(item.total_count ?? 0)}
          valueTitle={t('Recharges')}
        />
      </div>
    </div>
  )
}

function StatTable(props: {
  title: string
  description?: string
  action?: ReactNode
  items: OperationsStatsUserStat[]
  loading: boolean
  error: boolean
  refreshing: boolean
  generatedAt: number
  refreshSchedule: string
  valueTitle: string
  value: (item: OperationsStatsUserStat) => string
}) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <CardTitle>{props.title}</CardTitle>
        {props.description ? (
          <CardDescription>{props.description}</CardDescription>
        ) : null}
        {props.action ? (
          <CardAction className='col-span-2 row-start-2 mt-2 justify-self-start sm:col-span-1 sm:col-start-2 sm:row-span-2 sm:row-start-1 sm:mt-0 sm:justify-self-end'>
            {props.action}
          </CardAction>
        ) : null}
      </CardHeader>
      <CardContent>
        <StatTableContent {...props} />
      </CardContent>
      <CardFooter className='text-muted-foreground flex-col items-start gap-1 text-xs'>
        {props.generatedAt > 0 ? (
          <span>
            {t('Last updated:')} {formatTimestamp(props.generatedAt)}
          </span>
        ) : null}
        <span>{props.refreshSchedule}</span>
      </CardFooter>
    </Card>
  )
}

function StatTableContent(props: {
  items: OperationsStatsUserStat[]
  loading: boolean
  error: boolean
  refreshing: boolean
  valueTitle: string
  value: (item: OperationsStatsUserStat) => string
}) {
  const { t } = useTranslation()
  if (props.loading) return <Skeleton className='h-60 w-full' />
  if (props.error) {
    return (
      <Empty role='alert'>
        <EmptyHeader>
          <EmptyTitle>{t('Loading failed')}</EmptyTitle>
          <EmptyDescription>{t('Request failed')}</EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }
  if (props.refreshing) {
    return (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>{t('Thinking...')}</EmptyTitle>
        </EmptyHeader>
      </Empty>
    )
  }
  if (props.items.length === 0) {
    return (
      <Empty>
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
          <TableHead>{t('User')}</TableHead>
          <TableHead className='text-right'>{props.valueTitle}</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.items.map((item) => (
          <TableRow key={item.user_id}>
            <TableCell>{item.username}</TableCell>
            <TableCell className='text-right tabular-nums'>
              {props.value(item)}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
