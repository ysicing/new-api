import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, RefreshCw, UserPlus } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { useAuthStore } from '@/stores/auth-store'

import { canListAllQuotaPools, shouldShowQuotaPoolList } from './access'
import { getQuotaPools, getSelfQuotaPool } from './api'
import { QuotaPoolDetail } from './components/quota-pool-detail'
import {
  CreateQuotaPoolDialog,
  AddQuotaPoolMemberDialog,
  RefillQuotaPoolDialog,
} from './components/quota-pool-dialogs'
import { QuotaPoolList } from './components/quota-pool-list'
import type { QuotaPool, QuotaPoolCapabilities } from './types'

const noCapabilities: QuotaPoolCapabilities = {
  can_view: false,
  can_edit: false,
  can_edit_monthly_refill: false,
  can_refill: false,
  can_manage_members: false,
  can_manage_v1_admins: false,
  can_manage_v2_admins: false,
  can_delete: false,
}

export function QuotaPools() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const queryClient = useQueryClient()
  const [selectedId, setSelectedId] = useState<number>()
  const [returnFocusId, setReturnFocusId] = useState<number>()
  const [createOpen, setCreateOpen] = useState(false)
  const [refillOpen, setRefillOpen] = useState(false)
  const [addMemberOpen, setAddMemberOpen] = useState(false)
  const canListAll = canListAllQuotaPools(user)
  const listFirst = shouldShowQuotaPoolList(user)
  const query = useQuery({
    queryKey: ['quota-pools', canListAll ? 'all' : 'self'],
    queryFn: async () => {
      if (canListAll) {
        const response = await getQuotaPools()
        return {
          pools: response.data?.items ?? [],
          capabilities: response.data?.capabilities ?? noCapabilities,
        }
      }
      const response = await getSelfQuotaPool()
      return {
        pools: response.data?.pool ? [response.data.pool] : [],
        capabilities: response.data?.capabilities ?? noCapabilities,
      }
    },
  })
  const pools = useMemo(() => query.data?.pools ?? [], [query.data?.pools])
  const capabilities = query.data?.capabilities ?? noCapabilities
  const selected =
    pools.find((pool) => pool.id === selectedId) ??
    (listFirst ? undefined : pools[0])
  const refresh = async () => {
    const invalidations = [
      queryClient.invalidateQueries({ queryKey: ['quota-pools'] }),
    ]
    if (selectedId) {
      invalidations.push(
        queryClient.invalidateQueries({ queryKey: ['quota-pool', selectedId] })
      )
    }
    await Promise.all(invalidations)
  }

  let content: React.ReactNode
  if (query.isLoading) {
    content = <Skeleton className='h-72 w-full' />
  } else if (pools.length === 0) {
    content = (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>{t('No quota pools')}</EmptyTitle>
          <EmptyDescription>
            {t('No quota pool is available for this account.')}
          </EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  } else if (!selected) {
    content = (
      <QuotaPoolList
        pools={pools}
        selectedId={selectedId}
        focusPoolId={returnFocusId}
        onFocusRestored={() => setReturnFocusId(undefined)}
        onSelect={(pool: QuotaPool) => {
          setReturnFocusId(undefined)
          setSelectedId(pool.id)
        }}
      />
    )
  } else {
    content = (
      <QuotaPoolDetail
        key={selected.id}
        pool={selected}
        capabilities={capabilities}
        selfMode={!canListAll}
        onBack={
          listFirst
            ? () => {
                setReturnFocusId(selected.id)
                setSelectedId(undefined)
              }
            : undefined
        }
      />
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Quota pools')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        {capabilities.can_refill && selected && (
          <Button variant='outline' onClick={() => setRefillOpen(true)}>
            <RefreshCw data-icon='inline-start' />
            {t('Refill')}
          </Button>
        )}
        {capabilities.can_manage_members && selected && (
          <Button variant='outline' onClick={() => setAddMemberOpen(true)}>
            <UserPlus data-icon='inline-start' />
            {t('Add member')}
          </Button>
        )}
        {capabilities.can_delete && (
          <Button onClick={() => setCreateOpen(true)}>
            <Plus data-icon='inline-start' />
            {t('Create pool')}
          </Button>
        )}
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>{content}</SectionPageLayout.Content>
      <CreateQuotaPoolDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onSaved={refresh}
      />
      <RefillQuotaPoolDialog
        poolId={selected?.id}
        open={refillOpen}
        onOpenChange={setRefillOpen}
        onSaved={refresh}
      />
      <AddQuotaPoolMemberDialog
        poolId={selected?.id}
        self={!canListAll}
        open={addMemberOpen}
        onOpenChange={setAddMemberOpen}
        onSaved={refresh}
      />
    </SectionPageLayout>
  )
}
