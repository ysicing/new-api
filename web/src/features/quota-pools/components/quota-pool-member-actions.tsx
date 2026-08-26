/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useTranslation } from 'react-i18next'

import { DataTableRowActionMenu } from '@/components/data-table/core/row-action-menu'
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu'
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
  rechargeDisabled?: boolean
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
  const canGrantPoolAdmin =
    props.member.role === ROLE.USER ||
    props.member.role === ROLE.QUOTA_POOL_SUPER_ADMIN ||
    props.member.role === ROLE.ADMIN
  const canManagePoolAdmin =
    props.capabilities.can_manage_admins &&
    (props.member.quota_pool_admin || canGrantPoolAdmin)
  const showQuotaActions = props.capabilities.can_manage_members
  const hasActions = showQuotaActions || canRemoveMember || canManagePoolAdmin

  if (!hasActions) return null

  return (
    <DataTableRowActionMenu ariaLabel={t('Open menu')}>
      {showQuotaActions ? (
        <>
          <DropdownMenuItem
            disabled={props.rechargeDisabled}
            onClick={() => props.onQuotaAction('recharge')}
          >
            {t('Recharge')}
          </DropdownMenuItem>
          <DropdownMenuItem
            disabled={(props.member.reclaim_amounts?.length ?? 0) === 0}
            onClick={() => props.onQuotaAction('reclaim')}
          >
            {t('Reclaim')}
          </DropdownMenuItem>
        </>
      ) : null}

      {showQuotaActions && (canManagePoolAdmin || canRemoveMember) ? (
        <DropdownMenuSeparator />
      ) : null}

      {canManagePoolAdmin ? (
        <DropdownMenuItem
          onClick={() =>
            props.onAdminAction(
              props.member.quota_pool_admin ? 'revoke' : 'grant'
            )
          }
        >
          {props.member.quota_pool_admin
            ? t('Remove pool administrator')
            : t('Set pool administrator')}
        </DropdownMenuItem>
      ) : null}

      {canRemoveMember && (showQuotaActions || canManagePoolAdmin) ? (
        <DropdownMenuSeparator />
      ) : null}

      {canRemoveMember ? (
        <DropdownMenuItem variant='destructive' onClick={props.onRemove}>
          {t('Remove member')}
        </DropdownMenuItem>
      ) : null}
    </DataTableRowActionMenu>
  )
}
