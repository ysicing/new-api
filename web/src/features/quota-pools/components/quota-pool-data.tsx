import type { UseQueryResult } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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

import type {
  ApiResponse,
  PageData,
  QuotaPool,
  QuotaPoolOperationLog,
  QuotaPoolStats,
  QuotaPoolTransaction,
} from '../types'

type QueryLike = { isLoading: boolean; data?: { data?: unknown } }

export function LoadingOrEmpty(props: {
  query: QueryLike
  empty: boolean
  children: React.ReactNode
}) {
  const { t } = useTranslation()
  if (props.query.isLoading) {
    return <Skeleton className='mt-4 h-40 w-full' />
  }
  if (props.empty) {
    return (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>{t('No data')}</EmptyTitle>
          <EmptyDescription>
            {t('No records are available for this quota pool.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }
  return props.children
}

export function PoolOverview({ pool }: { pool: QuotaPool }) {
  const { t } = useTranslation()
  const cards = [
    [
      t('Available quota'),
      pool.quota < 0 ? t('Unlimited') : formatQuota(pool.quota),
    ],
    [
      t('Base quota'),
      pool.base_quota < 0 ? t('Unlimited') : formatQuota(pool.base_quota),
    ],
    [t('Members'), String(pool.member_count ?? 0)],
  ]
  return (
    <div className='grid gap-3 py-4 sm:grid-cols-3'>
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
  )
}

export function PoolTransactions(props: {
  query: UseQueryResult<ApiResponse<PageData<QuotaPoolTransaction>>>
}) {
  const { t } = useTranslation()
  const items = props.query.data?.data?.items ?? []
  return (
    <LoadingOrEmpty query={props.query} empty={items.length === 0}>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Type')}</TableHead>
            <TableHead>{t('User')}</TableHead>
            <TableHead className='text-right'>{t('Amount')}</TableHead>
            <TableHead>{t('Time')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((item) => (
            <TableRow key={item.id}>
              <TableCell>{t(item.type)}</TableCell>
              <TableCell>{item.user_name || '—'}</TableCell>
              <TableCell className='text-right'>
                {formatQuota(item.amount)}
              </TableCell>
              <TableCell>{formatTimestamp(item.created_at)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </LoadingOrEmpty>
  )
}

export function PoolStats(props: {
  query: UseQueryResult<ApiResponse<QuotaPoolStats>>
}) {
  const { t } = useTranslation()
  const stats = props.query.data?.data
  const cards = [
    ['Total usage', stats?.total_usage],
    ['Total allocated', stats?.total_allocate],
    ['Total refilled', stats?.total_refill],
  ]
  return (
    <LoadingOrEmpty query={props.query} empty={!stats}>
      <div className='grid gap-3 py-4 sm:grid-cols-3'>
        {cards.map(([label, value]) => (
          <Card key={String(label)}>
            <CardHeader>
              <CardTitle className='text-sm'>{t(String(label))}</CardTitle>
            </CardHeader>
            <CardContent className='text-xl font-semibold'>
              {formatQuota(Number(value ?? 0))}
            </CardContent>
          </Card>
        ))}
      </div>
    </LoadingOrEmpty>
  )
}

export function PoolOperationLogs(props: {
  query: UseQueryResult<ApiResponse<PageData<QuotaPoolOperationLog>>>
}) {
  const { t } = useTranslation()
  const items = props.query.data?.data?.items ?? []
  return (
    <LoadingOrEmpty query={props.query} empty={items.length === 0}>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('User')}</TableHead>
            <TableHead>{t('Content')}</TableHead>
            <TableHead>{t('Time')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {items.map((item) => (
            <TableRow key={item.id}>
              <TableCell>{item.username || '—'}</TableCell>
              <TableCell>{item.content}</TableCell>
              <TableCell>{formatTimestamp(item.created_at)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </LoadingOrEmpty>
  )
}
