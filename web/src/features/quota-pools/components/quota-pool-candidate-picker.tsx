/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

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

import { getQuotaPoolCandidates } from '../api'
import type { QuotaPoolMember } from '../types'

function formatDepartment(department: string) {
  const parts = department
    .split(/[/\\>｜|,，;；-]+/)
    .map((part) => part.trim())
    .filter(Boolean)
  if (parts.length <= 1) return department
  return `${parts[0]} / ${parts.at(-1)}`
}

function formatCandidateLabel(candidate: QuotaPoolMember) {
  const details = [
    candidate.display_name,
    candidate.email,
    formatDepartment(candidate.department),
  ].filter(Boolean)
  const suffix = details.length > 0 ? ` — ${details.join(' / ')}` : ''
  return `${candidate.username} (ID:${candidate.id})${suffix}`
}

export function QuotaPoolCandidatePicker(props: {
  open: boolean
  self?: boolean
  value: QuotaPoolMember | null
  onValueChange: (candidate: QuotaPoolMember | null) => void
}) {
  const { t } = useTranslation()
  const [keyword, setKeyword] = useState('')
  const debouncedKeyword = useDebounce(keyword, 250)
  const query = useQuery({
    queryKey: [
      'quota-pool-candidates',
      props.self ? 'self' : 'all',
      debouncedKeyword,
    ],
    queryFn: () =>
      getQuotaPoolCandidates(Boolean(props.self), {
        page: 1,
        pageSize: 20,
        keyword: debouncedKeyword.trim(),
      }),
    enabled: props.open,
  })
  const candidates = query.data?.data?.items ?? []
  let items = candidates
  if (
    props.value &&
    !candidates.some((candidate) => candidate.id === props.value?.id)
  ) {
    items = [...candidates, props.value]
  }
  const failed = query.isError || query.data?.success === false
  let status: string | null = null
  if (query.isFetching) status = t('Searching users...')
  else if (failed) status = t('Failed to load users.')

  return (
    <Combobox
      items={items}
      value={props.value}
      filter={null}
      itemToStringLabel={formatCandidateLabel}
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
        id='member-user-id'
        className='w-full'
        placeholder={t(
          'Search user ID, username, display name, email or department'
        )}
        showTrigger={false}
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
            {(candidate: QuotaPoolMember) => (
              <ComboboxItem key={candidate.id} value={candidate}>
                <span className='min-w-0 flex-1'>
                  <span className='block truncate font-medium'>
                    {candidate.username}{' '}
                    <span className='text-muted-foreground font-normal'>
                      (ID:{candidate.id})
                    </span>
                  </span>
                  <span className='text-muted-foreground block truncate text-xs'>
                    {[
                      candidate.display_name,
                      candidate.email,
                      formatDepartment(candidate.department),
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
