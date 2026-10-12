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
import { expect, test } from 'vitest'

import { channelSchema } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from '../channel-form'

const channel = channelSchema.parse({
  id: 7,
  type: 1,
  key: 'test-key',
  name: 'Account pool',
  status: 1,
  group: 'default',
  models: 'chat-model',
  created_time: 0,
  test_time: 0,
  response_time: 0,
  balance_updated_time: 0,
})

test('old channels default off and opening a saved enabled channel preserves opt-in', () => {
  expect(transformChannelToFormDefaults(channel).magpie_enabled).toBe(false)
  expect(
    transformChannelToFormDefaults({
      ...channel,
      settings: '{"magpie_enabled":true}',
    }).magpie_enabled
  ).toBe(true)
})

test.each([false, true])(
  'create and update save Magpie monitoring %s without losing other settings',
  (enabled) => {
    const values = channelFormSchema.parse({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'Account pool',
      type: 1,
      key: 'test-key',
      models: 'chat-model',
      magpie_enabled: enabled,
      settings: '{"custom_field":"preserved","magpie_enabled":true}',
    })
    const payloads = [
      transformFormDataToCreatePayload(values).channel,
      transformFormDataToUpdatePayload(values, channel.id),
    ]
    for (const payload of payloads) {
      const settings = JSON.parse(payload.settings ?? '{}')
      expect(settings.magpie_enabled).toBe(enabled)
      expect(settings.custom_field).toBe('preserved')
      expect(
        transformChannelToFormDefaults({ ...channel, ...payload })
          .magpie_enabled
      ).toBe(enabled)
    }
  }
)
