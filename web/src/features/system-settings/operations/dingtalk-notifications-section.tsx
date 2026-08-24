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
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatTimestampToDate } from '@/lib/format'

import { listDingTalkNotifications } from '../api'
import { SettingsSection } from '../components/settings-section'
import type {
  DingTalkNotificationStatus,
  DingTalkNotificationRecord,
} from '../types'

const PAGE_SIZE = 20
const LOADING_ROW_KEYS = [
  'loading-1',
  'loading-2',
  'loading-3',
  'loading-4',
  'loading-5',
]

type NotificationFilters = {
  eventType: string
  status: DingTalkNotificationStatus | ''
  keyword: string
  startTime: string
  endTime: string
}

const EMPTY_FILTERS: NotificationFilters = {
  eventType: '',
  status: '',
  keyword: '',
  startTime: '',
  endTime: '',
}

const STATUS_BADGE: Record<
  DingTalkNotificationStatus,
  {
    label: string
    variant: 'secondary' | 'destructive' | 'warning'
    className?: string
  }
> = {
  pending: { label: 'Pending', variant: 'warning' },
  succeeded: {
    label: 'Succeeded',
    variant: 'secondary',
    className: 'bg-success/10 text-success',
  },
  failed: { label: 'Failed', variant: 'destructive' },
  skipped: { label: 'Skipped', variant: 'secondary' },
}

function toTimestamp(value: string): number | undefined {
  if (!value) return undefined
  const timestamp = new Date(value).getTime()
  return Number.isNaN(timestamp) ? undefined : Math.floor(timestamp / 1000)
}

function eventTypeLabel(eventType: string) {
  if (eventType === 'new_user_quota_exhausted') {
    return 'New-user quota exhausted'
  }
  return eventType
}

function NotificationStatusBadge(props: {
  status: DingTalkNotificationStatus
}) {
  const { t } = useTranslation()
  const config = STATUS_BADGE[props.status]
  return (
    <Badge variant={config.variant} className={config.className}>
      {t(config.label)}
    </Badge>
  )
}

function NotificationDetail(props: { error: string; metadata: string }) {
  const hasMetadata =
    props.metadata !== '' &&
    props.metadata !== '{}' &&
    props.metadata !== 'null'
  if (!props.error && !hasMetadata) return '—'
  return (
    <div className='space-y-1'>
      {props.error ? <div>{props.error}</div> : null}
      {hasMetadata ? (
        <code className='text-foreground block break-all whitespace-normal'>
          {props.metadata}
        </code>
      ) : null}
    </div>
  )
}

