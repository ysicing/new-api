import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Landmark, Pencil, Tags, Trash2, WalletCards } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatQuota } from '@/lib/format'

import { SettingsSection } from '../../components/settings-section'
import {
  createQuotaPoolBudgetTag,
  deleteQuotaPoolBudgetTag,
  getQuotaPoolBudgetStats,
  listAssignableQuotaPools,
  listQuotaPoolBudgetTags,
  replaceQuotaPoolBudgetTagPools,
  updateQuotaPoolBudgetTag,
} from './api'
import { PoolAssignmentDialog } from './pool-assignment-dialog'
import type { BudgetTag } from './types'

const budgetTagsQueryKey = ['quota-pool-budget-tags'] as const
const budgetPoolsQueryKey = ['quota-pool-budget-pools'] as const

function currentBeijingMonth(): string {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
  }).format(new Date())
}

export function BudgetTagsSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const currentMonth = currentBeijingMonth()
  const [draftStartMonth, setDraftStartMonth] = useState(currentMonth)
  const [draftEndMonth, setDraftEndMonth] = useState(currentMonth)
  const [range, setRange] = useState({
    startMonth: currentMonth,
    endMonth: currentMonth,
  })
  const [newTagName, setNewTagName] = useState('')
  const [editingTag, setEditingTag] = useState<BudgetTag | null>(null)
  const [editingName, setEditingName] = useState('')
  const [deletingTag, setDeletingTag] = useState<BudgetTag | null>(null)
  const [assignmentTag, setAssignmentTag] = useState<BudgetTag | null>(null)
  const [selectedPoolIds, setSelectedPoolIds] = useState<Set<number>>(new Set())

  const tagsQuery = useQuery({
    queryKey: budgetTagsQueryKey,
    queryFn: listQuotaPoolBudgetTags,
  })
  const statsQuery = useQuery({
    queryKey: ['quota-pool-budget-stats', range],
    queryFn: () => getQuotaPoolBudgetStats(range),
  })
  const poolsQuery = useQuery({
    queryKey: budgetPoolsQueryKey,
    queryFn: listAssignableQuotaPools,
  })

  const refreshBudgetData = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: budgetTagsQueryKey }),
      queryClient.invalidateQueries({ queryKey: ['quota-pool-budget-stats'] }),
      queryClient.invalidateQueries({ queryKey: budgetPoolsQueryKey }),
    ])
  }
  const createMutation = useMutation({
    mutationFn: (name: string) => createQuotaPoolBudgetTag(name),
    onSuccess: async () => {
      setNewTagName('')
      toast.success(t('Budget tag created'))
      await refreshBudgetData()
    },
  })
  const updateMutation = useMutation({
    mutationFn: ({ id, name }: { id: number; name: string }) =>
      updateQuotaPoolBudgetTag(id, name),
    onSuccess: async () => {
      setEditingTag(null)
      toast.success(t('Budget tag updated'))
      await refreshBudgetData()
    },
  })
  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteQuotaPoolBudgetTag(id),
    onSuccess: async () => {
      setDeletingTag(null)
      toast.success(t('Budget tag deleted'))
      await refreshBudgetData()
    },
  })
  const assignMutation = useMutation({
    mutationFn: ({ id, poolIds }: { id: number; poolIds: number[] }) =>
      replaceQuotaPoolBudgetTagPools(id, poolIds),
    onSuccess: async () => {
      setAssignmentTag(null)
      toast.success(t('Quota pool assignments updated'))
      await refreshBudgetData()
    },
  })

  const tags = tagsQuery.data?.data ?? []
  const stats = statsQuery.data?.data
  const pools = poolsQuery.data?.data.items ?? []
  const isLoading = tagsQuery.isLoading || statsQuery.isLoading
  const queryFailed =
    tagsQuery.isError || statsQuery.isError || poolsQuery.isError

  const openAssignments = (tag: BudgetTag) => {
    setSelectedPoolIds(
      new Set(
        pools
          .filter((pool) => pool.budget_tag_id === tag.id)
          .map((pool) => pool.id)
      )
    )
    setAssignmentTag(tag)
  }

  if (queryFailed) {
    return (
      <SettingsSection title={t('Budget Tags')}>
        <Card role='alert' className='border-destructive/30'>
          <CardHeader>
            <CardTitle>{t('Unable to load budget statistics')}</CardTitle>
            <CardDescription>
              {t(
                'Check the connection and try again. No amount is shown while the query is unavailable.'
              )}
            </CardDescription>
            <CardAction>
              <Button
                variant='outline'
                onClick={() => {
                  void Promise.all([
                    tagsQuery.refetch(),
                    statsQuery.refetch(),
                    poolsQuery.refetch(),
                  ])
                }}
              >
                {t('Retry')}
              </Button>
            </CardAction>
          </CardHeader>
        </Card>
      </SettingsSection>
    )
  }

  return (
    <SettingsSection title={t('Budget Tags')}>
      <Card>
        <CardHeader>
          <CardTitle>{t('Natural month range')}</CardTitle>
          <CardDescription>
            {t(
              'Aggregate quota pool funding and consumption by the current budget tag.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='grid gap-3 sm:grid-cols-[1fr_1fr_auto] sm:items-end'>
          <div className='space-y-1.5'>
            <Label htmlFor='budget-start-month'>{t('Start month')}</Label>
            <Input
              id='budget-start-month'
              type='month'
              value={draftStartMonth}
              onChange={(event) => setDraftStartMonth(event.target.value)}
            />
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='budget-end-month'>{t('End month')}</Label>
            <Input
              id='budget-end-month'
              type='month'
              value={draftEndMonth}
              onChange={(event) => setDraftEndMonth(event.target.value)}
            />
          </div>
          <Button
            onClick={() =>
              setRange({
                startMonth: draftStartMonth,
                endMonth: draftEndMonth,
              })
            }
            disabled={!draftStartMonth || !draftEndMonth}
          >
            {t('Apply')}
          </Button>
        </CardContent>
      </Card>

      <div className='grid gap-3 sm:grid-cols-2'>
        <Card className='border-emerald-500/20 bg-emerald-500/5'>
          <CardHeader>
            <CardDescription className='flex items-center gap-2'>
              <Landmark className='size-4' aria-hidden='true' />
              {t('Net recharge')}
            </CardDescription>
            <CardTitle className='text-2xl tabular-nums'>
              {formatQuota(stats?.summary.net_recharge ?? 0)}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card className='border-amber-500/20 bg-amber-500/5'>
          <CardHeader>
            <CardDescription className='flex items-center gap-2'>
              <WalletCards className='size-4' aria-hidden='true' />
              {t('Net consumption')}
            </CardDescription>
            <CardTitle className='text-2xl tabular-nums'>
              {formatQuota(stats?.summary.net_consumption ?? 0)}
            </CardTitle>
          </CardHeader>
        </Card>
      </div>

      {(stats?.months.length ?? 0) > 1 && (
        <Card>
          <CardHeader>
            <CardTitle>{t('Monthly totals')}</CardTitle>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Month')}</TableHead>
                  <TableHead className='text-right'>
                    {t('Net recharge')}
                  </TableHead>
                  <TableHead className='text-right'>
                    {t('Net consumption')}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {stats?.months.map((month) => (
                  <TableRow key={month.month}>
                    <TableCell>{month.month}</TableCell>
                    <TableCell className='text-right'>
                      {formatQuota(month.net_recharge)}
                    </TableCell>
                    <TableCell className='text-right'>
                      {formatQuota(month.net_consumption)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle className='flex items-center gap-2'>
            <Tags className='size-4' aria-hidden='true' />
            {t('Manage budget tags')}
          </CardTitle>
          <CardDescription>
            {t('Each quota pool can belong to one budget tag.')}
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-col gap-2 sm:flex-row'>
          <div className='flex-1 space-y-1.5'>
            <Label htmlFor='new-budget-tag'>{t('Budget tag name')}</Label>
            <Input
              id='new-budget-tag'
              value={newTagName}
              maxLength={64}
              onChange={(event) => setNewTagName(event.target.value)}
            />
          </div>
          <Button
            className='sm:self-end'
            onClick={() => createMutation.mutate(newTagName.trim())}
            disabled={!newTagName.trim() || createMutation.isPending}
          >
            {t('Add tag')}
          </Button>
        </CardContent>
      </Card>

      {isLoading && (
        <p role='status' className='text-muted-foreground py-6 text-center'>
          {t('Loading...')}
        </p>
      )}

      {!isLoading && tags.length === 0 && (
        <Card>
          <CardContent className='text-muted-foreground py-8 text-center'>
            {t('No budget tags yet')}
          </CardContent>
        </Card>
      )}

      <div className='space-y-3'>
        {tags.map((tag) => {
          const tagStats = stats?.tags.find((item) => item.tag_id === tag.id)
          const editing = editingTag?.id === tag.id
          return (
            <Card key={tag.id}>
              <CardHeader>
                <CardTitle>
                  {editing ? (
                    <Input
                      aria-label={t('Budget tag name')}
                      value={editingName}
                      maxLength={64}
                      onChange={(event) => setEditingName(event.target.value)}
                    />
                  ) : (
                    tag.name
                  )}
                </CardTitle>
                <CardDescription>
                  <StatusBadge
                    label={t('{{count}} quota pools', {
                      count: tag.pool_count,
                    })}
                    copyable={false}
                  />
                </CardDescription>
                <CardAction className='flex gap-1'>
                  {editing ? (
                    <>
                      <Button
                        size='sm'
                        onClick={() =>
                          updateMutation.mutate({
                            id: tag.id,
                            name: editingName.trim(),
                          })
                        }
                        disabled={
                          !editingName.trim() || updateMutation.isPending
                        }
                      >
                        {t('Save')}
                      </Button>
                      <Button
                        size='sm'
                        variant='outline'
                        onClick={() => setEditingTag(null)}
                      >
                        {t('Cancel')}
                      </Button>
                    </>
                  ) : (
                    <>
                      <Button
                        size='sm'
                        variant='outline'
                        onClick={() => openAssignments(tag)}
                        disabled={poolsQuery.isLoading || poolsQuery.isError}
                      >
                        {t('Manage pools')}
                      </Button>
                      <Button
                        size='icon-sm'
                        variant='ghost'
                        aria-label={t('Rename {{name}}', { name: tag.name })}
                        onClick={() => {
                          setEditingTag(tag)
                          setEditingName(tag.name)
                        }}
                      >
                        <Pencil aria-hidden='true' />
                      </Button>
                      <Button
                        size='icon-sm'
                        variant='ghost'
                        aria-label={t('Delete {{name}}', { name: tag.name })}
                        onClick={() => setDeletingTag(tag)}
                      >
                        <Trash2 aria-hidden='true' />
                      </Button>
                    </>
                  )}
                </CardAction>
              </CardHeader>
              <CardContent className='space-y-3'>
                <div className='grid grid-cols-2 gap-3'>
                  <div className='bg-muted/30 rounded-lg p-3'>
                    <p className='text-muted-foreground text-xs'>
                      {t('Net recharge')}
                    </p>
                    <p className='mt-1 font-medium tabular-nums'>
                      {formatQuota(tagStats?.net_recharge ?? 0)}
                    </p>
                  </div>
                  <div className='bg-muted/30 rounded-lg p-3'>
                    <p className='text-muted-foreground text-xs'>
                      {t('Net consumption')}
                    </p>
                    <p className='mt-1 font-medium tabular-nums'>
                      {formatQuota(tagStats?.net_consumption ?? 0)}
                    </p>
                  </div>
                </div>
                {(tagStats?.pools.length ?? 0) > 0 && (
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('Quota pool')}</TableHead>
                        <TableHead className='text-right'>
                          {t('Net recharge')}
                        </TableHead>
                        <TableHead className='text-right'>
                          {t('Net consumption')}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {tagStats?.pools.map((pool) => (
                        <TableRow key={pool.pool_id}>
                          <TableCell>{pool.pool_name}</TableCell>
                          <TableCell className='text-right'>
                            {formatQuota(pool.net_recharge)}
                          </TableCell>
                          <TableCell className='text-right'>
                            {formatQuota(pool.net_consumption)}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>
          )
        })}
      </div>

      <PoolAssignmentDialog
        tag={assignmentTag}
        pools={pools}
        selectedPoolIds={selectedPoolIds}
        onSelectedPoolIdsChange={setSelectedPoolIds}
        onOpenChange={(open) => {
          if (!open) setAssignmentTag(null)
        }}
        onSave={() => {
          if (!assignmentTag) return
          assignMutation.mutate({
            id: assignmentTag.id,
            poolIds: [...selectedPoolIds],
          })
        }}
        saving={assignMutation.isPending}
      />

      <ConfirmDialog
        open={deletingTag !== null}
        onOpenChange={(open) => {
          if (!open) setDeletingTag(null)
        }}
        title={t('Delete budget tag')}
        desc={t(
          'Delete {{name}}? Assigned quota pools must be removed first.',
          {
            name: deletingTag?.name ?? '',
          }
        )}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() => {
          if (deletingTag) deleteMutation.mutate(deletingTag.id)
        }}
      />
    </SettingsSection>
  )
}
