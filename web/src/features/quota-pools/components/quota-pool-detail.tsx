import { Alert02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { lazy, Suspense, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import {
  getQuotaPoolMembers,
  getQuotaPoolOperationLogs,
  getQuotaPoolStats,
  getQuotaPoolTransactions,
} from '../api'
import { isQuotaPoolStatsRangeReady } from '../lib/quota-pool-stats-time'
import type {
  QuotaPool,
  QuotaPoolAdminContact,
  QuotaPoolCapabilities,
  QuotaPoolDirectoryItem,
  QuotaPoolStatsRange,
} from '../types'
import { PoolConfiguration } from './quota-pool-configuration'
import {
  PoolOperationLogs,
  PoolAdminContacts,
  PoolOverview,
  PoolTransactions,
} from './quota-pool-data'
import { AvailableQuotaPoolDirectory } from './quota-pool-directory'
import { PoolMembers } from './quota-pool-members'

const PoolStats = lazy(() =>
  import('./quota-pool-stats').then((module) => ({ default: module.PoolStats }))
)

const QUOTA_POOL_STATS_STALE_TIME = 5 * 60 * 1000

type DetailTab =
  | 'overview'
  | 'members'
  | 'transactions'
  | 'logs'
  | 'stats'
  | 'config'

export function QuotaPoolDetail(props: {
  pool: QuotaPool
  capabilities: QuotaPoolCapabilities
  adminContacts?: QuotaPoolAdminContact[]
  availablePools?: QuotaPoolDirectoryItem[]
  selfMode?: boolean
  onBack?: () => void
  title?: ReactNode
}) {
  const { t } = useTranslation()
  const [tab, setTab] = useState<DetailTab>('overview')
  const [membersPage, setMembersPage] = useState(1)
  const [membersPageSize, setMembersPageSize] = useState(20)
  const [membersKeyword, setMembersKeyword] = useState('')
  const [statsRange, setStatsRange] = useState<QuotaPoolStatsRange>(() => ({
    preset: 'rolling_7d',
  }))
  const canViewManagement = props.capabilities.can_manage_members
  const showNewUserNotice =
    props.selfMode === true &&
    !canViewManagement &&
    props.pool.pool_type === 'new_user'
  const showPoolAdminContacts =
    !canViewManagement && props.pool.pool_type === 'normal'
  const activeTab = canViewManagement ? tab : 'overview'
  const members = useQuery({
    queryKey: [
      'quota-pool',
      props.pool.id,
      'members',
      props.selfMode ? 'self' : 'all',
      membersPage,
      membersPageSize,
      membersKeyword,
    ],
    queryFn: () =>
      getQuotaPoolMembers(props.pool.id, props.selfMode, {
        page: membersPage,
        pageSize: membersPageSize,
        keyword: membersKeyword,
      }),
    enabled: canViewManagement && tab === 'members',
  })
  const transactions = useQuery({
    queryKey: ['quota-pool', props.pool.id, 'transactions'],
    queryFn: () => getQuotaPoolTransactions(props.pool.id, props.selfMode),
    enabled: canViewManagement && tab === 'transactions',
  })
  const stats = useQuery({
    queryKey: ['quota-pool', props.pool.id, 'stats', statsRange],
    queryFn: () => getQuotaPoolStats(props.pool.id, props.selfMode, statsRange),
    enabled:
      canViewManagement &&
      tab === 'stats' &&
      isQuotaPoolStatsRangeReady(statsRange),
    staleTime: QUOTA_POOL_STATS_STALE_TIME,
    retry: false,
    placeholderData: keepPreviousData,
  })
  const logs = useQuery({
    queryKey: ['quota-pool', props.pool.id, 'operation-logs'],
    queryFn: () => getQuotaPoolOperationLogs(props.pool.id, props.selfMode),
    enabled: canViewManagement && tab === 'logs',
  })

  return (
    <Card className='min-w-0'>
      <CardHeader>
        <div className='flex min-w-0 items-center gap-2'>
          {props.onBack ? (
            <Button autoFocus size='sm' variant='ghost' onClick={props.onBack}>
              <ArrowLeft data-icon='inline-start' aria-hidden='true' />
              {t('Back to list')}
            </Button>
          ) : null}
          <CardTitle className='min-w-0'>
            {props.title ?? (
              <span className='block truncate'>{props.pool.name}</span>
            )}
          </CardTitle>
        </div>
      </CardHeader>
      <CardContent>
        <Tabs
          value={activeTab}
          onValueChange={(value) => setTab(value as DetailTab)}
        >
          <TabsList className='max-w-full overflow-x-auto'>
            <TabsTrigger value='overview'>{t('Overview')}</TabsTrigger>
            {canViewManagement && (
              <>
                <TabsTrigger value='members'>{t('Members')}</TabsTrigger>
                <TabsTrigger value='transactions'>
                  {t('Transactions')}
                </TabsTrigger>
                <TabsTrigger value='logs'>{t('Operation logs')}</TabsTrigger>
                <TabsTrigger value='stats'>{t('Statistics')}</TabsTrigger>
                <TabsTrigger value='config'>{t('Configuration')}</TabsTrigger>
              </>
            )}
          </TabsList>
          <TabsContent value='overview'>
            {showNewUserNotice ? (
              <Alert className='mb-4'>
                <HugeiconsIcon icon={Alert02Icon} aria-hidden='true' />
                <AlertTitle>{t('Trial quota notice')}</AlertTitle>
                <AlertDescription>
                  {t(
                    'Please contact your department quota pool administrator to join the appropriate quota pool. The current pool provides a one-time trial quota only; no additional quota will be granted after it is used up.'
                  )}
                </AlertDescription>
              </Alert>
            ) : null}
            <PoolOverview pool={props.pool} />
            {showNewUserNotice ? (
              <AvailableQuotaPoolDirectory pools={props.availablePools ?? []} />
            ) : null}
            {showPoolAdminContacts && (
              <PoolAdminContacts contacts={props.adminContacts ?? []} />
            )}
          </TabsContent>
          {canViewManagement && (
            <>
              <TabsContent value='members'>
                <PoolMembers
                  pool={props.pool}
                  capabilities={props.capabilities}
                  selfMode={props.selfMode}
                  query={members}
                  page={membersPage}
                  pageSize={membersPageSize}
                  keyword={membersKeyword}
                  onSearch={(keyword) => {
                    setMembersPage(1)
                    setMembersKeyword(keyword)
                  }}
                  onPageChange={setMembersPage}
                  onPageSizeChange={(pageSize) => {
                    setMembersPage(1)
                    setMembersPageSize(pageSize)
                  }}
                />
              </TabsContent>
              <TabsContent value='transactions'>
                <PoolTransactions query={transactions} />
              </TabsContent>
              <TabsContent value='logs'>
                <PoolOperationLogs query={logs} />
              </TabsContent>
              <TabsContent value='stats'>
                <Suspense
                  fallback={
                    <p
                      className='text-muted-foreground py-6 text-center text-sm'
                      role='status'
                    >
                      {t('Loading...')}
                    </p>
                  }
                >
                  <PoolStats
                    query={stats}
                    range={statsRange}
                    onRangeChange={setStatsRange}
                    poolId={props.pool.id}
                    selfMode={props.selfMode}
                  />
                </Suspense>
              </TabsContent>
              <TabsContent value='config'>
                <PoolConfiguration
                  pool={props.pool}
                  capabilities={props.capabilities}
                  selfMode={props.selfMode}
                />
              </TabsContent>
            </>
          )}
        </Tabs>
      </CardContent>
    </Card>
  )
}
