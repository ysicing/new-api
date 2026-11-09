/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { VChart } from '@visactor/react-vchart'
import { memo } from 'react'
import { useTranslation } from 'react-i18next'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useTheme } from '@/context/theme-provider'
import { formatQuota } from '@/lib/format'
import { VCHART_OPTION } from '@/lib/vchart'

import type { QuotaPoolTrendStat } from '../types'

export const QuotaPoolStatsCharts = memo(function QuotaPoolStatsCharts(props: {
  trend: QuotaPoolTrendStat[]
}) {
  const { t } = useTranslation()
  const { resolvedTheme } = useTheme()
  const values = props.trend.map((item) => ({
    date: item.label,
    activeMembers: item.active_members,
    activeRate: item.active_rate,
    requestCount: item.request_count,
    usedQuota: item.used_quota,
  }))
  const theme = resolvedTheme === 'dark' ? 'dark' : 'light'
  const activeSpec = {
    type: 'area' as const,
    data: [{ id: 'activity', values }],
    xField: 'date',
    yField: 'activeMembers',
    point: { visible: true },
    area: { visible: true },
    axes: [
      { orient: 'bottom' as const, type: 'band' as const },
      { orient: 'left' as const, type: 'linear' as const, min: 0 },
    ],
    tooltip: {
      mark: {
        content: [
          {
            key: t('Active members'),
            value: (datum: Record<string, unknown>) =>
              Number(datum.activeMembers) || 0,
          },
          {
            key: t('Activity rate'),
            value: (datum: Record<string, unknown>) =>
              `${Number(datum.activeRate || 0).toFixed(2)}%`,
          },
        ],
      },
    },
    theme,
    background: 'transparent',
  }
  const usageSpec = {
    type: 'common' as const,
    data: [{ id: 'usage', values }],
    series: [
      {
        type: 'bar' as const,
        name: t('Requests'),
        dataId: 'usage',
        xField: 'date',
        yField: 'requestCount',
      },
      {
        type: 'line' as const,
        name: t('Usage'),
        dataId: 'usage',
        xField: 'date',
        yField: 'usedQuota',
        point: { visible: true },
      },
    ],
    axes: [
      { orient: 'bottom' as const, type: 'band' as const },
      {
        orient: 'left' as const,
        type: 'linear' as const,
        min: 0,
        seriesIndex: [0],
      },
      {
        orient: 'right' as const,
        type: 'linear' as const,
        min: 0,
        seriesIndex: [1],
        label: {
          formatMethod: (value: number | string) => formatQuota(Number(value)),
        },
      },
    ],
    tooltip: {
      mark: {
        content: [
          {
            key: t('Requests'),
            value: (datum: Record<string, unknown>) =>
              Number(datum.requestCount) || 0,
          },
          {
            key: t('Usage'),
            value: (datum: Record<string, unknown>) =>
              formatQuota(Number(datum.usedQuota) || 0),
          },
        ],
      },
    },
    theme,
    background: 'transparent',
  }

  return (
    <div className='grid gap-3 xl:grid-cols-2'>
      <Card>
        <CardHeader>
          <CardTitle>{t('Member activity trend')}</CardTitle>
        </CardHeader>
        <CardContent className='h-[300px]'>
          <VChart spec={activeSpec} option={VCHART_OPTION} />
        </CardContent>
        <table className='sr-only'>
          <caption>{t('Trend member activity data')}</caption>
          <thead>
            <tr>
              <th>{t('Date')}</th>
              <th>{t('Active members')}</th>
              <th>{t('Activity rate')}</th>
            </tr>
          </thead>
          <tbody>
            {values.map((item) => (
              <tr key={item.date}>
                <td>{item.date}</td>
                <td>{item.activeMembers}</td>
                <td>{item.activeRate.toFixed(2)}%</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>{t('Request and usage trend')}</CardTitle>
        </CardHeader>
        <CardContent className='h-[300px]'>
          <VChart spec={usageSpec} option={VCHART_OPTION} />
        </CardContent>
        <table className='sr-only'>
          <caption>{t('Trend request and usage data')}</caption>
          <thead>
            <tr>
              <th>{t('Date')}</th>
              <th>{t('Requests')}</th>
              <th>{t('Usage')}</th>
            </tr>
          </thead>
          <tbody>
            {values.map((item) => (
              <tr key={item.date}>
                <td>{item.date}</td>
                <td>{item.requestCount}</td>
                <td>{formatQuota(item.usedQuota)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </Card>
    </div>
  )
})
