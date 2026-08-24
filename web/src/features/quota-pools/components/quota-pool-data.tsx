import type { UseQueryResult } from '@tanstack/react-query'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'

import {
  Card,
  CardContent,
  CardDescription,
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

import type {
  ApiResponse,
  PageData,
  QuotaPool,
  QuotaPoolAdminContact,
  QuotaPoolOperationLog,
  QuotaPoolTransaction,
} from '../types'

type QueryLike = {
  isLoading: boolean
  isError?: boolean
  data?: { data?: unknown }
}

const transactionTypeLabelKeys: Record<string, string> = {
  initial_fund: 'Initial funding',
  manual_refill: 'Temporary refill',
  monthly_refill: 'Monthly automatic refill',
  allocate_auto: 'Automatic allocation',
  allocate_manual: 'Manual allocation',
  reclaim_user: 'Reclaimed user quota',
  adjust_base_quota: 'Base quota adjustment',
}

function formatPoolLevelQuota(
  pool: QuotaPool,
  quota: number,
  t: TFunction
): string {
  if (pool.pool_type === 'new_user') return t('Not applicable')
  if (quota < 0) return t('Unlimited')
  return formatQuota(quota)
}

export function LoadingOrEmpty(props: {
  query: QueryLike
  empty: boolean
  children: React.ReactNode
}) {
  const { t } = useTranslation()
  if (props.query.isLoading) {
    return <Skeleton className='mt-4 h-40 w-full' />
  }
  if (props.query.isError) {
    return (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>{t('Loading failed')}</EmptyTitle>
          <EmptyDescription>{t('Request failed')}</EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
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
    [t('Available quota'), formatPoolLevelQuota(pool, pool.quota, t)],
    [t('Base quota'), formatPoolLevelQuota(pool, pool.base_quota, t)],
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

export function PoolAdminContacts(props: {
  contacts: QuotaPoolAdminContact[]
}) {
  const { t } = useTranslation()
  return (
    <Card className='mt-3'>
      <CardHeader>
        <CardTitle className='text-sm'>{t('Pool administrators')}</CardTitle>
        <CardDescription>
          {t('Contact a pool administrator when quota is insufficient.')}
        </CardDescription>
      </CardHeader>
      <CardContent>
        {props.contacts.length === 0 ? (
          <p className='text-muted-foreground text-sm'>
            {t('No pool administrators')}
          </p>
        ) : (
          <div className='grid gap-2 sm:grid-cols-2'>
            {props.contacts.map((admin) => (
              <div key={admin.id} className='rounded-lg border px-3 py-2'>
                <p className='font-medium'>
                  {admin.display_name || admin.username}
                </p>
                {admin.email ? (
                  <a
                    className='text-muted-foreground hover:text-foreground text-sm underline-offset-4 hover:underline'
                    href={`mailto:${admin.email}`}
                  >
                    {admin.email}
                  </a>
                ) : (
                  <p className='text-muted-foreground text-sm'>
                    {t('Email not set')}
                  </p>
                )}
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
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
              <TableCell>
                {t(transactionTypeLabelKeys[item.type] ?? item.type)}
              </TableCell>
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
