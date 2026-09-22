import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

export function QuotaPoolTransactionFilters(props: {
  type: string
  keyword: string
  onTypeChange: (type: string) => void
  onSearch: (keyword: string) => void
}) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState(props.keyword)
  const types = [
    ['initial_fund', t('Initial funding')],
    ['manual_refill', t('Temporary refill')],
    ['manual_deduct', t('Manual deduction')],
    ['monthly_refill', t('Monthly automatic refill')],
    ['allocate_auto', t('Automatic allocation')],
    ['allocate_manual', t('Manual allocation')],
    ['reclaim_user', t('Reclaimed user quota')],
    ['adjust_base_quota', t('Base quota adjustment')],
  ]
  return (
    <form
      className='mb-3 flex flex-wrap items-center gap-2'
      onSubmit={(event) => {
        event.preventDefault()
        props.onSearch(draft.trim())
      }}
    >
      <NativeSelect
        aria-label={t('Type')}
        value={props.type}
        onChange={(event) => props.onTypeChange(event.target.value)}
      >
        <NativeSelectOption value=''>{t('All')}</NativeSelectOption>
        {types.map(([value, label]) => (
          <NativeSelectOption key={value} value={value}>
            {label}
          </NativeSelectOption>
        ))}
      </NativeSelect>
      <Input
        className='min-w-48 flex-1'
        aria-label={t(
          'Search user or operator by ID, username, or display name'
        )}
        placeholder={t(
          'Search user or operator by ID, username, or display name'
        )}
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
      />
      <Button type='submit' variant='outline'>
        {t('Search')}
      </Button>
    </form>
  )
}

export function QuotaPoolOperationFilter(props: {
  action: string
  onChange: (action: string) => void
}) {
  const { t } = useTranslation()
  const actions = [
    ['quota_pool.create', t('Create pool')],
    ['quota_pool.update', t('Configuration')],
    ['quota_pool.self_update', t('Automatic recharge configuration')],
    ['quota_pool.enabled', t('Status')],
    ['quota_pool.delete', t('Delete')],
    ['quota_pool.refill', t('Refill')],
    ['quota_pool.deduct', t('Deduct')],
    ['quota_pool.member_add', t('Add member')],
    ['quota_pool.member_move', t('Move')],
    ['quota_pool.member_remove', t('Remove member')],
    ['quota_pool.member_recharge', t('Recharge')],
    ['quota_pool.member_reclaim', t('Reclaim')],
    ['quota_pool.admin_grant', t('Set pool administrator')],
    ['quota_pool.admin_revoke', t('Remove pool administrator')],
  ]
  return (
    <div className='mb-3'>
      <NativeSelect
        aria-label={t('Operation')}
        value={props.action}
        onChange={(event) => props.onChange(event.target.value)}
      >
        <NativeSelectOption value=''>{t('All')}</NativeSelectOption>
        {actions.map(([value, label]) => (
          <NativeSelectOption key={value} value={value}>
            {label}
          </NativeSelectOption>
        ))}
      </NativeSelect>
    </div>
  )
}
