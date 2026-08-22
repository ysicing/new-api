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
import { formatQuota } from '@/lib/format'

import {
  getRechargeLeaderboard,
  getTopUsers,
  type OperationsStatsPeriod,
} from '../../api'

interface UserStat {
  user_id: number
  username: string
  used_quota: number
  total_count?: number
}

export function OperationsStats() {
  const { t } = useTranslation()
  const [period, setPeriod] = useState<OperationsStatsPeriod>('week')
  const topUsers = useQuery({
    queryKey: ['operations', 'top-users', period],
    queryFn: () => getTopUsers(10, period),
    retry: false,
  })
  const recharge = useQuery({
    queryKey: ['operations', 'recharge'],
    queryFn: () => getRechargeLeaderboard(10),
    retry: false,
  })
  const topItems = (topUsers.data?.data ?? []) as UserStat[]
  const rechargeItems = (recharge.data?.data?.list ?? []) as UserStat[]
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
  return (
    <div className='grid gap-4 xl:grid-cols-2'>
      <StatTable
        title={t('Top users')}
        action={periodControls}
        items={topItems}
        loading={topUsers.isLoading}
        error={topUsers.isError}
        value={(item) => formatQuota(item.used_quota)}
        valueTitle={t('Usage')}
      />
      <StatTable
        title={t('Recharge leaderboard')}
        description={t('This week')}
        items={rechargeItems}
        loading={recharge.isLoading}
        error={recharge.isError}
        value={(item) => String(item.total_count ?? 0)}
        valueTitle={t('Recharges')}
      />
    </div>
  )
}

function StatTable(props: {
  title: string
  description?: string
  action?: ReactNode
  items: UserStat[]
  loading: boolean
  error: boolean
  valueTitle: string
  value: (item: UserStat) => string
}) {
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
    </Card>
  )
}

function StatTableContent(props: {
  items: UserStat[]
  loading: boolean
  error: boolean
  valueTitle: string
  value: (item: UserStat) => string
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
