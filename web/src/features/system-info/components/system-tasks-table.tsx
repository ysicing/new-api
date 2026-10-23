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
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { Badge } from '@/components/ui/badge'
import {
  Collapsible,
  CollapsibleTrigger,
  CollapsibleContent,
} from '@/components/ui/collapsible'
import { Progress } from '@/components/ui/progress'
import {
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import type {
  SystemTask,
  SystemTaskStatus,
} from '@/features/system-settings/types'
import { toIntlLocale } from '@/i18n/languages'
import { formatTimestampRelative, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import { SYSTEM_TASK_TYPE_LABEL } from '../constants'

const STATUS_VARIANT: Record<SystemTaskStatus, 'secondary' | 'destructive'> = {
  pending: 'secondary',
  running: 'secondary',
  succeeded: 'secondary',
  failed: 'destructive',
}

const STATUS_CLASS_NAME: Record<SystemTaskStatus, string> = {
  pending:
    'bg-amber-50 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300',
  running:
    'bg-sky-50 text-sky-700 dark:bg-sky-500/15 dark:text-sky-300 [&_span]:bg-sky-500',
  succeeded:
    'bg-emerald-50 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300',
  failed: '',
}

const STATUS_DOT_CLASS_NAME: Record<SystemTaskStatus, string> = {
  pending: 'bg-amber-500',
  running: 'bg-sky-500',
  succeeded: 'bg-emerald-500',
  failed: 'bg-destructive',
}

const PROGRESS_BAR_CLASS_NAME: Record<SystemTaskStatus, string> = {
  pending: '[&_[data-slot=progress-indicator]]:bg-amber-500',
  running: '[&_[data-slot=progress-indicator]]:bg-sky-500',
  succeeded: '[&_[data-slot=progress-indicator]]:bg-emerald-500',
  failed: '[&_[data-slot=progress-indicator]]:bg-destructive',
}

const TYPE_DISPLAY_ID: Record<string, string> = {
  midjourney_poll: 'drawing_task_poll',
}

function getProgress(task: SystemTask): number | null {
  const progress = (task.state as { progress?: unknown } | undefined)?.progress
  if (typeof progress !== 'number' || Number.isNaN(progress)) return null
  return Math.min(100, Math.max(0, progress))
}

const MAGPIE_COUNTER_LABELS: Record<string, string> = {
  checked: 'Channels checked: {{count}}',
  skipped: 'Channel probes deferred: {{count}}',
  enabled: 'Channels enabled: {{count}}',
  disabled: 'Channels disabled: {{count}}',
  unchanged: 'Channels unchanged: {{count}}',
  failed: 'Channel checks failed: {{count}}',
  notification_failed: 'Notifications failed: {{count}}',
}

const MAGPIE_ACTION_LABELS: Record<string, string> = {
  cached: 'Waiting for the next availability check',
  enabled: 'Automatically enabled',
  disabled: 'Automatically disabled',
  unchanged: 'Channel status preserved',
  query_failed: 'Availability query failed',
  update_failed: 'Channel status update failed',
}

const AUTO_RECHARGE_SKIP_GROUPS = [
  {
    keys: ['quota_above_threshold'],
    label: 'Balance above threshold: {{count}}',
  },
  {
    keys: ['weekly_limited'],
    label: 'Weekly limit reached: {{count}}',
  },
  {
    keys: ['monthly_limited'],
    label: 'Monthly limit reached: {{count}}',
  },
  {
    keys: [
      'quota_pool_not_found',
      'new_user_pool_disabled',
      'quota_pool_disabled',
    ],
    label: 'Quota pool unavailable: {{count}}',
  },
  {
    keys: ['quota_pool_insufficient'],
    label: 'Quota pool insufficient: {{count}}',
  },
  {
    keys: ['disabled', 'amount_not_configured'],
    label: 'Recharge configuration disabled: {{count}}',
  },
  {
    keys: ['weekly_count_failed', 'monthly_count_failed', 'recharge_failed'],
    label: 'Recharge check failed: {{count}}',
  },
] as const

type TaskDetail = {
  summary: string
  reasons?: string
  channels?: {
    channelID: number
    channelName: string
    available?: boolean
    action: string
    error?: string
    notificationError?: string
    nextCheckAt?: string
  }[]
}

// Renders a succeeded task's main counters separately from skip reasons so
// scheduled maintenance details stay readable in a table row.
function useTaskDetail() {
  const { t, i18n } = useTranslation()
  return (task: SystemTask): TaskDetail | null => {
    const result = task.result as Record<string, unknown> | undefined
    if (
      task.type === 'magpie_channel_monitor' &&
      (task.status === 'succeeded' || task.status === 'failed') &&
      result
    ) {
      const parts: string[] = []
      for (const [key, label] of Object.entries(MAGPIE_COUNTER_LABELS)) {
        if (typeof result[key] === 'number') {
          parts.push(t(label, { count: result[key] }))
        }
      }
      // 旧任务仅保存 changed，不能补造当时的渠道查询详情。
      if (parts.length === 0 && typeof result.changed === 'number') {
        parts.push(t('Channels changed: {{count}}', { count: result.changed }))
      }
      const channels: NonNullable<TaskDetail['channels']> = []
      if (Array.isArray(result.channels)) {
        for (const value of result.channels) {
          if (!value || typeof value !== 'object') continue
          const entry = value as Record<string, unknown>
          if (
            typeof entry.channel_id !== 'number' ||
            typeof entry.channel_name !== 'string' ||
            typeof entry.action !== 'string'
          ) {
            continue
          }
          channels.push({
            channelID: entry.channel_id,
            channelName: entry.channel_name,
            action: entry.action,
            available:
              typeof entry.available === 'boolean'
                ? entry.available
                : undefined,
            error: typeof entry.error === 'string' ? entry.error : undefined,
            notificationError:
              typeof entry.notification_error === 'string'
                ? entry.notification_error
                : undefined,
            nextCheckAt:
              typeof entry.next_check_at === 'string' &&
              Number.isFinite(Date.parse(entry.next_check_at))
                ? new Date(entry.next_check_at).toLocaleString(
                    toIntlLocale(i18n.language)
                  )
                : undefined,
          })
        }
      }
      return {
        summary: task.error || parts.join(' · ') || '-',
        reasons: task.error && parts.length > 0 ? parts.join(' · ') : undefined,
        channels,
      }
    }
    if (task.error) return { summary: task.error }
    if (task.status !== 'succeeded' || !result) return null
    const summaryParts: string[] = []
    const reasonParts: string[] = []
    if (typeof result.monthly_refilled === 'number') {
      summaryParts.push(
        t('Pools refilled: {{count}}', { count: result.monthly_refilled })
      )
    }
    const usersRecharged =
      typeof result.users_recharged === 'number' ? result.users_recharged : null
    const usersSkipped =
      typeof result.users_skipped === 'number' ? result.users_skipped : null
    let usersChecked: number | null = null
    if (typeof result.users_checked === 'number') {
      usersChecked = result.users_checked
    } else if (usersRecharged != null && usersSkipped != null) {
      usersChecked = usersRecharged + usersSkipped
    }
    if (usersChecked != null) {
      summaryParts.push(t('Users checked: {{count}}', { count: usersChecked }))
    }
    if (usersRecharged != null) {
      summaryParts.push(
        t('Users recharged: {{count}}', { count: usersRecharged })
      )
    }
    if (usersSkipped != null) {
      summaryParts.push(
        t('Users not eligible: {{count}}', { count: usersSkipped })
      )
    }
    const skipReasons = result.skip_reasons
    if (
      skipReasons &&
      typeof skipReasons === 'object' &&
      !Array.isArray(skipReasons)
    ) {
      const reasonCounts = skipReasons as Record<string, unknown>
      for (const group of AUTO_RECHARGE_SKIP_GROUPS) {
        const count = group.keys.reduce((total, key) => {
          const value = reasonCounts[key]
          return total + (typeof value === 'number' ? value : 0)
        }, 0)
        if (count > 0) reasonParts.push(t(group.label, { count }))
      }
    }
    if (typeof result.deleted_count === 'number') {
      summaryParts.push(
        t('Rows deleted: {{count}}', { count: result.deleted_count })
      )
    }
    if (summaryParts.length === 0) return null
    return {
      summary: summaryParts.join(' · '),
      reasons: reasonParts.length > 0 ? reasonParts.join(' · ') : undefined,
    }
  }
}

function TaskDetailContent({ detail }: { detail: TaskDetail | null }) {
  const { t } = useTranslation()
  if (!detail) return '-'
  return (
    <div className='space-y-0.5'>
      <div>{detail.summary}</div>
      {detail.reasons ? (
        <div className='text-muted-foreground/80'>{detail.reasons}</div>
      ) : null}
      {detail.channels && detail.channels.length > 0 ? (
        <Collapsible>
          <CollapsibleTrigger className='text-foreground cursor-pointer text-xs underline underline-offset-4'>
            {t('Channel results')}
          </CollapsibleTrigger>
          <CollapsibleContent>
            <ul className='mt-2 max-h-64 space-y-2 overflow-y-auto'>
              {detail.channels.map((channel) => (
                <li key={channel.channelID} className='break-words'>
                  <div className='text-foreground font-medium'>
                    {channel.channelName} (#{channel.channelID})
                  </div>
                  <div>
                    {t(MAGPIE_ACTION_LABELS[channel.action] ?? channel.action)}
                    {channel.available === true
                      ? ` · ${t('Codex accounts available')}`
                      : null}
                    {channel.available === false
                      ? ` · ${t('No available Codex accounts')}`
                      : null}
                  </div>
                  {channel.error ? (
                    <div className='text-destructive'>{channel.error}</div>
                  ) : null}
                  {channel.nextCheckAt ? (
                    <div>
                      {t('Next availability check no earlier than: {{time}}', {
                        time: channel.nextCheckAt,
                      })}
                    </div>
                  ) : null}
                  {channel.notificationError ? (
                    <div className='text-destructive'>
                      {t('Notification failed: {{error}}', {
                        error: channel.notificationError,
                      })}
                    </div>
                  ) : null}
                </li>
              ))}
            </ul>
          </CollapsibleContent>
        </Collapsible>
      ) : null}
    </div>
  )
}

type SystemTasksTableProps = {
  tasks: SystemTask[]
}

export function SystemTasksTable(props: SystemTasksTableProps) {
  const { t, i18n } = useTranslation()
  const taskDetail = useTaskDetail()

  return (
    <StaticDataTable tableClassName='min-w-[900px]'>
      <TableHeader>
        <TableRow className='bg-muted/40 hover:bg-muted/40'>
          <TableHead className='h-9 w-[260px] px-4 text-xs'>
            {t('Type')}
          </TableHead>
          <TableHead className='h-9 w-[130px] text-xs'>{t('Status')}</TableHead>
          <TableHead className='h-9 w-[180px] text-xs'>
            {t('Progress')}
          </TableHead>
          <TableHead className='h-9 min-w-[260px] text-xs'>
            {t('Executor')}
          </TableHead>
          <TableHead className='h-9 w-[190px] text-xs'>
            {t('Updated')}
          </TableHead>
          <TableHead className='h-9 w-[220px] pr-4 text-xs'>
            {t('Detail')}
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {props.tasks.map((task) => {
          const progress = getProgress(task)
          return (
            <TableRow key={task.task_id} className='hover:bg-muted/30'>
              <TableCell className='px-4 py-3 align-middle'>
                <div className='space-y-0.5'>
                  <div className='font-medium'>
                    {t(SYSTEM_TASK_TYPE_LABEL[task.type] ?? task.type)}
                  </div>
                  <div className='text-muted-foreground font-mono text-[11px]'>
                    {TYPE_DISPLAY_ID[task.type] ?? task.type}
                  </div>
                </div>
              </TableCell>
              <TableCell className='py-3 align-middle'>
                <Badge
                  variant={STATUS_VARIANT[task.status]}
                  className={cn('gap-1.5', STATUS_CLASS_NAME[task.status])}
                >
                  <span
                    className={cn(
                      'size-1.5 rounded-full',
                      STATUS_DOT_CLASS_NAME[task.status]
                    )}
                    aria-hidden='true'
                  />
                  {t(task.status)}
                </Badge>
              </TableCell>
              <TableCell className='py-3 align-middle'>
                <div className='flex items-center gap-2'>
                  <Progress
                    value={progress ?? 0}
                    className={cn('w-24', PROGRESS_BAR_CLASS_NAME[task.status])}
                  />
                  <span className='text-muted-foreground w-10 text-right text-xs tabular-nums'>
                    {progress === null ? '-' : `${progress}%`}
                  </span>
                </div>
              </TableCell>
              <TableCell className='text-muted-foreground max-w-[280px] truncate py-3 align-middle font-mono text-xs'>
                {task.locked_by || '-'}
              </TableCell>
              <TableCell
                className='text-muted-foreground py-3 align-middle text-xs whitespace-nowrap'
                title={formatTimestampToDate(task.updated_at)}
              >
                {formatTimestampRelative(
                  task.updated_at,
                  'seconds',
                  toIntlLocale(i18n.language)
                )}
              </TableCell>
              <TableCell
                className='max-w-[440px] py-3 pr-4 align-middle text-xs break-words'
                title={task.error || undefined}
              >
                <TaskDetailContent detail={taskDetail(task)} />
              </TableCell>
            </TableRow>
          )
        })}
      </TableBody>
    </StaticDataTable>
  )
}
