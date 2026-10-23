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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'

import {
  addQuotaPoolMember,
  createQuotaPool,
  deductQuotaPool,
  refillQuotaPool,
} from '../api'
import type { QuotaPoolMember } from '../types'
import { QuotaPoolCandidatePicker } from './quota-pool-candidate-picker'

export function CreateQuotaPoolDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => Promise<void>
}) {
  const { t } = useTranslation()
  const [name, setName] = useState('')
  const [amount, setAmount] = useState('')
  const [saving, setSaving] = useState(false)
  const save = async () => {
    const baseQuota = Number(amount)
    if (!name.trim() || !Number.isFinite(baseQuota) || baseQuota <= 0) {
      toast.error(t('Enter a valid pool name and quota.'))
      return
    }
    setSaving(true)
    try {
      const result = await createQuotaPool({
        name: name.trim(),
        base_quota: baseQuota,
      })
      if (!result.success) {
        return toast.error(result.message || t('Create failed'))
      }
      toast.success(t('Quota pool created'))
      props.onOpenChange(false)
      setName('')
      setAmount('')
      await props.onSaved()
    } finally {
      setSaving(false)
    }
  }
  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}

      title={t('Create quota pool')}
      description={t('Create an isolated quota source for a team.')}
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button disabled={saving} onClick={() => void save()}>
            {saving && <Spinner data-icon='inline-start' />}
            {t('Create')}
          </Button>
        </>
      }
    >
      <FieldGroup>
        <Field>
          <FieldLabel htmlFor='pool-name'>{t('Name')}</FieldLabel>
          <Input
            id='pool-name'
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor='pool-quota'>{t('Base quota')}</FieldLabel>
          <Input
            id='pool-quota'
            inputMode='decimal'
            value={amount}
            onChange={(event) => setAmount(event.target.value)}
          />
        </Field>
      </FieldGroup>
    </Dialog>
  )
}

export function RefillQuotaPoolDialog(props: {
  poolId?: number
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => Promise<void>
}) {
  const { t } = useTranslation()
  const [amount, setAmount] = useState('')
  const [saving, setSaving] = useState(false)
  const save = async () => {
    const value = Number(amount)
    if (!props.poolId || !Number.isFinite(value) || value <= 0) {
      toast.error(t('Enter a valid refill amount.'))
      return
    }
    setSaving(true)
    try {
      const result = await refillQuotaPool(props.poolId, value)
      if (!result.success) {
        return toast.error(result.message || t('Refill failed'))
      }
      toast.success(t('Quota pool refilled'))
      props.onOpenChange(false)
      setAmount('')
      await props.onSaved()
    } finally {
      setSaving(false)
    }
  }
  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}

      title={t('Refill quota pool')}
      description={t('Add temporary quota to the selected pool.')}
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button disabled={saving} onClick={() => void save()}>
            {saving && <Spinner data-icon='inline-start' />}
            {t('Refill')}
          </Button>
        </>
      }
    >
      <FieldGroup>
        <Field>
          <FieldLabel htmlFor='refill-amount'>{t('Amount')}</FieldLabel>
          <Input
            id='refill-amount'
            inputMode='decimal'
            value={amount}
            onChange={(event) => setAmount(event.target.value)}
          />
        </Field>
      </FieldGroup>
    </Dialog>
  )
}

export function DeductQuotaPoolDialog(props: {
  poolId?: number
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => Promise<void>
}) {
  const { t } = useTranslation()
  const [amount, setAmount] = useState('')
  const [saving, setSaving] = useState(false)
  const save = async () => {
    const value = Number(amount)
    if (!props.poolId || !Number.isFinite(value) || value <= 0) {
      toast.error(t('Enter a valid deduction amount.'))
      return
    }
    setSaving(true)
    try {
      const result = await deductQuotaPool(props.poolId, value)
      if (!result.success) {
        return toast.error(result.message || t('Deduction failed'))
      }
      toast.success(t('Quota deducted'))
      props.onOpenChange(false)
      setAmount('')
      await props.onSaved()
    } finally {
      setSaving(false)
    }
  }
  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}

      title={t('Deduct quota pool')}
      description={t(
        'Deduct available quota from the selected pool without changing its base quota.'
      )}
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button disabled={saving} onClick={() => void save()}>
            {saving && <Spinner data-icon='inline-start' />}
            {t('Deduct')}
          </Button>
        </>
      }
    >
      <FieldGroup>
        <Field>
          <FieldLabel htmlFor='deduct-amount'>{t('Amount')}</FieldLabel>
          <Input
            id='deduct-amount'
            inputMode='decimal'
            value={amount}
            onChange={(event) => setAmount(event.target.value)}
          />
        </Field>
      </FieldGroup>
    </Dialog>
  )
}

export function AddQuotaPoolMemberDialog(props: {
  poolId?: number
  self?: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
  onSaved: () => Promise<void>
}) {
  const { t } = useTranslation()
  const [selectedUser, setSelectedUser] = useState<QuotaPoolMember | null>(null)
  const [saving, setSaving] = useState(false)
  const save = async () => {
    if (!props.poolId || !selectedUser) {
      toast.error(t('Select an eligible user.'))
      return
    }
    setSaving(true)
    try {
      const result = await addQuotaPoolMember(
        props.poolId,
        selectedUser.id,
        props.self
      )
      if (!result.success) {
        toast.error(result.message || t('Operation failed'))
        return
      }
      toast.success(t('Member added'))
      props.onOpenChange(false)
      setSelectedUser(null)
      await props.onSaved()
    } finally {
      setSaving(false)
    }
  }
  return (
    <Dialog
      open={props.open}
      onOpenChange={(open) => {
        if (!open) setSelectedUser(null)
        props.onOpenChange(open)
      }}

      title={t('Add member')}
      description={t('Move an eligible user into this quota pool.')}
      footer={
        <>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button disabled={saving} onClick={() => void save()}>
            {saving && <Spinner data-icon='inline-start' />}
            {t('Add member')}
          </Button>
        </>
      }
    >
      <FieldGroup>
        <Field>
          <FieldLabel htmlFor='member-user-id'>{t('User')}</FieldLabel>
          <QuotaPoolCandidatePicker
            open={props.open}
            self={props.self}
            value={selectedUser}
            onValueChange={setSelectedUser}
          />
        </Field>
      </FieldGroup>
    </Dialog>
  )
}
