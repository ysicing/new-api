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
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
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
        handleConfirm={() => mutation.mutate(!props.pool.enabled)}
        confirmText={
          <>
            {mutation.isPending && (
              <Spinner data-icon='inline-start' aria-hidden='true' />
            )}
            {t('Confirm')}
          </>
        }
      />
    </>
  )
}
