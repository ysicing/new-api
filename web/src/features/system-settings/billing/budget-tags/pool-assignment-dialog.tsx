import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'

import type { AssignableQuotaPool, BudgetTag } from './types'

export function PoolAssignmentDialog(props: {
  tag: BudgetTag | null
  pools: AssignableQuotaPool[]
  selectedPoolIds: Set<number>
  onSelectedPoolIdsChange: (selected: Set<number>) => void
  onSave: () => void
  onOpenChange: (open: boolean) => void
  saving: boolean
}) {
  const { t } = useTranslation()
  const open = props.tag !== null

  return (
    <Dialog
      open={open}
      onOpenChange={props.onOpenChange}
      title={t('Assign quota pools')}
      description={
        props.tag
          ? t(
              'Choose quota pools for {{name}}. A quota pool can belong to only one budget tag.',
              {
                name: props.tag.name,
              }
            )
          : undefined
      }
      footer={
        <>
          <Button
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={props.saving}
          >
            {t('Cancel')}
          </Button>
          <Button onClick={props.onSave} disabled={props.saving}>
            {t('Save assignments')}
          </Button>
        </>
      }
    >
      <div className='space-y-2'>
        {props.pools.map((pool) => {
          const checked = props.selectedPoolIds.has(pool.id)
          return (
            <Label
              key={pool.id}
              className='hover:bg-muted/50 flex cursor-pointer items-center gap-3 rounded-lg border px-3 py-2.5'
            >
              <Checkbox
                checked={checked}
                aria-label={pool.name}
                onCheckedChange={(nextChecked) => {
                  const selected = new Set(props.selectedPoolIds)
                  if (nextChecked) {
                    selected.add(pool.id)
                  } else {
                    selected.delete(pool.id)
                  }
                  props.onSelectedPoolIdsChange(selected)
                }}
              />
              <span className='min-w-0 flex-1 truncate'>{pool.name}</span>
              {pool.budget_tag_id > 0 &&
                pool.budget_tag_id !== props.tag?.id && (
                  <span className='text-muted-foreground text-xs'>
                    {t('Assigned to another tag')}
                  </span>
                )}
            </Label>
          )
        })}
        {props.pools.length === 0 && (
          <p className='text-muted-foreground py-6 text-center text-sm'>
            {t('No quota pools available')}
          </p>
        )}
      </div>
    </Dialog>
  )
}
