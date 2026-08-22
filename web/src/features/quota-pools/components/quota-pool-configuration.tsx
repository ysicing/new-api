import { useQueryClient } from '@tanstack/react-query'
import { type ReactNode, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'
import { formatQuota } from '@/lib/format'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { updateQuotaPool } from '../api'
import type { QuotaPool, QuotaPoolCapabilities } from '../types'

function QuotaPoolNumberField(props: {
  id: string
  label: string
  value: number
  min: number
  max?: number
  disabled?: boolean
  description?: ReactNode
  onChange: (value: number) => void
}) {
  return (
    <Field>
      <FieldLabel htmlFor={props.id}>{props.label}</FieldLabel>
      <Input
        id={props.id}
        type='number'
        step='any'
        min={props.min}
        max={props.max}
        disabled={props.disabled}
        value={props.value}
        onChange={(event) => props.onChange(Number(event.target.value))}
      />
      {props.description ? (
        <FieldDescription>{props.description}</FieldDescription>
      ) : null}
    </Field>
  )
}

export function PoolConfiguration(props: {
  pool: QuotaPool
  capabilities: QuotaPoolCapabilities
  selfMode?: boolean
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const quotaPerUnit = useSystemConfigStore(
    (state) => state.config.currency.quotaPerUnit
  )
  const toAmount = (quota: number) =>
    quota > 0 && quotaPerUnit > 0 ? quota / quotaPerUnit : quota
  const [autoRechargeAmount, setAutoRechargeAmount] = useState(
    toAmount(props.pool.auto_recharge_amount)
  )
  const [weeklyLimit, setWeeklyLimit] = useState(props.pool.weekly_limit)
  const [monthlyLimit, setMonthlyLimit] = useState(props.pool.monthly_limit)
  const [monthlyRefillEnabled, setMonthlyRefillEnabled] = useState(
    props.pool.monthly_refill_enabled
  )
  const [monthlyRefillTopUp, setMonthlyRefillTopUp] = useState(
    props.pool.monthly_refill_top_up
  )
  const [monthlyRefillAmount, setMonthlyRefillAmount] = useState(
    toAmount(props.pool.monthly_refill_amount)
  )
  const [monthlyRefillDay, setMonthlyRefillDay] = useState(
    props.pool.monthly_refill_day
  )
  const [saving, setSaving] = useState(false)
  const canEditPolicy = props.capabilities.can_edit
  const canEditMonthlyRefill =
    props.capabilities.can_edit_monthly_refill && !props.selfMode
  const save = async () => {
    setSaving(true)
    try {
      const values: Record<string, number | boolean> = {}
      if (canEditPolicy) {
        values.auto_recharge_amount = autoRechargeAmount
        values.weekly_limit = weeklyLimit
        values.monthly_limit = monthlyLimit
      }
      if (canEditMonthlyRefill) {
        values.monthly_refill_enabled = monthlyRefillEnabled
        values.monthly_refill_top_up = monthlyRefillTopUp
        values.monthly_refill_amount = monthlyRefillAmount
        values.monthly_refill_day = monthlyRefillDay
      }
      const result = await updateQuotaPool(
        props.pool.id,
        values,
        props.selfMode
      )
      if (!result.success) {
        toast.error(result.message || t('Operation failed'))
        return
      }
      if (result.message) {
        toast.warning(result.message)
      } else {
        toast.success(t('Configuration saved'))
      }
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['quota-pools'] }),
        queryClient.invalidateQueries({
          queryKey: ['quota-pool', props.pool.id],
        }),
      ])
    } finally {
      setSaving(false)
    }
  }
  const system = props.pool.system_auto_recharge
  let autoRechargeValue = formatQuota(props.pool.auto_recharge_amount)
  if (props.pool.auto_recharge_amount < 0) {
    autoRechargeValue = t('Inherit system setting')
  } else if (props.pool.auto_recharge_amount === 0) {
    autoRechargeValue = t('Disabled')
  }
  return (
    <div className='flex flex-col gap-4 py-4'>
      {system ? (
        <Alert>
          <AlertTitle>{t('System settings')}</AlertTitle>
          <AlertDescription>
            <dl className='grid gap-2 sm:grid-cols-3'>
              <div>
                <dt>{t('Automatic recharge')}</dt>
                <dd>{t(system.enabled ? 'Enabled' : 'Disabled')}</dd>
              </div>
              <div>
                <dt>{t('Recharge threshold')}</dt>
                <dd>{formatQuota(system.threshold)}</dd>
              </div>
              <div>
                <dt>{t('Recharge amount')}</dt>
                <dd>{formatQuota(system.amount)}</dd>
              </div>
              <div>
                <dt>{t('Weekly limit')}</dt>
                <dd>{system.weekly_limit || t('Unlimited')}</dd>
              </div>
              <div>
                <dt>{t('Monthly limit')}</dt>
                <dd>{system.monthly_limit || t('Unlimited')}</dd>
              </div>
              <div>
                <dt>{t('Interval in minutes')}</dt>
                <dd>{system.interval}</dd>
              </div>
            </dl>
          </AlertDescription>
        </Alert>
      ) : null}
      {canEditPolicy || canEditMonthlyRefill ? (
        <FieldGroup>
          {canEditPolicy ? (
            <div className='grid gap-4 sm:grid-cols-2'>
              <QuotaPoolNumberField
                id='pool-auto-recharge-amount'
                label={t('Recharge amount')}
                value={autoRechargeAmount}
                min={-1}
                onChange={setAutoRechargeAmount}
                description={t(
                  '-1 inherits the system setting; 0 disables automatic recharge.'
                )}
              />
              <QuotaPoolNumberField
                id='pool-weekly-limit'
                label={t('Weekly limit')}
                value={weeklyLimit}
                min={-1}
                onChange={setWeeklyLimit}
                description={t(
                  '-1 inherits the system setting; 0 means unlimited.'
                )}
              />
              <QuotaPoolNumberField
                id='pool-monthly-limit'
                label={t('Monthly limit')}
                value={monthlyLimit}
                min={-1}
                onChange={setMonthlyLimit}
                description={t(
                  '-1 inherits the system setting; 0 means unlimited.'
                )}
              />
            </div>
          ) : null}
          {canEditMonthlyRefill ? (
            <div className='flex flex-col gap-4 rounded-lg border p-4'>
              <Field orientation='horizontal'>
                <FieldLabel htmlFor='pool-monthly-refill-enabled'>
                  {t('Monthly refill')}
                </FieldLabel>
                <Switch
                  id='pool-monthly-refill-enabled'
                  checked={monthlyRefillEnabled}
                  onCheckedChange={setMonthlyRefillEnabled}
                />
              </Field>
              <Field orientation='horizontal'>
                <FieldLabel htmlFor='pool-monthly-refill-top-up'>
                  {t('Top up to target quota')}
                </FieldLabel>
                <Switch
                  id='pool-monthly-refill-top-up'
                  checked={monthlyRefillTopUp}
                  disabled={!monthlyRefillEnabled}
                  onCheckedChange={setMonthlyRefillTopUp}
                />
                <FieldDescription>
                  {t(
                    'When disabled, add a fixed amount each month; when enabled, top up to the target quota.'
                  )}
                </FieldDescription>
              </Field>
              <div className='grid gap-4 sm:grid-cols-2'>
                <QuotaPoolNumberField
                  id='pool-monthly-refill-amount'
                  label={t('Monthly refill amount')}
                  value={monthlyRefillAmount}
                  min={0}
                  disabled={!monthlyRefillEnabled}
                  onChange={setMonthlyRefillAmount}
                />
                <QuotaPoolNumberField
                  id='pool-monthly-refill-day'
                  label={t('Monthly refill day')}
                  value={monthlyRefillDay}
                  min={1}
                  max={28}
                  disabled={!monthlyRefillEnabled}
                  onChange={setMonthlyRefillDay}
                />
              </div>
            </div>
          ) : null}
          <Button
            className='w-fit'
            disabled={saving}
            onClick={() => void save()}
          >
            {saving && <Spinner data-icon='inline-start' />}
            {t('Save')}
          </Button>
        </FieldGroup>
      ) : (
        <dl className='grid gap-3 text-sm sm:grid-cols-2'>
          <div>
            <dt className='text-muted-foreground'>{t('Automatic recharge')}</dt>
            <dd>{autoRechargeValue}</dd>
          </div>
          <div>
            <dt className='text-muted-foreground'>{t('Monthly refill day')}</dt>
            <dd>{props.pool.monthly_refill_day}</dd>
          </div>
        </dl>
      )}
    </div>
  )
}
