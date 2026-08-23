import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, RefreshCw, UserPlus } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'
import { useAuthStore } from '@/stores/auth-store'

import { canListAllQuotaPools, shouldShowQuotaPoolList } from './access'
import { getQuotaPool, getQuotaPools, getSelfQuotaPool } from './api'
import { QuotaPoolDetail } from './components/quota-pool-detail'
import {
  CreateQuotaPoolDialog,
  AddQuotaPoolMemberDialog,
  RefillQuotaPoolDialog,
} from './components/quota-pool-dialogs'
import { QuotaPoolList } from './components/quota-pool-list'
import { QuotaPoolSwitcher } from './components/quota-pool-switcher'
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
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(20)
  const [keyword, setKeyword] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [refillOpen, setRefillOpen] = useState(false)
  const [addMemberOpen, setAddMemberOpen] = useState(false)
  const canListAll = canListAllQuotaPools(user)
  const listFirst = shouldShowQuotaPoolList(user)
  const query = useQuery({
    queryKey: [
      'quota-pools',
      canListAll ? 'all' : 'self',
      canListAll ? page : 1,
      canListAll ? pageSize : 1,
      canListAll ? keyword : '',
    ],
    queryFn: async () => {
      if (canListAll) {
        const response = await getQuotaPools({ page, pageSize, keyword })
        return {
          pools: response.data?.items ?? [],
          total: response.data?.total ?? 0,
          page: response.data?.page ?? page,
          capabilities: response.data?.capabilities ?? noCapabilities,
        }
      }
      const response = await getSelfQuotaPool()
      return {
        pools: response.data?.pool ? [response.data.pool] : [],
        total: response.data?.pool ? 1 : 0,
        page: 1,
        capabilities: response.data?.capabilities ?? noCapabilities,
      }
    },
    placeholderData: (previousData, previousQuery) => {
      const scope = canListAll ? 'all' : 'self'
      return previousQuery?.queryKey[1] === scope ? previousData : undefined
    },
  })
  const pools = useMemo(() => query.data?.pools ?? [], [query.data?.pools])
  const total = query.data?.total ?? 0
  const capabilities = query.data?.capabilities ?? noCapabilities
  const detailQuery = useQuery({
    queryKey: ['quota-pool', selectedId, 'detail'],
    queryFn: async () => {
      if (selectedId === undefined) {
        throw new Error('Quota pool ID is required')
      }
      const response = await getQuotaPool(selectedId)
      if (!response.success || !response.data?.pool) {
        throw new Error('Failed to load quota pool')
      }
      return {
        pool: response.data.pool,
        capabilities: response.data.capabilities ?? noCapabilities,
      }
    },
    enabled: canListAll && selectedId !== undefined,
  })
  let selected: QuotaPool | undefined
  if (canListAll && selectedId !== undefined) {
    selected = detailQuery.data?.pool
  } else if (selectedId !== undefined) {
    selected = pools.find((pool) => pool.id === selectedId)
  } else if (!listFirst) {
    selected = pools[0]
  }
  const selectedCapabilities =
    canListAll && selectedId !== undefined
      ? (detailQuery.data?.capabilities ?? noCapabilities)
      : capabilities
  useEffect(() => {
    const serverPage = query.data?.page
    if (
      canListAll &&
      !query.isPlaceholderData &&
      serverPage !== undefined &&
      serverPage !== page
    ) {
      setPage(serverPage)
    }
  }, [canListAll, page, query.data?.page, query.isPlaceholderData])
  const backToList = () => {
    setReturnFocusId(selectedId)
    setSelectedId(undefined)
  }
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
  } else if (canListAll && selectedId !== undefined && detailQuery.isPending) {
    content = <Skeleton className='h-72 w-full' />
  } else if (
    canListAll &&
    selectedId !== undefined &&
    (detailQuery.isError || !detailQuery.data?.pool)
  ) {
    content = (
      <Empty>
        <EmptyHeader>
          <EmptyTitle>{t('Failed to load')}</EmptyTitle>
          <EmptyDescription>{t('Operation failed')}</EmptyDescription>
        </EmptyHeader>
        <EmptyContent>
          <div className='flex gap-2'>
            <Button variant='outline' onClick={backToList}>
              {t('Back to list')}
            </Button>
            <Button onClick={() => void detailQuery.refetch()}>
              {t('Retry')}
            </Button>
          </div>
        </EmptyContent>
      </Empty>
    )
  } else if (pools.length === 0 && !listFirst) {
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
        showControls={canListAll}
        total={total}
        page={page}
        pageSize={pageSize}
        keyword={keyword}
        isFetching={query.isFetching}
        onFocusRestored={() => setReturnFocusId(undefined)}
        onSearch={(nextKeyword) => {
          setPage(1)
          setKeyword(nextKeyword)
        }}
        onPageChange={setPage}
        onPageSizeChange={(nextPageSize) => {
          setPage(1)
          setPageSize(nextPageSize)
        }}
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
        capabilities={selectedCapabilities}
        selfMode={!canListAll}
        title={
          canListAll ? (
            <QuotaPoolSwitcher
              pool={selected}
              onSelect={(pool) => {
                setSelectedId(pool.id)
              }}
            />
          ) : undefined
        }
        onBack={listFirst ? backToList : undefined}
      />
    )
  }

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Quota pools')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          {selectedCapabilities.can_refill && selected && (
            <Button variant='outline' onClick={() => setRefillOpen(true)}>
              <RefreshCw data-icon='inline-start' />
              {t('Refill')}
            </Button>
          )}
          {selectedCapabilities.can_manage_members && selected && (
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
      </SectionPageLayout>
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
    </>
  )
}
