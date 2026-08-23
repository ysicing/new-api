/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { formatQuota } from '@/lib/format'

import type { QuotaPoolMember } from '../types'

export function QuotaPoolMemberRemoveDialog(props: {
  member: QuotaPoolMember
  open: boolean
  onOpenChange: (open: boolean) => void
  onConfirm: () => Promise<boolean>
}) {
  const { t } = useTranslation()
  const [processing, setProcessing] = useState(false)
  const confirm = async () => {
    setProcessing(true)
    try {
      if (await props.onConfirm()) {
        props.onOpenChange(false)
      }
    } finally {
      setProcessing(false)
    }
  }

  return (
    <AlertDialog
      open={props.open}
      onOpenChange={(open) => !processing && props.onOpenChange(open)}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('Remove member')}</AlertDialogTitle>
          <QuotaPoolMemberRemovalDescription member={props.member} />
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={processing}>
            {t('Cancel')}
          </AlertDialogCancel>
          <AlertDialogAction
            variant='destructive'
            disabled={processing}
            onClick={(event) => {
              event.preventDefault()
              void confirm()
            }}
          >
            {t('Confirm removal')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

function QuotaPoolMemberRemovalDescription(props: { member: QuotaPoolMember }) {
  const { t } = useTranslation()
  const memberName = props.member.display_name || props.member.username
  return (
    <AlertDialogDescription>
      {t('Remove {{user}} from this quota pool?', { user: memberName })}{' '}
      {t(
        "The member's current balance of {{quota}} will be returned to this quota pool. The member will move to the new-user default pool with a zero balance. No trial quota will be granted again.",
        { quota: formatQuota(props.member.quota) }
      )}
    </AlertDialogDescription>
  )
}
