/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { ROLE } from '@/lib/roles'

import type {
  QuotaPool,
  QuotaPoolCapabilities,
  QuotaPoolMember,
} from '../types'

export function QuotaPoolMemberActions(props: {
  pool: QuotaPool
  capabilities: QuotaPoolCapabilities
  member: QuotaPoolMember
  onQuotaAction: (action: 'recharge' | 'reclaim') => void
  onRemove: () => void
  onAdminAction: (action: 'grant' | 'revoke') => void
}) {
  const { t } = useTranslation()
  const canRemoveMember =
    props.pool.pool_type === 'normal' &&
    props.capabilities.can_remove_members &&
    (props.capabilities.can_manage_admins ||
      (!props.member.quota_pool_admin && props.member.role === ROLE.USER))

  return (
    <div className='flex justify-end gap-1'>
      <QuotaPoolMemberQuotaActions
        visible={props.capabilities.can_manage_members}
        reclaimDisabled={(props.member.reclaim_amounts?.length ?? 0) === 0}
        onAction={props.onQuotaAction}
      />
      {canRemoveMember ? (
        <Button size='sm' variant='destructive' onClick={props.onRemove}>
          {t('Remove member')}
        </Button>
      ) : null}
      <QuotaPoolMemberAdminAction
        visible={props.capabilities.can_manage_admins}
        administrator={props.member.quota_pool_admin}
        onAction={props.onAdminAction}
      />
    </div>
  )
}

function QuotaPoolMemberQuotaActions(props: {
  visible: boolean
  reclaimDisabled: boolean
  onAction: (action: 'recharge' | 'reclaim') => void
}) {
  const { t } = useTranslation()
  if (!props.visible) return null
  return (
    <>
      <Button
        size='sm'
        variant='outline'
        onClick={() => props.onAction('recharge')}
      >
        {t('Recharge')}
      </Button>
      <Button
        size='sm'
        variant='outline'
        disabled={props.reclaimDisabled}
        onClick={() => props.onAction('reclaim')}
      >
        {t('Reclaim')}
      </Button>
    </>
  )
}

function QuotaPoolMemberAdminAction(props: {
  visible: boolean
  administrator: boolean
  onAction: (action: 'grant' | 'revoke') => void
}) {
  const { t } = useTranslation()
  if (!props.visible) return null
  const action = props.administrator ? 'revoke' : 'grant'
  const label = props.administrator
    ? t('Remove pool administrator')
    : t('Set pool administrator')
  return (
    <Button size='sm' variant='ghost' onClick={() => props.onAction(action)}>
      {label}
    </Button>
  )
}
