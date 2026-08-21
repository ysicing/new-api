import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'

import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

export interface AutoRechargeSettingsValues {
  QuotaPoolEnabled: boolean
  'auto_recharge_setting.enabled': boolean
  'auto_recharge_setting.interval': number
  'auto_recharge_setting.threshold': number
  'auto_recharge_setting.amount': number
  'auto_recharge_setting.weekly_limit': number
  'auto_recharge_setting.monthly_limit': number
}

export function AutoRechargeSection({
  defaultValues,
}: {
  defaultValues: AutoRechargeSettingsValues
}) {
  const { t } = useTranslation()
  const update = useUpdateOption()
  const [values, setValues] = useState(defaultValues)
  const set = <K extends keyof AutoRechargeSettingsValues>(
    key: K,
    value: AutoRechargeSettingsValues[K]
  ) => setValues((current) => ({ ...current, [key]: value }))
  const save = async () => {
    for (const [key, value] of Object.entries(values)) {
      if (value !== defaultValues[key as keyof AutoRechargeSettingsValues]) {
        await update.mutateAsync({ key, value })
      }
    }
  }
  const numberField = (
    key: keyof AutoRechargeSettingsValues,
    label: string,
    min = 0
  ) => (
    <Field>
      <FieldLabel htmlFor={key}>{t(label)}</FieldLabel>
      <Input
        id={key}
        type='number'
        min={min}
        value={Number(values[key])}
        onChange={(event) => set(key, Number(event.target.value) as never)}
      />
    </Field>
  )
  return (
    <SettingsSection title={t('Quota pools and automatic recharge')}>
      <FieldGroup>
        <Field orientation='horizontal'>
          <div className='flex-1'>
            <FieldLabel htmlFor='quota-pool-enabled'>
              {t('Enable quota pools')}
            </FieldLabel>
          </div>
          <Switch
            id='quota-pool-enabled'
            checked={values.QuotaPoolEnabled}
            disabled={defaultValues.QuotaPoolEnabled}
            onCheckedChange={(value) => set('QuotaPoolEnabled', value)}
          />
        </Field>
        {defaultValues.QuotaPoolEnabled && (
          <Alert>
            <AlertTitle>{t('Permanent setting')}</AlertTitle>
            <AlertDescription>
              {t('Quota pools cannot be disabled after activation.')}
            </AlertDescription>
          </Alert>
        )}
        <Field orientation='horizontal'>
          <div className='flex-1'>
            <FieldLabel htmlFor='auto-recharge-enabled'>
              {t('Automatic recharge')}
            </FieldLabel>
          </div>
          <Switch
            id='auto-recharge-enabled'
            checked={values['auto_recharge_setting.enabled']}
            onCheckedChange={(value) =>
              set('auto_recharge_setting.enabled', value)
            }
          />
        </Field>
        <div className='grid gap-4 sm:grid-cols-2'>
          {numberField(
            'auto_recharge_setting.interval',
            'Interval in minutes',
            1
          )}
          {numberField('auto_recharge_setting.threshold', 'Recharge threshold')}
          {numberField('auto_recharge_setting.amount', 'Recharge amount', 1)}
          {numberField('auto_recharge_setting.weekly_limit', 'Weekly limit')}
          {numberField('auto_recharge_setting.monthly_limit', 'Monthly limit')}
        </div>
        <Button
          className='w-fit'
          disabled={update.isPending}
          onClick={() => void save()}
        >
          {update.isPending && <Spinner data-icon='inline-start' />}
          {t('Save')}
        </Button>
      </FieldGroup>
    </SettingsSection>
  )
}
