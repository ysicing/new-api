/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useQuery } from '@tanstack/react-query'
import { Check, ChevronsUpDown } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Spinner } from '@/components/ui/spinner'
import { useDebounce } from '@/hooks/use-debounce'
import { cn } from '@/lib/utils'

import { getQuotaPools } from '../api'
import type { QuotaPool } from '../types'

export function QuotaPoolSwitcher(props: {
  pool: QuotaPool
  onSelect: (pool: QuotaPool) => void
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const debouncedSearch = useDebounce(search, 250)
  const query = useQuery({
    queryKey: ['quota-pool-switcher', debouncedSearch],
    queryFn: () =>
      getQuotaPools({
        page: 1,
        pageSize: 20,
        keyword: debouncedSearch.trim(),
      }),
    enabled: open,
  })
  let options = query.data?.data?.items ?? []
  if (
    !debouncedSearch.trim() &&
    !options.some((option) => option.id === props.pool.id)
  ) {
    options = [props.pool, ...options]
  }
  const failed = query.isError || query.data?.success === false

  return (
    <Popover
      open={open}
      onOpenChange={(nextOpen) => {
        setOpen(nextOpen)
        if (!nextOpen) setSearch('')
      }}
    >
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='ghost'
            size='sm'
            aria-label={t('Switch quota pool: {{name}}', {
              name: props.pool.name,
            })}
            title={t('Switch quota pool')}
            aria-expanded={open}
            className='max-w-[min(18rem,55vw)] justify-start px-2 text-base'
          />
        }
      >
        <span className='truncate'>{props.pool.name}</span>
        <ChevronsUpDown aria-hidden='true' className='opacity-50' />
      </PopoverTrigger>
      <PopoverContent
        align='start'
        className='w-80 max-w-[calc(100vw-2rem)] p-0'
      >
        <Command shouldFilter={false}>
          <CommandInput
            placeholder={t('Search by pool ID or name')}
            value={search}
            onValueChange={setSearch}
          />
          <CommandList aria-busy={query.isFetching || undefined}>
            {query.isFetching ? (
              <div
                role='status'
                className='text-muted-foreground flex items-center gap-2 px-3 py-2 text-sm'
              >
                <Spinner />
                {t('Loading...')}
              </div>
            ) : null}
            {failed ? (
              <div role='alert' className='text-destructive px-3 py-2 text-sm'>
                {t('Failed to load quota pools.')}
              </div>
            ) : null}
            {!query.isFetching && !failed ? (
              <CommandEmpty>{t('No quota pools')}</CommandEmpty>
            ) : null}
            <CommandGroup heading={t('Quota pools')}>
              {options.map((option) => (
                <CommandItem
                  key={option.id}
                  value={String(option.id)}
                  onSelect={() => {
                    props.onSelect(option)
                    setOpen(false)
                    setSearch('')
                  }}
                >
                  <Check
                    aria-hidden='true'
                    className={cn(
                      'size-4',
                      option.id === props.pool.id ? 'opacity-100' : 'opacity-0'
                    )}
                  />
                  {option.id === props.pool.id ? (
                    <span className='sr-only'>{t('Current')}</span>
                  ) : null}
                  <span className='min-w-0 flex-1 truncate'>{option.name}</span>
                  <span className='text-muted-foreground text-xs'>
                    #{option.id}
                  </span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
