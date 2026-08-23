import { useQuery } from '@tanstack/react-query'
import { ArrowLeft } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import {
  getQuotaPoolMembers,
  getQuotaPoolOperationLogs,
  getQuotaPoolStats,
  getQuotaPoolTransactions,
} from '../api'
import type {
  QuotaPool,
  QuotaPoolAdminContact,
  QuotaPoolCapabilities,
  QuotaPoolStatsPeriod,
} from '../types'
import { PoolConfiguration } from './quota-pool-configuration'
import {
  PoolOperationLogs,
  PoolAdminContacts,
  PoolOverview,
  PoolTransactions,
} from './quota-pool-data'
import { PoolMembers } from './quota-pool-members'
import { PoolStats } from './quota-pool-stats'

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
  selfMode?: boolean
  onBack?: () => void
  title?: ReactNode
}) {
  const { t } = useTranslation()
  const [tab, setTab] = useState<DetailTab>('overview')
  const [membersPage, setMembersPage] = useState(1)
  const [membersPageSize, setMembersPageSize] = useState(20)
  const [membersKeyword, setMembersKeyword] = useState('')
  const [statsPeriod, setStatsPeriod] = useState<QuotaPoolStatsPeriod>('week')
  const canViewManagement = props.capabilities.can_manage_members
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
    queryKey: ['quota-pool', props.pool.id, 'stats', statsPeriod],
    queryFn: () =>
      getQuotaPoolStats(props.pool.id, props.selfMode, statsPeriod),
    enabled: canViewManagement && tab === 'stats',
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
            <PoolOverview pool={props.pool} />
            {!canViewManagement && (
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
                <PoolStats
                  query={stats}
                  period={statsPeriod}
                  onPeriodChange={setStatsPeriod}
                />
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
