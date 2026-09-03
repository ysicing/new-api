/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DateTimePicker } from '@/components/datetime-picker'
import { Button } from '@/components/ui/button'
import { ButtonGroup } from '@/components/ui/button-group'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'

import { exportQuotaPoolStats } from '../api'
import {
  beijingPickerDateToTimestamp,
  isQuotaPoolStatsRangeReady,
  timestampToBeijingPickerDate,
} from '../lib/quota-pool-stats-time'
import type {
  QuotaPoolStatsActualRange,
  QuotaPoolStatsGranularity,
  QuotaPoolStatsRange,
} from '../types'
import {
  defaultQuotaPoolStatsGranularity,
  quotaPoolStatsPresets,
} from './quota-pool-stats-options'

type ExportFormat = 'markdown' | 'xlsx'

export function QuotaPoolStatsExportDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  poolId: number
  selfMode?: boolean
  initialRange: QuotaPoolStatsActualRange
}) {
  const { t } = useTranslation()
  const [range, setRange] = useState<QuotaPoolStatsRange>({
    preset: 'custom',
    start_timestamp: props.initialRange.start_timestamp,
    end_timestamp: props.initialRange.end_timestamp,
  })
  const [granularity, setGranularity] = useState<QuotaPoolStatsGranularity>(
    props.initialRange.granularity
  )
  const [format, setFormat] = useState<ExportFormat>('markdown')
  const [exporting, setExporting] = useState(false)
  const initialStart = props.initialRange.start_timestamp
  const initialEnd = props.initialRange.end_timestamp
  const initialGranularity = props.initialRange.granularity

  const selectPreset = (preset: QuotaPoolStatsRange['preset']) => {
    if (preset === 'custom') {
      setRange({
        preset,
        start_timestamp: initialStart,
        end_timestamp: initialEnd,
      })
      setGranularity(initialGranularity)
      return
    }
    setRange({ preset })
    setGranularity(defaultQuotaPoolStatsGranularity(preset))
  }

  const handleExport = async () => {
    if (!isQuotaPoolStatsRangeReady(range)) {
      toast.error(t('Statistics time range is invalid'))
      return
    }
    setExporting(true)
    try {
      const request = { ...range, granularity }
      const exported = await exportQuotaPoolStats(
        props.poolId,
        props.selfMode === true,
        request,
        format
      )
      const href = URL.createObjectURL(exported.blob)
      const link = document.createElement('a')
      link.href = href
      link.download = exported.filename
      document.body.append(link)
      link.click()
      link.remove()
      URL.revokeObjectURL(href)
      toast.success(t('Statistics exported successfully'))
      props.onOpenChange(false)
    } catch {
      toast.error(t('Failed to export statistics'))
    } finally {
      setExporting(false)
    }
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{t('Export quota pool statistics')}</DialogTitle>
          <DialogDescription>
            {t(
              'Choose an independent time range, granularity, and file format.'
            )}
          </DialogDescription>
        </DialogHeader>
        <div className='space-y-5'>
          <div className='space-y-2'>
            <Label>{t('Time range')}</Label>
            <ButtonGroup
              className='flex-wrap'
              aria-label={t('Export time range')}
            >
              {quotaPoolStatsPresets.map((preset) => (
                <Button
                  key={preset.value}
                  size='sm'
                  variant={
                    range.preset === preset.value ? 'secondary' : 'outline'
                  }
                  aria-pressed={range.preset === preset.value}
                  onClick={() => selectPreset(preset.value)}
                >
                  {t(preset.label)}
                </Button>
              ))}
            </ButtonGroup>
          </div>
          {range.preset === 'custom' ? (
            <div className='grid gap-3 sm:grid-cols-2'>
              <div
                className='space-y-2'
                role='group'
                aria-labelledby='quota-pool-export-start-time'
              >
                <Label id='quota-pool-export-start-time'>
                  {t('Start time')}
                </Label>
                <DateTimePicker
                  dateAriaLabel={t('Start date')}
                  timeAriaLabel={t('Start time')}
                  clearAriaLabel={t('Clear start time')}
                  utcFields
                  value={
                    range.start_timestamp
                      ? timestampToBeijingPickerDate(range.start_timestamp)
                      : undefined
                  }
                  onChange={(date) =>
                    setRange({
                      ...range,
                      start_timestamp: date
                        ? beijingPickerDateToTimestamp(date)
                        : undefined,
                    })
                  }
                />
              </div>
              <div
                className='space-y-2'
                role='group'
                aria-labelledby='quota-pool-export-end-time'
              >
                <Label id='quota-pool-export-end-time'>{t('End time')}</Label>
                <DateTimePicker
                  dateAriaLabel={t('End date')}
                  timeAriaLabel={t('End time')}
                  clearAriaLabel={t('Clear end time')}
                  utcFields
                  value={
                    range.end_timestamp
                      ? timestampToBeijingPickerDate(range.end_timestamp)
                      : undefined
                  }
                  onChange={(date) =>
                    setRange({
                      ...range,
                      end_timestamp: date
                        ? beijingPickerDateToTimestamp(date)
                        : undefined,
                    })
                  }
                />
              </div>
            </div>
          ) : null}
          <div className='grid gap-5 sm:grid-cols-2'>
            <div className='space-y-2'>
              <Label id='quota-pool-export-granularity-label'>
                {t('Granularity')}
              </Label>
              <RadioGroup
                className='flex flex-wrap gap-4'
                aria-labelledby='quota-pool-export-granularity-label'
                value={granularity}
                onValueChange={(value) =>
                  setGranularity(value as QuotaPoolStatsGranularity)
                }
              >
                {(['hour', 'day', 'week'] as const).map((value) => {
                  const id = `quota-pool-export-granularity-${value}`
                  return (
                    <div key={value} className='flex items-center gap-2'>
                      <RadioGroupItem
                        id={id}
                        value={value}
                        aria-label={t(
                          `${value[0].toUpperCase()}${value.slice(1)} granularity`
                        )}
                      />
                      <Label htmlFor={id} className='cursor-pointer'>
                        {t(value)}
                      </Label>
                    </div>
                  )
                })}
              </RadioGroup>
            </div>
            <div className='space-y-2'>
              <Label id='quota-pool-export-format-label'>
                {t('File format')}
              </Label>
              <RadioGroup
                className='flex gap-4'
                aria-labelledby='quota-pool-export-format-label'
                value={format}
                onValueChange={(value) => setFormat(value as ExportFormat)}
              >
                <div className='flex items-center gap-2'>
                  <RadioGroupItem
                    id='quota-pool-export-markdown'
                    value='markdown'
                    aria-label={t('Markdown format')}
                  />
                  <Label
                    htmlFor='quota-pool-export-markdown'
                    className='cursor-pointer'
                  >
                    Markdown
                  </Label>
                </div>
                <div className='flex items-center gap-2'>
                  <RadioGroupItem
                    id='quota-pool-export-xlsx'
                    value='xlsx'
                    aria-label={t('Excel format')}
                  />
                  <Label
                    htmlFor='quota-pool-export-xlsx'
                    className='cursor-pointer'
                  >
                    Excel (.xlsx)
                  </Label>
                </div>
              </RadioGroup>
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={() => props.onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button disabled={exporting} onClick={() => void handleExport()}>
            {exporting ? t('Exporting...') : t('Export file')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
