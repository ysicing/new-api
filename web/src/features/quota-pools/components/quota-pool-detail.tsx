import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'

import {
  getQuotaPoolMembers,
  getQuotaPoolStats,
  getQuotaPoolTransactions,
} from '../api'
import type { QuotaPool, QuotaPoolCapabilities } from '../types'
import { PoolConfiguration } from './quota-pool-configuration'
import { PoolOverview, PoolStats, PoolTransactions } from './quota-pool-data'
import { PoolMembers } from './quota-pool-members'

type DetailTab = 'overview' | 'members' | 'transactions' | 'stats' | 'config'

export function QuotaPoolDetail(props: {
  pool: QuotaPool
  capabilities: QuotaPoolCapabilities
  selfMode?: boolean
}) {
  const { t } = useTranslation()
  const [tab, setTab] = useState<DetailTab>('overview')
  const members = useQuery({
    queryKey: ['quota-pool', props.pool.id, 'members'],
    queryFn: () => getQuotaPoolMembers(props.pool.id, props.selfMode),
    enabled: tab === 'members',
  })
  const transactions = useQuery({
    queryKey: ['quota-pool', props.pool.id, 'transactions'],
    queryFn: () => getQuotaPoolTransactions(props.pool.id, props.selfMode),
    enabled: tab === 'transactions',
  })
  const stats = useQuery({
    queryKey: ['quota-pool', props.pool.id, 'stats'],
    queryFn: () => getQuotaPoolStats(props.pool.id, props.selfMode),
    enabled: tab === 'stats',
  })

  return (
    <Card className='min-w-0'>
      <CardHeader>
        <CardTitle className='truncate'>{props.pool.name}</CardTitle>
      </CardHeader>
      <CardContent>
        <Tabs value={tab} onValueChange={(value) => setTab(value as DetailTab)}>
          <TabsList className='max-w-full overflow-x-auto'>
            <TabsTrigger value='overview'>{t('Overview')}</TabsTrigger>
            <TabsTrigger value='members'>{t('Members')}</TabsTrigger>
            <TabsTrigger value='transactions'>{t('Transactions')}</TabsTrigger>
            <TabsTrigger value='stats'>{t('Statistics')}</TabsTrigger>
            <TabsTrigger value='config'>{t('Configuration')}</TabsTrigger>
          </TabsList>
          <TabsContent value='overview'>
            <PoolOverview pool={props.pool} />
          </TabsContent>
          <TabsContent value='members'>
            <PoolMembers
              pool={props.pool}
              capabilities={props.capabilities}
              selfMode={props.selfMode}
              query={members}
            />
          </TabsContent>
          <TabsContent value='transactions'>
            <PoolTransactions query={transactions} />
          </TabsContent>
          <TabsContent value='stats'>
            <PoolStats query={stats} />
          </TabsContent>
          <TabsContent value='config'>
            <PoolConfiguration
              pool={props.pool}
              capabilities={props.capabilities}
              selfMode={props.selfMode}
            />
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  )
}
