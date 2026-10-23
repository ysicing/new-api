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
import type { Control } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'

import type { ChannelFormValues } from '../lib/channel-form'

export function ChannelMagpieSetting(props: {
  control: Control<ChannelFormValues>
}) {
  const { t } = useTranslation()

  return (
    <FormField
      control={props.control}
      name='magpie_enabled'
      render={({ field }) => (
        <FormItem className='flex items-center justify-between gap-4'>
          <div className='space-y-0.5'>
            <FormLabel>{t('Magpie channel')}</FormLabel>
            <FormDescription>
              {t(
                'Check Codex accounts on this Magpie using the channel URL and key every 2 minutes from 09:00 to 22:00 Beijing time, and every 30 minutes overnight. Disable when none are available and restore when available. Query failures and manual disabling preserve the current status.'
              )}{' '}
              {t(
                'With Redis, unavailable channels defer probes until the earliest account recovery, for up to 5 minutes during the day and 30 minutes overnight (Beijing time). Manual checks bypass the cache. Channel configuration changes invalidate the cache; without Redis, checks follow the regular schedule.'
              )}
            </FormDescription>
            <FormMessage />
          </div>
          <FormControl>
            <Switch
              checked={field.value === true}
              onCheckedChange={field.onChange}
            />
          </FormControl>
        </FormItem>
      )}
    />
  )
}
