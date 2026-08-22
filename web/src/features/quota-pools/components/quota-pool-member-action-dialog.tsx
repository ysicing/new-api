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
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { formatQuota } from '@/lib/format'

export function QuotaPoolMemberActionDialog(props: {
  action: 'recharge' | 'reclaim'
  memberName: string
  reclaimAmounts: number[]
  onOpenChange: (open: boolean) => void
  onConfirm: (amount?: number) => Promise<boolean>
}) {
  const { t } = useTranslation()
  const [selectedAmount, setSelectedAmount] = useState(props.reclaimAmounts[0])
  const [processing, setProcessing] = useState(false)
  const reclaim = props.action === 'reclaim'
  const confirm = async () => {
    setProcessing(true)
    try {
      if (await props.onConfirm(reclaim ? selectedAmount : undefined)) {
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
              : t('Confirm recharge for {{user}}?', {
                  user: props.memberName,
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
        ) : null}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={processing}>
            {t('Cancel')}
          </AlertDialogCancel>
          <AlertDialogAction
            variant={reclaim ? 'destructive' : 'default'}
            disabled={processing || (reclaim && selectedAmount === undefined)}
            onClick={() => void confirm()}
          >
            {processing ? t('Processing...') : t('Confirm')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
