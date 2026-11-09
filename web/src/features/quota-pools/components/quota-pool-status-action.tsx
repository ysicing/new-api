import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import { handleServerError } from '@/lib/handle-server-error'
import { createServerError } from '@/lib/server-error-message'

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
        throw createServerError(result, t('Operation failed'))
      }
    },
    onSuccess: async () => {
      await props.onSaved()
      setOpen(false)
      toast.success(t('Updated successfully'))
    },
    onError: (error) => handleServerError(error),
  })
  const label = props.pool.enabled ? t('Disable') : t('Enable')

  return (
    <>
      <Button variant='outline' onClick={() => setOpen(true)}>
        {label}
      </Button>
      <ConfirmDialog
        open={open}
        onOpenChange={(nextOpen) => !mutation.isPending && setOpen(nextOpen)}
        title={label}
        desc={
          <>
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
          </>
        }
        destructive={props.pool.enabled}
        isLoading={mutation.isPending}
        confirmText={
          <>
            {mutation.isPending && (
              <Spinner data-icon='inline-start' aria-hidden='true' />
            )}
            {t('Confirm')}
          </>
        }
        handleConfirm={() => mutation.mutate(!props.pool.enabled)}
      />
    </>
  )
}
