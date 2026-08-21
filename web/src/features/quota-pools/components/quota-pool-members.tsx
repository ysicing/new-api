import { useQueryClient, type UseQueryResult } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
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

export function PoolMembers(props: {
  pool: QuotaPool
  capabilities: QuotaPoolCapabilities
  selfMode?: boolean
  query: UseQueryResult<ApiResponse<PageData<QuotaPoolMember>>>
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const items = props.query.data?.data?.items ?? []
  const recharge = async (userId: number) => {
    const result = await rechargeQuotaPoolMember(
      props.pool.id,
      userId,
      props.selfMode
    )
    if (!result.success) {
      return toast.error(result.message || t('Recharge failed'))
    }
    toast.success(t('Recharge completed'))
    await refresh()
  }
  const refresh = () =>
    queryClient.invalidateQueries({ queryKey: ['quota-pool', props.pool.id] })
  const runMemberAction = async (
    action: 'reclaim' | 'grant' | 'revoke',
    userId: number
  ) => {
    let result: ApiResponse
    if (action === 'reclaim') {
      result = await reclaimQuotaPoolMember(
        props.pool.id,
        userId,
        props.selfMode
      )
    } else if (action === 'grant') {
      result = await setQuotaPoolAdmin(props.pool.id, userId, 1, props.selfMode)
    } else {
      result = await revokeQuotaPoolAdmin(props.pool.id, userId, props.selfMode)
    }
    if (!result.success) {
      return toast.error(result.message || t('Operation failed'))
    }
    toast.success(t('Operation completed'))
    await refresh()
  }
  return (
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
                        onClick={() => void recharge(member.id)}
                      >
                        {t('Recharge')}
                      </Button>
                      <Button
                        size='sm'
                        variant='outline'
                        onClick={() =>
                          void runMemberAction('reclaim', member.id)
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
  )
}
