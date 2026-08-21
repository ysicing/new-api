import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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

import { getRechargeLeaderboard, getTopUsers } from '../../api'

interface UserStat {
  user_id: number
  username: string
  used_quota: number
  total_count?: number
}

export function OperationsStats() {
  const { t } = useTranslation()
  const topUsers = useQuery({
    queryKey: ['operations', 'top-users'],
    queryFn: () => getTopUsers(10),
  })
  const recharge = useQuery({
    queryKey: ['operations', 'recharge'],
    queryFn: () => getRechargeLeaderboard(10),
  })
  if (topUsers.isLoading || recharge.isLoading) {
    return <Skeleton className='h-80 w-full' />
  }
  const topItems = (topUsers.data?.data ?? []) as UserStat[]
  const rechargeItems = (recharge.data?.data?.list ?? []) as UserStat[]
  return (
    <div className='grid gap-4 xl:grid-cols-2'>
      <StatTable
        title={t('Top users')}
        items={topItems}
        value={(item) => formatQuota(item.used_quota)}
        valueTitle={t('Usage')}
      />
      <StatTable
        title={t('Recharge leaderboard')}
        items={rechargeItems}
        value={(item) => String(item.total_count ?? 0)}
        valueTitle={t('Recharges')}
      />
    </div>
  )
}

function StatTable(props: {
  title: string
  items: UserStat[]
  valueTitle: string
  value: (item: UserStat) => string
}) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <CardTitle>{props.title}</CardTitle>
      </CardHeader>
      <CardContent>
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
      </CardContent>
    </Card>
  )
}
