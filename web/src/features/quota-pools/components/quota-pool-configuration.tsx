import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { formatQuota } from '@/lib/format'

import { updateQuotaPool } from '../api'
import type { QuotaPool, QuotaPoolCapabilities } from '../types'

export function PoolConfiguration(props: {
  pool: QuotaPool
  capabilities: QuotaPoolCapabilities
  selfMode?: boolean
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [weeklyLimit, setWeeklyLimit] = useState(props.pool.weekly_limit)
  const [monthlyLimit, setMonthlyLimit] = useState(props.pool.monthly_limit)
  const [saving, setSaving] = useState(false)
  const save = async () => {
    setSaving(true)
    try {
      const result = await updateQuotaPool(
        props.pool.id,
        { weekly_limit: weeklyLimit, monthly_limit: monthlyLimit },
        props.selfMode
      )
      if (!result.success) {
        toast.error(result.message || t('Operation failed'))
        return
      }
      toast.success(t('Configuration saved'))
      await queryClient.invalidateQueries({ queryKey: ['quota-pools'] })
    } finally {
      setSaving(false)
    }
  }
  return (
    <div className='flex flex-col gap-4 py-4'>
      <dl className='grid gap-3 text-sm sm:grid-cols-2'>
        <div>
          <dt className='text-muted-foreground'>{t('Automatic recharge')}</dt>
          <dd>
            {props.pool.auto_recharge_amount < 0
              ? t('Inherit system setting')
              : formatQuota(props.pool.auto_recharge_amount)}
          </dd>
        </div>
        <div>
          <dt className='text-muted-foreground'>{t('Monthly refill day')}</dt>
          <dd>{props.pool.monthly_refill_day}</dd>
        </div>
      </dl>
      {props.capabilities.can_edit && (
        <FieldGroup>
          <div className='grid gap-4 sm:grid-cols-2'>
            <Field>
              <FieldLabel htmlFor='pool-weekly-limit'>
                {t('Weekly limit')}
              </FieldLabel>
              <Input
                id='pool-weekly-limit'
                type='number'
                min={-1}
                value={weeklyLimit}
                onChange={(event) => setWeeklyLimit(Number(event.target.value))}
              />
            </Field>
            <Field>
              <FieldLabel htmlFor='pool-monthly-limit'>
                {t('Monthly limit')}
              </FieldLabel>
              <Input
                id='pool-monthly-limit'
                type='number'
                min={-1}
                value={monthlyLimit}
                onChange={(event) =>
                  setMonthlyLimit(Number(event.target.value))
                }
              />
            </Field>
          </div>
          <Button
            className='w-fit'
            disabled={saving}
            onClick={() => void save()}
          >
            {saving && <Spinner data-icon='inline-start' />}
            {t('Save')}
          </Button>
        </FieldGroup>
      )}
    </div>
  )
}
