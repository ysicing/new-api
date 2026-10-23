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
import { VChart } from '@visactor/react-vchart'
import { UsersRound } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { IconBadge } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { useTheme } from '@/context/theme-provider'
import { VCHART_OPTION } from '@/lib/vchart'

import { getActiveUserStats } from '../../api'

const ACTIVE_USER_STALE_TIME = 60_000

export function ActiveUserStats(props: {
  startTimestamp: number
  endTimestamp: number
  rangeKey?: string | number
}) {
  const { t } = useTranslation()
  const { resolvedTheme } = useTheme()
  const rangeKey =
    props.rangeKey ?? `${props.startTimestamp}:${props.endTimestamp}`
  const query = useQuery({
    queryKey: ['dashboard', 'active-users', rangeKey],
    queryFn: () =>
      getActiveUserStats({
        start_timestamp: props.startTimestamp,
        end_timestamp: props.endTimestamp,
      }),
    retry: false,
    staleTime: ACTIVE_USER_STALE_TIME,
  })
  const data = query.data?.data
  const values =
    data?.daily.map((item) => ({
      date: item.date,
      activeUsers: item.active_users,
    })) ?? []
  const chartSpec = {
    type: 'area' as const,
    data: [{ id: 'activeUsers', values }],
    xField: 'date',
    yField: 'activeUsers',
    point: { visible: true },
    area: { visible: true },
    axes: [
      { orient: 'bottom' as const, type: 'band' as const },
      {
        orient: 'left' as const,
        type: 'linear' as const,
        min: 0,
        tick: { tickCount: 5 },
      },
    ],
    tooltip: {
      mark: {
        content: [
          {
            key: t('Active users'),
            value: (datum: Record<string, unknown>) =>
              Number(datum.activeUsers) || 0,
          },
        ],
      },
    },
    background: 'transparent',
    theme: resolvedTheme === 'dark' ? 'dark' : 'light',
  }
  let metricContent: ReactNode
  if (query.isLoading) {
    metricContent = <Skeleton className='h-10 w-24' />
  } else if (query.isError) {
    metricContent = (
      <span className='text-destructive text-sm'>{t('Loading failed')}</span>
    )
  } else {
    metricContent = (
      <div className='text-3xl font-semibold tabular-nums'>
        {data?.total_active_users ?? 0}
      </div>
    )
  }
  let chartContent: ReactNode
  if (query.isLoading) {
    chartContent = <Skeleton className='h-full w-full' />
  } else if (query.isError) {
    chartContent = (
      <Empty role='alert' className='h-full'>
        <EmptyHeader>
          <EmptyTitle>{t('Loading failed')}</EmptyTitle>
          <EmptyDescription>{t('Request failed')}</EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  } else if (values.length === 0) {
    chartContent = (
      <Empty className='h-full'>
        <EmptyHeader>
          <EmptyTitle>{t('No data')}</EmptyTitle>
        </EmptyHeader>
      </Empty>
    )
  } else {
    chartContent = <VChart spec={chartSpec} option={VCHART_OPTION} />
  }

  return (
    <div
      className='grid gap-3 lg:grid-cols-[minmax(0,260px)_minmax(0,1fr)]'
      aria-live='polite'
      aria-busy={query.isLoading}
    >
      {query.isLoading ? (
        <span className='sr-only' role='status'>
          {t('Loading...')}
        </span>
      ) : null}
      <Card>
        <CardHeader>
          <div className='flex items-center gap-2'>
            <IconBadge tone='info' size='sm'>
              <UsersRound />
            </IconBadge>
            <CardTitle>{t('Active users in range')}</CardTitle>
          </div>
        </CardHeader>
        <CardContent>{metricContent}</CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('Daily active users')}</CardTitle>
        </CardHeader>
        <CardContent className='h-[260px]'>{chartContent}</CardContent>
        {values.length > 0 ? (
          <ul className='sr-only' aria-label={t('Daily active users')}>
            {values.map((item) => (
              <li key={item.date}>
                {item.date}: {item.activeUsers}
              </li>
            ))}
          </ul>
        ) : null}
      </Card>
    </div>
  )
}
