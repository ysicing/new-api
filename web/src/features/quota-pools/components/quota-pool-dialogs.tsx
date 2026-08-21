import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Field, FieldGroup, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'

import { addQuotaPoolMember, createQuotaPool, refillQuotaPool } from '../api'

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
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('Create quota pool')}</DialogTitle>
          <DialogDescription>
            {t('Create an isolated quota source for a team.')}
          </DialogDescription>
        </DialogHeader>
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
        <DialogFooter>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button disabled={saving} onClick={() => void save()}>
            {saving && <Spinner data-icon='inline-start' />}
            {t('Create')}
          </Button>
        </DialogFooter>
      </DialogContent>
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
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('Refill quota pool')}</DialogTitle>
          <DialogDescription>
            {t('Add temporary quota to the selected pool.')}
          </DialogDescription>
        </DialogHeader>
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
        <DialogFooter>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button disabled={saving} onClick={() => void save()}>
            {saving && <Spinner data-icon='inline-start' />}
            {t('Refill')}
          </Button>
        </DialogFooter>
      </DialogContent>
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
  const [userId, setUserId] = useState('')
  const [saving, setSaving] = useState(false)
  const save = async () => {
    const id = Number(userId)
    if (!props.poolId || !Number.isInteger(id) || id <= 0) {
      toast.error(t('Enter a valid user ID.'))
      return
    }
    setSaving(true)
    try {
      const result = await addQuotaPoolMember(props.poolId, id, props.self)
      if (!result.success) {
        toast.error(result.message || t('Operation failed'))
        return
      }
      toast.success(t('Member added'))
      props.onOpenChange(false)
      setUserId('')
      await props.onSaved()
    } finally {
      setSaving(false)
    }
  }
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('Add member')}</DialogTitle>
          <DialogDescription>
            {t('Move an eligible user into this quota pool.')}
          </DialogDescription>
        </DialogHeader>
        <FieldGroup>
          <Field>
            <FieldLabel htmlFor='member-user-id'>{t('User ID')}</FieldLabel>
            <Input
              id='member-user-id'
              inputMode='numeric'
              value={userId}
              onChange={(event) => setUserId(event.target.value)}
            />
          </Field>
        </FieldGroup>
        <DialogFooter>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button disabled={saving} onClick={() => void save()}>
            {saving && <Spinner data-icon='inline-start' />}
            {t('Add member')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
