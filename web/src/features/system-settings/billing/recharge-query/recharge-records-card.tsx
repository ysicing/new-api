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
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { Badge } from '@/components/ui/badge'
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
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { formatQuota, formatTimestamp } from '@/lib/format'

import { SettingsCard } from '../../components/settings-card'
import { listQuotaPoolRechargeRecords } from './api'
import { rechargeQueryErrorMessage } from './error-message'
import type { QuotaPoolRechargeRecord, RechargeQueryPeriod } from './types'

const PAGE_SIZE = 20
const LOADING_ROWS = ['row-1', 'row-2', 'row-3', 'row-4', 'row-5']

function RechargeTypeBadge(props: { type: QuotaPoolRechargeRecord['type'] }) {
  const { t } = useTranslation()
  return props.type === 'allocate_auto' ? (
    <Badge variant='secondary'>{t('Automatic recharge')}</Badge>
  ) : (
    <Badge variant='outline'>{t('Manual recharge')}</Badge>
  )
}

function RechargeRecordsTable(props: { items: QuotaPoolRechargeRecord[] }) {
  const { t } = useTranslation()
  return (
    <div className='overflow-x-auto rounded-lg border'>
      <Table className='min-w-[900px]'>
        <TableHeader>
          <TableRow className='bg-muted/40 hover:bg-muted/40'>
            <TableHead>{t('Time')}</TableHead>
            <TableHead>{t('User')}</TableHead>
            <TableHead>{t('Quota pool')}</TableHead>
            <TableHead>{t('Type')}</TableHead>
            <TableHead className='text-right'>{t('Amount')}</TableHead>
            <TableHead>{t('Operator')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.items.map((item) => (
            <TableRow key={item.id}>
              <TableCell className='whitespace-nowrap'>
                {formatTimestamp(item.created_at)}
              </TableCell>
              <TableCell>
                <div className='font-medium'>
                  {item.user_name || `#${item.user_id}`}
                </div>
                <div className='text-muted-foreground text-xs'>
                  {item.user_email || `ID ${item.user_id}`}
                </div>
              </TableCell>
              <TableCell>{item.pool_name || `#${item.pool_id}`}</TableCell>
              <TableCell>
                <RechargeTypeBadge type={item.type} />
              </TableCell>
              <TableCell className='text-right font-medium tabular-nums'>
                {formatQuota(item.amount)}
              </TableCell>
              <TableCell>
                {item.type === 'allocate_auto'
                  ? t('System automatic recharge')
                  : item.operator_name ||
                    (item.operator_id > 0 ? `#${item.operator_id}` : '—')}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

export function RechargeRecordsCard() {
  const { t } = useTranslation()
  const [period, setPeriod] = useState<RechargeQueryPeriod>('week')
  const [page, setPage] = useState(1)
  const recordsQuery = useQuery({
    queryKey: ['quota-pool-recharge-records', period, page],
    queryFn: async () => {
      const response = await listQuotaPoolRechargeRecords({
        page,
        pageSize: PAGE_SIZE,
        period,
      })
      if (!response.success || !response.data) {
        throw new Error(
          response.message || t('We could not load recharge records.')
        )
      }
      return response.data
    },
    retry: false,
  })
  const items = recordsQuery.data?.items ?? []
  const total = recordsQuery.data?.total ?? 0
  const currentPage = recordsQuery.data?.page ?? page
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  let content: ReactNode
  if (recordsQuery.isLoading) {
    content = (
      <div className='flex flex-col gap-2'>
        {LOADING_ROWS.map((key) => (
          <Skeleton key={key} className='h-10 w-full rounded-md' />
        ))}
      </div>
    )
  } else if (recordsQuery.isError) {
    content = (
      <ErrorState
        title={t('We could not load recharge records.')}
        description={rechargeQueryErrorMessage(
          recordsQuery.error,
          t,
          t('Request failed')
        )}
        onRetry={() => void recordsQuery.refetch()}
        className='min-h-52'
      />
    )
  } else if (items.length === 0) {
    content = (
      <Empty className='min-h-40 border'>
        <EmptyHeader>
          <EmptyTitle>{t('No data')}</EmptyTitle>
          <EmptyDescription>
            {t('No recharge records in this period.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  } else {
    content = <RechargeRecordsTable items={items} />
  }

  return (
    <SettingsCard
      title={t('Recharge records')}
      description={t(
        'Only automatic and manual quota-pool member recharges are included.'
      )}
    >
      <div className='mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <p className='text-muted-foreground text-sm'>
          {period === 'week'
            ? t('Showing records from this week.')
            : t('Showing records from the past month.')}
        </p>
        <ToggleGroup
          value={[period]}
          onValueChange={(values) => {
            const nextPeriod = values.find((value) => value !== period) as
              | RechargeQueryPeriod
              | undefined
            if (!nextPeriod) return
            setPage(1)
            setPeriod(nextPeriod)
          }}
          aria-label={t('Recharge record period')}
          variant='outline'
          size='sm'
        >
          <ToggleGroupItem value='week'>{t('This week')}</ToggleGroupItem>
          <ToggleGroupItem value='month'>{t('Past month')}</ToggleGroupItem>
        </ToggleGroup>
      </div>

      <div aria-live='polite' aria-busy={recordsQuery.isFetching}>
        {recordsQuery.isFetching ? (
          <span className='sr-only' role='status'>
            {t('Loading...')}
          </span>
        ) : null}
        {content}
      </div>

      <div className='mt-4 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
        <div className='text-muted-foreground text-sm'>
          {t('Total:')} <span className='tabular-nums'>{total}</span>
          {' · '}
          {t('Page {{current}} of {{total}}', {
            current: currentPage,
            total: totalPages,
          })}
        </div>
        <ButtonGroup aria-label={t('Page')}>
          <Button
            type='button'
            size='sm'
            variant='outline'
            disabled={currentPage <= 1}
            onClick={() => setPage(currentPage - 1)}
          >
            {t('Previous page')}
          </Button>
          <Button
            type='button'
            size='sm'
            variant='outline'
            disabled={currentPage >= totalPages}
            onClick={() => setPage(currentPage + 1)}
          >
            {t('Next page')}
          </Button>
        </ButtonGroup>
      </div>
    </SettingsCard>
  )
}
