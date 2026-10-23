/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

import type { QuotaPoolDirectoryItem } from '../types'
import { PoolAdminContacts } from './quota-pool-data'

export function AvailableQuotaPoolDirectory(props: {
  pools: QuotaPoolDirectoryItem[]
}) {
  const { t } = useTranslation()
  const [selectedId, setSelectedId] = useState('')
  const selected = props.pools.find((pool) => String(pool.id) === selectedId)

  return (
    <>
      <Card className='mt-3'>
        <CardHeader>
          <CardTitle className='text-sm'>
            {t('Available quota pools')}
          </CardTitle>
          <CardDescription>
            {t('Select a quota pool to view its administrators.')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {props.pools.length === 0 ? (
            <p className='text-muted-foreground text-sm'>
              {t('No quota pools')}
            </p>
          ) : (
            <NativeSelect
              aria-label={t('Available quota pools')}
              value={selectedId}
              onChange={(event) => setSelectedId(event.target.value)}
              className='w-full'
            >
              <NativeSelectOption value=''>
                {t('Select a quota pool')}
              </NativeSelectOption>
              {props.pools.map((pool) => (
                <NativeSelectOption key={pool.id} value={String(pool.id)}>
                  {pool.name}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          )}
        </CardContent>
      </Card>
      {selected ? (
        <PoolAdminContacts contacts={selected.admin_contacts} />
      ) : null}
    </>
  )
}
