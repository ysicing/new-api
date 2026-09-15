import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import { handleServerError } from '@/lib/handle-server-error'

import { setQuotaPoolEnabled } from '../api'
import type { QuotaPool } from '../types'

export function QuotaPoolStatusAction(props: {
  pool: QuotaPool
  onSaved: () => Promise<void>
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const mutation = useMutation({
    mutationFn: async (enabled: boolean) => {
      const result = await setQuotaPoolEnabled(props.pool.id, enabled)
      if (!result.success) {
        throw new Error(result.message || t('Operation failed'))
      }
    },
    onSuccess: async () => {
      await props.onSaved()
      setOpen(false)
      toast.success(t('Updated successfully'))
    },
    onError: handleServerError,
  })
  const label = props.pool.enabled ? t('Disable') : t('Enable')

  return (
    <AlertDialog
      open={open}
      onOpenChange={(nextOpen) => !mutation.isPending && setOpen(nextOpen)}
    >
      <AlertDialogTrigger render={<Button variant='outline' />}>
        {label}
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{label}</AlertDialogTitle>
          <AlertDialogDescription>
            {props.pool.enabled
              ? t('Disable quota pool {{pool}}?', { pool: props.pool.name })
              : t('Enable quota pool {{pool}}?', { pool: props.pool.name })}
            {props.pool.enabled && (
              <span className='mt-2 block'>
                {t(
                  'Members can still use their existing quota after this pool is disabled. Automatic recharge and quota allocation by pool administrators will stop.'
                )}
              </span>
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={mutation.isPending}>
            {t('Cancel')}
          </AlertDialogCancel>
          <AlertDialogAction
            variant={props.pool.enabled ? 'destructive' : 'default'}
            disabled={mutation.isPending}
            onClick={(event) => {
              event.preventDefault()
              mutation.mutate(!props.pool.enabled)
            }}
          >
            {mutation.isPending && (
              <Spinner data-icon='inline-start' aria-hidden='true' />
            )}
            {t('Confirm')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
