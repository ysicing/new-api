import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { ButtonGroup } from '@/components/ui/button-group'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

export function QuotaPoolHistoryPagination(props: {
  page: number
  pageSize: number
  total: number
  loading: boolean
  onPageChange: (page: number) => void
  onPageSizeChange: (pageSize: number) => void
}) {
  const { t } = useTranslation()
  const totalPages = Math.max(1, Math.ceil(props.total / props.pageSize))
  return (
    <div className='mt-3 flex flex-wrap items-center justify-between gap-2'>
      <div className='text-muted-foreground text-sm'>
        {t('Total:')} <span className='tabular-nums'>{props.total}</span>
        {' · '}
        {t('Page {{current}} of {{total}}', {
          current: props.page,
          total: totalPages,
        })}
      </div>
      <div className='flex items-center gap-2'>
        <NativeSelect
          aria-label={t('Rows per page')}
          value={props.pageSize}
          disabled={props.loading}
          onChange={(event) =>
            props.onPageSizeChange(Number(event.target.value))
          }
        >
          {[10, 20, 50, 100].map((size) => (
            <NativeSelectOption key={size} value={size}>
              {size}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        <ButtonGroup aria-label={t('Page')}>
          <Button
            size='sm'
            variant='outline'
            disabled={props.loading || props.page <= 1}
            onClick={() => props.onPageChange(props.page - 1)}
          >
            {t('Previous page')}
          </Button>
          <Button
            size='sm'
            variant='outline'
            disabled={props.loading || props.page >= totalPages}
            onClick={() => props.onPageChange(props.page + 1)}
          >
            {t('Next page')}
          </Button>
        </ButtonGroup>
      </div>
    </div>
  )
}
