import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { QuotaPool } from '../types'

export function QuotaPoolList({
  pools,
  selectedId,
  focusPoolId,
  onFocusRestored,
  onSelect,
}: {
  pools: QuotaPool[]
  selectedId?: number
  focusPoolId?: number
  onFocusRestored?: () => void
  onSelect: (pool: QuotaPool) => void
}) {
  const { t } = useTranslation()
  useEffect(() => {
    if (focusPoolId === undefined) return
    const row = document.querySelector<HTMLElement>(
      `[data-quota-pool-id="${focusPoolId}"]`
    )
    if (!row) return
    row.focus()
    onFocusRestored?.()
  }, [focusPoolId, onFocusRestored])

  return (
    <div className='overflow-hidden rounded-lg border'>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Quota pool')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
            <TableHead className='text-right'>{t('Available quota')}</TableHead>
            <TableHead className='text-right'>{t('Members')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {pools.map((pool) => (
            <TableRow
              key={pool.id}
              data-quota-pool-id={pool.id}
              tabIndex={0}
              aria-selected={pool.id === selectedId}
              className={cn(
                'cursor-pointer transition-colors',
                pool.id === selectedId && 'bg-muted'
              )}
              onClick={() => onSelect(pool)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault()
                  onSelect(pool)
                }
              }}
            >
              <TableCell>
                <div className='flex min-w-0 flex-col gap-1'>
                  <span className='truncate font-medium'>{pool.name}</span>
                  <span className='text-muted-foreground text-xs'>
                    {t(
                      pool.pool_type === 'normal'
                        ? 'Managed pool'
                        : 'System pool'
                    )}
                  </span>
                </div>
              </TableCell>
              <TableCell>
                <Badge variant={pool.enabled ? 'default' : 'secondary'}>
                  {t(pool.enabled ? 'Enabled' : 'Disabled')}
                </Badge>
              </TableCell>
              <TableCell className='text-right tabular-nums'>
                {pool.quota < 0 ? t('Unlimited') : formatQuota(pool.quota)}
              </TableCell>
              <TableCell className='text-right tabular-nums'>
                {pool.member_count ?? 0}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  )
}
