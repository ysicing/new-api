/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  Combobox,
  ComboboxCollection,
  ComboboxContent,
  ComboboxEmpty,
  ComboboxInput,
  ComboboxItem,
  ComboboxList,
} from '@/components/ui/combobox'
import { useDebounce } from '@/hooks/use-debounce'

import { searchDingTalkTestUsers, type DingTalkTestUser } from './dingtalk-api'

function formatUserLabel(user: DingTalkTestUser) {
  const details = [user.display_name, user.email, user.department].filter(
    Boolean
  )
  const suffix = details.length > 0 ? ` — ${details.join(' / ')}` : ''
  return `${user.username} (ID:${user.id})${suffix}`
}

export function DingTalkTestRecipientPicker(props: {
  value: DingTalkTestUser | null
  onValueChange: (user: DingTalkTestUser | null) => void
  disabled?: boolean
}) {
  const { t } = useTranslation()
  const [keyword, setKeyword] = useState('')
  const debouncedKeyword = useDebounce(keyword, 250)
  const query = useQuery({
    queryKey: ['dingtalk-test-users', debouncedKeyword],
    queryFn: () => searchDingTalkTestUsers(debouncedKeyword.trim()),
  })
  const candidates = query.data?.data?.items ?? []
  const selectedMissing =
    props.value &&
    !candidates.some((candidate) => candidate.id === props.value?.id)
  const items = selectedMissing ? [...candidates, props.value] : candidates
  const failed = query.isError || query.data?.success === false
  let status: string | null = null
  if (query.isFetching) status = t('Searching users...')
  else if (failed) status = t('Failed to load users.')

  return (
    <Combobox
      items={items}
      value={props.value}
      filter={null}
      itemToStringLabel={formatUserLabel}
      isItemEqualToValue={(candidate, selected) => candidate.id === selected.id}
      onValueChange={props.onValueChange}
      onInputValueChange={(value, details) => {
        if (details.reason === 'item-press') return
        setKeyword(value)
        if (details.reason === 'input-change' && props.value) {
          props.onValueChange(null)
        }
      }}
    >
      <ComboboxInput
        id='dingtalk-test-recipient'
        className='w-full'
        placeholder={t(
          'Search user ID, username, display name, email or department'
        )}
        showTrigger={false}
        disabled={props.disabled}
      />
      <ComboboxContent aria-busy={query.isFetching || undefined}>
        <div
          role='status'
          aria-live='polite'
          className={
            status ? 'text-muted-foreground px-2 py-1.5 text-sm' : 'sr-only'
          }
        >
          {status}
        </div>
        <ComboboxList>
          <ComboboxCollection>
            {(candidate: DingTalkTestUser) => (
              <ComboboxItem key={candidate.id} value={candidate}>
                <span className='min-w-0 flex-1'>
                  <span className='flex items-center gap-2'>
                    <span className='truncate font-medium'>
                      {candidate.username} (ID:{candidate.id})
                    </span>
                    <Badge
                      variant={
                        candidate.dingtalk_bound ? 'secondary' : 'outline'
                      }
                    >
                      {candidate.dingtalk_bound
                        ? t('DingTalk bound')
                        : t('DingTalk not bound')}
                    </Badge>
                  </span>
                  <span className='text-muted-foreground block truncate text-xs'>
                    {[
                      candidate.display_name,
                      candidate.email,
                      candidate.department,
                    ]
                      .filter(Boolean)
                      .join(' / ')}
                  </span>
                </span>
              </ComboboxItem>
            )}
          </ComboboxCollection>
        </ComboboxList>
        <ComboboxEmpty>{t('No eligible users found.')}</ComboboxEmpty>
      </ComboboxContent>
    </Combobox>
  )
}
