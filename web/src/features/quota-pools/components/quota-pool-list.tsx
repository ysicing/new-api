import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ButtonGroup } from '@/components/ui/button-group'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Spinner } from '@/components/ui/spinner'
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
  showControls,
  total,
  page,
  pageSize,
  keyword,
  isFetching,
  onSearch,
  onPageChange,
  onPageSizeChange,
  onSelect,
}: {
  pools: QuotaPool[]
  selectedId?: number
  focusPoolId?: number
  onFocusRestored?: () => void
  showControls?: boolean
  total?: number
  page?: number
  pageSize?: number
  keyword?: string
  isFetching?: boolean
  onSearch?: (keyword: string) => void
  onPageChange?: (page: number) => void
  onPageSizeChange?: (pageSize: number) => void
  onSelect: (pool: QuotaPool) => void
}) {
  const { t } = useTranslation()
  const [searchValue, setSearchValue] = useState(keyword ?? '')
  const currentPage = page ?? 1
  const currentPageSize = pageSize ?? 20
  const totalItems = total ?? pools.length
  const totalPages = Math.max(1, Math.ceil(totalItems / currentPageSize))
  const pageSizeOptions = [
    { value: 10, label: t('10 / page') },
    { value: 20, label: t('20 / page') },
    { value: 50, label: t('50 / page') },
  ]
  useEffect(() => {
    if (focusPoolId === undefined) return
    const row = document.querySelector<HTMLElement>(
      `[data-quota-pool-id="${focusPoolId}"]`
    )
    if (row) {
      row.focus()
      onFocusRestored?.()
      return
    }
    const search = document.querySelector<HTMLInputElement>(
      '[data-quota-pool-search]'
    )
    if (search) {
      search.focus()
      onFocusRestored?.()
    }
  }, [focusPoolId, onFocusRestored])

  return (
    <div className='flex flex-col gap-3'>
      {showControls ? (
        <form
          className='flex flex-col gap-2 sm:flex-row sm:items-center'
          onSubmit={(event) => {
            event.preventDefault()
            onSearch?.(searchValue.trim())
          }}
        >
          <Input
            type='search'
            data-quota-pool-search
            aria-label={t('Search quota pools')}
            placeholder={t('Search by pool ID or name')}
            value={searchValue}
            onChange={(event) => setSearchValue(event.target.value)}
            className='sm:max-w-sm'
          />
          <Button type='submit' variant='outline'>
            {t('Search')}
          </Button>
          {isFetching ? <Spinner aria-label={t('Loading...')} /> : null}
          <NativeSelect
            size='sm'
            aria-label={t('Rows per page')}
            value={String(currentPageSize)}
            onChange={(event) => onPageSizeChange?.(Number(event.target.value))}
            className='sm:ml-auto'
          >
            {pageSizeOptions.map((option) => (
              <NativeSelectOption key={option.value} value={option.value}>
                {option.label}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </form>
      ) : null}

      <div
        className='overflow-hidden rounded-lg border'
        aria-busy={isFetching || undefined}
      >
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t('Quota pool')}</TableHead>
              <TableHead>{t('Status')}</TableHead>
              <TableHead className='text-right'>
                {t('Available quota')}
              </TableHead>
              <TableHead className='text-right'>{t('Members')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {pools.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={4}
                  className='text-muted-foreground h-24 text-center'
                >
                  {t('No quota pools')}
                </TableCell>
              </TableRow>
            ) : (
              pools.map((pool) => (
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
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {showControls ? (
        <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
          <div className='text-muted-foreground text-sm'>
            {t('Total:')} <span className='tabular-nums'>{totalItems}</span>
            {' · '}
            {t('Page {{current}} of {{total}}', {
              current: currentPage,
              total: totalPages,
            })}
          </div>
          <ButtonGroup aria-label={t('Page')}>
            <Button
              size='sm'
              variant='outline'
              disabled={currentPage <= 1}
              onClick={() => onPageChange?.(currentPage - 1)}
            >
              {t('Previous page')}
            </Button>
            <Button
              size='sm'
              variant='outline'
              disabled={currentPage >= totalPages}
              onClick={() => onPageChange?.(currentPage + 1)}
            >
              {t('Next page')}
            </Button>
          </ButtonGroup>
        </div>
      ) : null}
    </div>
  )
}
