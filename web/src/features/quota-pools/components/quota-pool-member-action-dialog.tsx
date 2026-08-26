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
import { Field, FieldDescription, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { formatQuota } from '@/lib/format'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

export function QuotaPoolMemberActionDialog(props: {
  action: 'recharge' | 'reclaim'
  memberName: string
  reclaimAmounts: number[]
  defaultRechargeAmount: number
  maximumRechargeAmount: number
  onOpenChange: (open: boolean) => void
  onConfirm: (amount?: number) => Promise<boolean>
}) {
  const { t } = useTranslation()
  const configuredQuotaPerUnit = useSystemConfigStore(
    (state) => state.config.currency.quotaPerUnit
  )
  const quotaPerUnit =
    configuredQuotaPerUnit > 0
      ? configuredQuotaPerUnit
      : DEFAULT_CURRENCY_CONFIG.quotaPerUnit
  const minimumRechargeAmount = Math.round(10 * quotaPerUnit)
  const defaultRechargeAvailable =
    props.defaultRechargeAmount >= minimumRechargeAmount &&
    props.defaultRechargeAmount <= props.maximumRechargeAmount
  const [selectedAmount, setSelectedAmount] = useState(props.reclaimAmounts[0])
  const [rechargeMode, setRechargeMode] = useState<'default' | 'custom'>(
    defaultRechargeAvailable ? 'default' : 'custom'
  )
  const [customAmount, setCustomAmount] = useState(10)
  const [processing, setProcessing] = useState(false)
  const reclaim = props.action === 'reclaim'
  const customRechargeAmount = Math.round(customAmount * quotaPerUnit)
  const rechargeAmount =
    rechargeMode === 'default'
      ? props.defaultRechargeAmount
      : customRechargeAmount
  const validRechargeAmount =
    rechargeAmount >= minimumRechargeAmount &&
    rechargeAmount <= props.maximumRechargeAmount
  const confirm = async () => {
    setProcessing(true)
    try {
      if (await props.onConfirm(reclaim ? selectedAmount : rechargeAmount)) {
        props.onOpenChange(false)
      }
    } finally {
      setProcessing(false)
    }
  }

  return (
    <AlertDialog
      open
      onOpenChange={(open) => !processing && props.onOpenChange(open)}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>
            {t(reclaim ? 'Reclaim' : 'Recharge')}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {reclaim
              ? t('Select the amount to reclaim from {{user}}.', {
                  user: props.memberName,
                })
              : t('Confirm recharge {{amount}} for {{user}}?', {
                  user: props.memberName,
                  amount: formatQuota(rechargeAmount),
                })}
          </AlertDialogDescription>
        </AlertDialogHeader>
        {reclaim ? (
          <RadioGroup
            value={selectedAmount === undefined ? '' : String(selectedAmount)}
            onValueChange={(value) => setSelectedAmount(Number(value))}
          >
            {props.reclaimAmounts.map((amount) => {
              const id = `quota-pool-reclaim-${amount}`
              return (
                <div key={amount} className='flex items-center gap-2'>
                  <RadioGroupItem id={id} value={String(amount)} />
                  <Label htmlFor={id} className='cursor-pointer'>
                    {formatQuota(amount)}
                  </Label>
                </div>
              )
            })}
          </RadioGroup>
        ) : (
          <div className='space-y-4'>
            <RadioGroup
              value={rechargeMode}
              onValueChange={(value) =>
                setRechargeMode(value as 'default' | 'custom')
              }
            >
              <div className='flex items-center gap-2'>
                <RadioGroupItem
                  id='quota-pool-recharge-default'
                  value='default'
                  disabled={!defaultRechargeAvailable}
                />
                <Label
                  htmlFor='quota-pool-recharge-default'
                  className='cursor-pointer'
                >
                  {t('Default')} · {formatQuota(props.defaultRechargeAmount)}
                </Label>
              </div>
              <div className='flex items-center gap-2'>
                <RadioGroupItem
                  id='quota-pool-recharge-custom'
                  value='custom'
                />
                <Label
                  htmlFor='quota-pool-recharge-custom'
                  className='cursor-pointer'
                >
                  {t('Custom')}
                </Label>
              </div>
            </RadioGroup>
            {rechargeMode === 'custom' ? (
              <Field>
                <FieldLabel htmlFor='quota-pool-recharge-amount'>
                  {t('Recharge amount')}
                </FieldLabel>
                <Input
                  id='quota-pool-recharge-amount'
                  type='number'
                  step='any'
                  min={10}
                  max={props.maximumRechargeAmount / quotaPerUnit}
                  value={customAmount}
                  onChange={(event) =>
                    setCustomAmount(Number(event.target.value))
                  }
                />
                <FieldDescription>
                  {t('Enter an amount from {{min}} to {{max}}.', {
                    min: formatQuota(minimumRechargeAmount),
                    max: formatQuota(props.maximumRechargeAmount),
                  })}
                </FieldDescription>
              </Field>
            ) : null}
          </div>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={processing}>
            {t('Cancel')}
          </AlertDialogCancel>
          <AlertDialogAction
            variant={reclaim ? 'destructive' : 'default'}
            disabled={
              processing ||
              (reclaim ? selectedAmount === undefined : !validRechargeAmount)
            }
            onClick={() => void confirm()}
          >
            {processing ? t('Processing...') : t('Confirm')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