function NotificationTable(props: { items: DingTalkNotificationRecord[] }) {
  const { t } = useTranslation()
  return (
    <div className='overflow-x-auto rounded-md border'>
      <Table className='min-w-[1100px]'>
        <TableHeader>
          <TableRow className='bg-muted/40 hover:bg-muted/40'>
            <TableHead>{t('Created')}</TableHead>
            <TableHead>{t('Notification type')}</TableHead>
            <TableHead>{t('User')}</TableHead>
            <TableHead>{t('Recipient')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
            <TableHead className='min-w-[280px]'>{t('Message')}</TableHead>
            <TableHead className='min-w-[220px]'>{t('Detail')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.items.map((item) => (
            <TableRow key={item.id}>
              <TableCell className='whitespace-nowrap'>
                <div>{formatTimestampToDate(item.created_at)}</div>
                <div className='text-muted-foreground mt-1 text-xs'>
                  {t('Sent at')}: {formatTimestampToDate(item.sent_at)}
                </div>
              </TableCell>
              <TableCell>{t(eventTypeLabel(item.event_type))}</TableCell>
              <TableCell>
                <div className='font-medium'>{item.username || '—'}</div>
                <div className='text-muted-foreground text-xs'>
                  ID {item.user_id}
                </div>
              </TableCell>
              <TableCell className='font-mono text-xs'>
                {item.recipient || '—'}
              </TableCell>
              <TableCell>
                <NotificationStatusBadge status={item.status} />
              </TableCell>
              <TableCell>
                <div className='font-medium'>{item.title}</div>
                <div className='text-muted-foreground mt-1 whitespace-normal'>
                  {item.content}
                </div>
              </TableCell>
              <TableCell className='text-muted-foreground whitespace-normal'>
                <NotificationDetail
                  error={item.error}
                  metadata={item.metadata}
                />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}

export function DingTalkNotificationsSection() {
  const { t } = useTranslation()
  const [formFilters, setFormFilters] = useState<NotificationFilters>({
    ...EMPTY_FILTERS,
  })
  const [filters, setFilters] = useState<NotificationFilters>({
    ...EMPTY_FILTERS,
  })
  const [page, setPage] = useState(1)
  const notificationsQuery = useQuery({
    queryKey: ['dingtalk-notifications', page, filters],
    queryFn: async () => {
      const response = await listDingTalkNotifications({
        page,
        pageSize: PAGE_SIZE,
        eventType: filters.eventType,
        status: filters.status,
        keyword: filters.keyword,
        startTimestamp: toTimestamp(filters.startTime),
        endTimestamp: toTimestamp(filters.endTime),
      })
      if (!response.success || !response.data) {
        throw new Error(
          response.message || t('We could not load DingTalk notifications.')
        )
      }
      return response.data
    },
    retry: false,
  })
  const items = notificationsQuery.data?.items ?? []
  const total = notificationsQuery.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  let recordsContent: ReactNode
  if (notificationsQuery.isLoading) {
    recordsContent = (
      <div className='space-y-2'>
        {LOADING_ROW_KEYS.map((key) => (
          <Skeleton key={key} className='h-10 w-full rounded-md' />
        ))}
      </div>
    )
  } else if (notificationsQuery.isError) {
    recordsContent = (
      <ErrorState
        title={t('We could not load DingTalk notifications.')}
        description={
          notificationsQuery.error instanceof Error
            ? notificationsQuery.error.message
            : undefined
        }
        onRetry={() => void notificationsQuery.refetch()}
        className='min-h-[260px]'
      />
    )
  } else if (items.length === 0) {
    recordsContent = (
      <div className='text-muted-foreground rounded-md border border-dashed px-4 py-10 text-center text-sm'>
        {t('No DingTalk notification records.')}
      </div>
    )
  } else {
    recordsContent = <NotificationTable items={items} />
  }

  return (
    <SettingsSection title={t('DingTalk notification records')}>
      <form
        className='grid gap-3 md:grid-cols-2 xl:grid-cols-5'
        onSubmit={(event) => {
          event.preventDefault()
          setPage(1)
          setFilters({ ...formFilters, keyword: formFilters.keyword.trim() })
        }}
      >
        <Field>
          <FieldLabel htmlFor='dingtalk-notification-type'>
            {t('Notification type')}
          </FieldLabel>
          <Input
            id='dingtalk-notification-type'
            placeholder={t('Notification type')}
            value={formFilters.eventType}
            onChange={(event) =>
              setFormFilters((current) => ({
                ...current,
                eventType: event.target.value,
              }))
            }
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='dingtalk-notification-status'>
            {t('Status')}
          </FieldLabel>
          <NativeSelect
            id='dingtalk-notification-status'
            value={formFilters.status}
            onChange={(event) =>
              setFormFilters((current) => ({
                ...current,
                status: event.target.value as DingTalkNotificationStatus | '',
              }))
            }
          >
            <NativeSelectOption value=''>
              {t('All statuses')}
            </NativeSelectOption>
            {Object.entries(STATUS_BADGE).map(([status, config]) => (
              <NativeSelectOption key={status} value={status}>
                {t(config.label)}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </Field>
        <Field>
          <FieldLabel htmlFor='dingtalk-notification-keyword'>
            {t('User or recipient')}
          </FieldLabel>
          <Input
            id='dingtalk-notification-keyword'
            type='search'
            placeholder={t('User or recipient')}
            value={formFilters.keyword}
            onChange={(event) =>
              setFormFilters((current) => ({
                ...current,
                keyword: event.target.value,
              }))
            }
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='dingtalk-notification-start-time'>
            {t('Start time')}
          </FieldLabel>
          <Input
            id='dingtalk-notification-start-time'
            type='datetime-local'
            value={formFilters.startTime}
            onChange={(event) =>
              setFormFilters((current) => ({
                ...current,
                startTime: event.target.value,
              }))
            }
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='dingtalk-notification-end-time'>
            {t('End time')}
          </FieldLabel>
          <Input
            id='dingtalk-notification-end-time'
            type='datetime-local'
            value={formFilters.endTime}
            onChange={(event) =>
              setFormFilters((current) => ({
                ...current,
                endTime: event.target.value,
              }))
            }
          />
        </Field>
        <div className='flex gap-2 md:col-span-2 xl:col-span-5'>
          <Button type='submit'>{t('Search')}</Button>
          <Button
            type='button'
            variant='outline'
            disabled={notificationsQuery.isFetching}
            onClick={() => void notificationsQuery.refetch()}
          >
            {t('Refresh')}
          </Button>
        </div>
      </form>

      {recordsContent}

      <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
        <div className='text-muted-foreground text-sm'>
          {t('Total:')} <span className='tabular-nums'>{total}</span>
          {' · '}
          {t('Page {{current}} of {{total}}', {
            current: page,
            total: totalPages,
          })}
        </div>
        <ButtonGroup aria-label={t('Page')}>
          <Button
            type='button'
            size='sm'
            variant='outline'
            disabled={page <= 1}
            onClick={() => setPage((current) => current - 1)}
          >
            {t('Previous page')}
          </Button>
          <Button
            type='button'
            size='sm'
            variant='outline'
            disabled={page >= totalPages}
            onClick={() => setPage((current) => current + 1)}
          >
            {t('Next page')}
          </Button>
        </ButtonGroup>
      </div>
    </SettingsSection>
  )
}
