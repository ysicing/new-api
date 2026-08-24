import { useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { ButtonGroup } from '@/components/ui/button-group'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Progress } from '@/components/ui/progress'
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
  removeQuotaPoolMember,
  reclaimQuotaPoolMember,
  rechargeQuotaPoolMember,
  revokeQuotaPoolAdmin,
  setQuotaPoolAdmin,
} from '../api'
import type {
  ApiResponse,
  PageData,
  QuotaPool,
  QuotaPoolCapabilities,
  QuotaPoolMember,
} from '../types'
import { LoadingOrEmpty } from './quota-pool-data'
import { QuotaPoolMemberActionDialog } from './quota-pool-member-action-dialog'
import { QuotaPoolMemberActions } from './quota-pool-member-actions'
import { QuotaPoolMemberRemoveDialog } from './quota-pool-member-remove-dialog'

export function PoolMembers(props: {
  pool: QuotaPool
  capabilities: QuotaPoolCapabilities
  selfMode?: boolean
  query: UseQueryResult<ApiResponse<PageData<QuotaPoolMember>>>
  page: number
  pageSize: number
  keyword: string
  onSearch: (keyword: string) => void
  onPageChange: (page: number) => void
  onPageSizeChange: (pageSize: number) => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [searchValue, setSearchValue] = useState(props.keyword)
  const [quotaAction, setQuotaAction] = useState<{
    action: 'recharge' | 'reclaim'
    member: QuotaPoolMember
  }>()
  const [removeMember, setRemoveMember] = useState<QuotaPoolMember>()
  const items = props.query.data?.data?.items ?? []
  const total = props.query.data?.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / props.pageSize))
  const pageSizeOptions = [
    { value: 10, label: t('10 / page') },
    { value: 20, label: t('20 / page') },
    { value: 50, label: t('50 / page') },
  ]
  const recharge = async (userId: number) => {
    const result = await rechargeQuotaPoolMember(
      props.pool.id,
      userId,
      props.selfMode
    )
    if (!result.success) {
      toast.error(result.message || t('Recharge failed'))
      return false
    }
    toast.success(t('Recharge completed'))
    await refresh()
    return true
  }
  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({
        queryKey: ['quota-pool', props.pool.id],
      }),
      queryClient.invalidateQueries({ queryKey: ['quota-pools'] }),
    ])
  const runMemberAction = async (
    action: 'reclaim' | 'grant' | 'revoke',
    userId: number,
    amount?: number
  ) => {
    let result: ApiResponse
    if (action === 'reclaim') {
      result = await reclaimQuotaPoolMember(
        props.pool.id,
        userId,
        amount ?? 0,
        props.selfMode
      )
    } else if (action === 'grant') {
      result = await setQuotaPoolAdmin(props.pool.id, userId)
    } else {
      result = await revokeQuotaPoolAdmin(props.pool.id, userId)
    }
    if (!result.success) {
      toast.error(result.message || t('Operation failed'))
      return false
    }
    toast.success(t('Operation completed'))
    await refresh()
    return true
  }
  const remove = async (member: QuotaPoolMember) => {
    let result: ApiResponse
    try {
      result = await removeQuotaPoolMember(
        props.pool.id,
        member.id,
        props.selfMode
      )
    } catch {
      toast.error(t('Failed to remove member'))
      return false
    }
    if (!result.success) {
      toast.error(result.message || t('Failed to remove member'))
      return false
    }
    toast.success(t('Member removed'))
    await refresh()
    return true
  }
  return (
    <div className='flex flex-col gap-3 pt-4'>
      <form
        className='flex flex-col gap-2 sm:flex-row sm:items-center'
        onSubmit={(event) => {
          event.preventDefault()
          props.onSearch(searchValue.trim())
        }}
      >
        <Input
          type='search'
          aria-label={t('Search')}
          placeholder={t('Search')}
          value={searchValue}
          onChange={(event) => setSearchValue(event.target.value)}
          className='sm:max-w-sm'
        />
        <Button type='submit' variant='outline'>
          {t('Search')}
        </Button>
        <NativeSelect
          size='sm'
          aria-label={t('Rows per page')}
          value={String(props.pageSize)}
          onChange={(event) =>
            props.onPageSizeChange(Number(event.target.value))
          }
          className='sm:ml-auto'
        >
          {pageSizeOptions.map((option) => (
            <NativeSelectOption key={option.value} value={option.value}>
              {option.label}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      </form>
      <LoadingOrEmpty query={props.query} empty={items.length === 0}>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('User')}</TableHead>
              <TableHead>{t('Department')}</TableHead>
              <TableHead className='text-right'>
                {t('Available quota / Total quota')}
              </TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((member) => {
              const totalQuota = member.quota + member.used_quota
              const availablePercentage =
                totalQuota > 0
                  ? Math.min(
                      100,
                      Math.max(0, (member.quota / totalQuota) * 100)
                    )
                  : 0

              return (
                <TableRow key={member.id}>
                  <TableCell>
                    {member.display_name || member.username}
                  </TableCell>
                  <TableCell>{member.department || '—'}</TableCell>
                  <TableCell className='min-w-48'>
                    <div className='flex items-center justify-between gap-3 tabular-nums'>
                      <span>{formatQuota(member.quota)}</span>
                      <span>{formatQuota(totalQuota)}</span>
                    </div>
                    <Progress
                      value={availablePercentage}
                      aria-label={t('Available quota')}
                      className='[&_[data-slot=progress-indicator]]:bg-success mt-1.5 gap-0'
                    />
                  </TableCell>
                  <TableCell>
                    <QuotaPoolMemberActions
                      pool={props.pool}
                      capabilities={props.capabilities}
                      member={member}
                      onQuotaAction={(action) =>
                        setQuotaAction({ action, member })
                      }
                      onRemove={() => setRemoveMember(member)}
                      onAdminAction={(action) =>
                        void runMemberAction(action, member.id)
                      }
                    />
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </LoadingOrEmpty>
      <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
        <div className='text-muted-foreground text-sm'>
          {t('Total:')} <span className='tabular-nums'>{total}</span>
          {' · '}
          {t('Page {{current}} of {{total}}', {
            current: props.page,
            total: totalPages,
          })}
        </div>
        <ButtonGroup aria-label={t('Page')}>
          <Button
            size='sm'
            variant='outline'
            disabled={props.page <= 1}
            onClick={() => props.onPageChange(props.page - 1)}
          >
            {t('Previous page')}
          </Button>
          <Button
            size='sm'
            variant='outline'
            disabled={props.page >= totalPages}
            onClick={() => props.onPageChange(props.page + 1)}
          >
            {t('Next page')}
          </Button>
        </ButtonGroup>
      </div>
      {quotaAction ? (
        <QuotaPoolMemberActionDialog
          key={`${quotaAction.action}-${quotaAction.member.id}`}
          action={quotaAction.action}
          memberName={
            quotaAction.member.display_name || quotaAction.member.username
          }
          reclaimAmounts={quotaAction.member.reclaim_amounts ?? []}
          onOpenChange={(open) => !open && setQuotaAction(undefined)}
          onConfirm={(amount) =>
            quotaAction.action === 'recharge'
              ? recharge(quotaAction.member.id)
              : runMemberAction('reclaim', quotaAction.member.id, amount)
          }
        />
      ) : null}
      {removeMember ? (
        <QuotaPoolMemberRemoveDialog
          key={removeMember.id}
          member={removeMember}
          open
          onOpenChange={(open) => !open && setRemoveMember(undefined)}
          onConfirm={() => remove(removeMember)}
        />
      ) : null}
    </div>
  )
}
