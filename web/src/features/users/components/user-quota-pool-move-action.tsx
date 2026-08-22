/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useQuery } from '@tanstack/react-query'
import { ArrowRightLeft, TriangleAlert } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription } from '@/components/ui/alert'
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
import { Button } from '@/components/ui/button'
import { Field, FieldLabel } from '@/components/ui/field'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getQuotaPools, moveUserQuotaPool } from '@/features/quota-pools/api'
import { useAuthStore } from '@/stores/auth-store'

import { ERROR_MESSAGES, USER_ROLE } from '../constants'
import type { User } from '../types'

const QUOTA_POOL_MEMBER_ROLES = new Set<number>([
  USER_ROLE.USER,
  USER_ROLE.QUOTA_POOL_SUPER_ADMIN,
  USER_ROLE.ADMIN,
])

export function UserQuotaPoolMoveAction(props: {
  user: User
  onSaved: () => void
}) {
  const { t } = useTranslation()
  const currentUser = useAuthStore((state) => state.auth.user)
  const [open, setOpen] = useState(false)
  const [targetPoolId, setTargetPoolId] = useState<string>()
  const [saving, setSaving] = useState(false)
  const query = useQuery({
    queryKey: ['quota-pools', 'user-migration'],
    queryFn: getQuotaPools,
    enabled: open,
  })
  const currentPoolId = props.user.quota_pool_id ?? 0
  const options = useMemo(
    () =>
      (query.data?.data?.items ?? [])
        .map((pool) => ({
          label: pool.name || t('Default pool'),
          value: pool.is_default ? 0 : pool.id,
          available: pool.is_default || pool.enabled,
        }))
        .filter((pool) => pool.available && pool.value !== currentPoolId),
    [currentPoolId, query.data?.data?.items, t]
  )
  const canMove = Boolean(
    currentUser?.quota_pool_enabled &&
    currentUser.role >= USER_ROLE.ADMIN &&
    QUOTA_POOL_MEMBER_ROLES.has(props.user.role)
  )
  if (!canMove) return null

  const failed = query.isError || query.data?.success === false
  const handleOpenChange = (nextOpen: boolean) => {
    if (!nextOpen) setTargetPoolId(undefined)
    setOpen(nextOpen)
  }
  const move = async () => {
    const poolId = Number(targetPoolId)
    if (!Number.isInteger(poolId) || poolId < 0) return
    setSaving(true)
    try {
      const result = await moveUserQuotaPool(props.user.id, poolId)
      if (!result.success) {
        toast.error(result.message || t('Operation failed'))
        return
      }
      toast.success(t('Quota pool moved'))
      props.onSaved()
      handleOpenChange(false)
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setSaving(false)
    }
  }

  return (
    <>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant='ghost'
              size='icon-sm'
              onClick={() => setOpen(true)}
              aria-label={t('Move quota pool')}
            />
          }
        >
          <ArrowRightLeft />
        </TooltipTrigger>
        <TooltipContent>{t('Move quota pool')}</TooltipContent>
      </Tooltip>

      <AlertDialog open={open} onOpenChange={handleOpenChange}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Move quota pool')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('Move {{username}} to another quota pool.', {
                username: props.user.username,
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>

          <Field>
            <FieldLabel htmlFor='target-quota-pool'>
              {t('Target quota pool')}
            </FieldLabel>
            {query.isLoading ? (
              <Skeleton className='h-8 w-full' />
            ) : (
              <Select
                value={targetPoolId}
                onValueChange={(value) => setTargetPoolId(value ?? undefined)}
                disabled={failed || options.length === 0}
              >
                <SelectTrigger id='target-quota-pool' className='w-full'>
                  <SelectValue placeholder={t('Select target quota pool')} />
                </SelectTrigger>
                <SelectContent>
                  {options.map((option) => (
                    <SelectItem key={option.value} value={String(option.value)}>
                      {option.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
            {!query.isLoading && failed ? (
              <p role='alert' className='text-destructive text-sm'>
                {t('Failed to load quota pools.')}
              </p>
            ) : null}
            {!query.isLoading && !failed && options.length === 0 ? (
              <p className='text-muted-foreground text-sm'>
                {t('No available target quota pools.')}
              </p>
            ) : null}
          </Field>

          <Alert>
            <TriangleAlert aria-hidden='true' />
            <AlertDescription>
              {t(
                "Moving will clear the user's current quota and return the balance to the original pool according to its rules."
              )}
            </AlertDescription>
          </Alert>

          <AlertDialogFooter>
            <AlertDialogCancel disabled={saving}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={saving || !targetPoolId || failed}
              onClick={() => void move()}
            >
              {saving ? <Spinner data-icon='inline-start' /> : null}
              {t('Confirm move')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
