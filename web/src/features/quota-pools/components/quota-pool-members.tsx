import { useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { ButtonGroup } from '@/components/ui/button-group'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
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
    queryClient.invalidateQueries({ queryKey: ['quota-pool', props.pool.id] })
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
      result = await setQuotaPoolAdmin(props.pool.id, userId, 1, props.selfMode)
    } else {
      result = await revokeQuotaPoolAdmin(props.pool.id, userId, props.selfMode)
    }
    if (!result.success) {
      toast.error(result.message || t('Operation failed'))
      return false
    }
    toast.success(t('Operation completed'))
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
              <TableHead className='text-right'>{t('Quota')}</TableHead>
              <TableHead />
            </TableRow>
          </TableHeader>
          <TableBody>
            {items.map((member) => (
              <TableRow key={member.id}>
                <TableCell>{member.display_name || member.username}</TableCell>
                <TableCell>{member.department || '—'}</TableCell>
                <TableCell className='text-right'>
                  {formatQuota(member.quota)}
                </TableCell>
                <TableCell>
                  <div className='flex justify-end gap-1'>
                    {props.capabilities.can_manage_members && (
                      <>
                        <Button
                          size='sm'
                          variant='outline'
                          onClick={() =>
                            setQuotaAction({ action: 'recharge', member })
                          }
                        >
                          {t('Recharge')}
                        </Button>
                        <Button
                          size='sm'
                          variant='outline'
                          disabled={(member.reclaim_amounts?.length ?? 0) === 0}
                          onClick={() =>
                            setQuotaAction({ action: 'reclaim', member })
                          }
                        >
                          {t('Reclaim')}
                        </Button>
                      </>
                    )}
                    {props.capabilities.can_manage_v1_admins && (
                      <Button
                        size='sm'
                        variant='ghost'
                        onClick={() =>
                          void runMemberAction(
                            member.quota_pool_admin_level > 0
                              ? 'revoke'
                              : 'grant',
                            member.id
                          )
                        }
                      >
                        {t(
                          member.quota_pool_admin_level > 0
                            ? 'Remove pool admin'
                            : 'Set pool admin'
                        )}
                      </Button>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            ))}
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
    </div>
  )
}
