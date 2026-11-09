/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useMutation } from '@tanstack/react-query'
import { RefreshCw, Search } from 'lucide-react'
import { type FormEvent, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import { Spinner } from '@/components/ui/spinner'
import { cn } from '@/lib/utils'

import { searchLDAPUsers, syncLDAPCandidate } from '../../api'
import type { LDAPSyncCandidate } from '../../types'
import { useUsers } from '../users-provider'

type LDAPSyncDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

function LDAPCandidateList(props: {
  candidates: LDAPSyncCandidate[]
  selectedKey: string
  searching: boolean
  hasSearched: boolean
  onSelect: (key: string) => void
}) {
  const { t } = useTranslation()
  if (props.searching) {
    return (
      <div className='space-y-2' aria-label={t('Searching LDAP users')}>
        {[1, 2, 3].map((item) => (
          <Skeleton key={item} className='h-20 w-full rounded-lg' />
        ))}
      </div>
    )
  }
  if (props.hasSearched && props.candidates.length === 0) {
    return (
      <div
        role='status'
        className='text-muted-foreground rounded-lg border border-dashed px-4 py-10 text-center text-sm'
      >
        {t('No LDAP users found')}
      </div>
    )
  }
  if (props.candidates.length === 0) return null

  return (
    <RadioGroup
      aria-label={t('LDAP users')}
      value={props.selectedKey}
      onValueChange={props.onSelect}
      className='gap-2'
    >
      {props.candidates.map((candidate, index) => {
        const id = `ldap-candidate-${index}`
        const selected = props.selectedKey === candidate.key
        return (
          <Label
            key={candidate.key}
            htmlFor={id}
            className={cn(
              'hover:border-primary/60 flex cursor-pointer items-start gap-3 rounded-lg border p-3 font-normal transition-colors',
              selected && 'border-primary ring-primary/20 ring-2'
            )}
          >
            <RadioGroupItem id={id} value={candidate.key} className='mt-0.5' />
            <span className='min-w-0 space-y-1'>
              <span className='block font-medium'>{candidate.username}</span>
              {candidate.display_name && (
                <span className='text-muted-foreground block text-xs'>
                  {candidate.display_name}
                </span>
              )}
              <span className='text-muted-foreground block text-xs break-all'>
                {[candidate.email, candidate.department]
                  .filter(Boolean)
                  .join(' · ')}
              </span>
            </span>
          </Label>
        )
      })}
    </RadioGroup>
  )
}

export function LDAPSyncDrawer(props: LDAPSyncDrawerProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useUsers()
  const [identifier, setIdentifier] = useState('')
  const [candidates, setCandidates] = useState<LDAPSyncCandidate[]>([])
  const [selectedKey, setSelectedKey] = useState('')
  const [hasSearched, setHasSearched] = useState(false)
  const reset = () => {
    setIdentifier('')
    setCandidates([])
    setSelectedKey('')
    setHasSearched(false)
  }
  const searchMutation = useMutation({
    mutationFn: (value: string) => searchLDAPUsers(value),
    onMutate: () => {
      setCandidates([])
      setSelectedKey('')
      setHasSearched(false)
    },
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(t('Failed to search LDAP users'))
        return
      }
      setCandidates(result.data?.users ?? [])
      setHasSearched(true)
    },
    onError: () => toast.error(t('Failed to search LDAP users')),
  })
  const syncMutation = useMutation({
    mutationFn: (candidate: LDAPSyncCandidate) => syncLDAPCandidate(candidate),
    onSuccess: (result) => {
      if (!result.success) {
        toast.error(t('Failed to synchronize LDAP user'))
        return
      }
      toast.success(t('LDAP user synchronized'))
      triggerRefresh()
      reset()
      props.onOpenChange(false)
    },
    onError: () => toast.error(t('Failed to synchronize LDAP user')),
  })
  const selectedCandidate = candidates.find(
    (candidate) => candidate.key === selectedKey
  )
  const handleSearch = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const value = identifier.trim()
    if (!value) {
      toast.error(t('Enter an LDAP username or email'))
      return
    }
    searchMutation.mutate(value)
  }
  const handleOpenChange = (open: boolean) => {
    if (!open) reset()
    props.onOpenChange(open)
  }

  return (
    <Sheet open={props.open} onOpenChange={handleOpenChange}>
      <SheetContent
        side='right'
        className={sideDrawerContentClassName('w-full sm:max-w-2xl')}
      >
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('Sync LDAP User')}</SheetTitle>
          <SheetDescription>
            {t('Search LDAP and select one account to synchronize.')}
          </SheetDescription>
        </SheetHeader>

        <div className={sideDrawerFormClassName('gap-4')}>
          <form onSubmit={handleSearch} className='flex items-end gap-2'>
            <div className='min-w-0 flex-1 space-y-2'>
              <Label htmlFor='ldap-sync-identifier'>
                {t('LDAP username or email')}
              </Label>
              <Input
                id='ldap-sync-identifier'
                value={identifier}
                onChange={(event) => setIdentifier(event.target.value)}
                placeholder={t('Enter an LDAP username or email')}
                autoComplete='off'
              />
            </div>
            <Button
              type='submit'
              variant='outline'
              disabled={searchMutation.isPending || syncMutation.isPending}
            >
              {searchMutation.isPending ? (
                <Spinner data-icon='inline-start' />
              ) : (
                <Search data-icon='inline-start' />
              )}
              {t('Search LDAP')}
            </Button>
          </form>

          {hasSearched && candidates.length > 0 && (
            <p className='text-muted-foreground text-xs' role='status'>
              {t('{{count}} LDAP users found', { count: candidates.length })}
            </p>
          )}
          <LDAPCandidateList
            candidates={candidates}
            selectedKey={selectedKey}
            searching={searchMutation.isPending}
            hasSearched={hasSearched}
            onSelect={setSelectedKey}
          />
        </div>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <Button
            variant='outline'
            onClick={() => handleOpenChange(false)}
            disabled={syncMutation.isPending}
          >
            {t('Cancel')}
          </Button>
          <Button
            onClick={() => {
              if (!selectedCandidate) {
                toast.error(t('Select an LDAP user to synchronize.'))
                return
              }
              syncMutation.mutate(selectedCandidate)
            }}
            disabled={!selectedCandidate || syncMutation.isPending}
          >
            {syncMutation.isPending ? (
              <Spinner data-icon='inline-start' />
            ) : (
              <RefreshCw data-icon='inline-start' />
            )}
            {syncMutation.isPending ? t('Syncing...') : t('Sync selected')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
