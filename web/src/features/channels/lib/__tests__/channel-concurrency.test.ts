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
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
  transformChannelToFormDefaults,
} from '../channel-form'

const form = {
  ...CHANNEL_FORM_DEFAULT_VALUES,
  name: 'Limited channel',
  type: 1,
  key: 'test-key',
  models: 'chat-model',
}

test.each([0, 1, 100000])(
  'accepts concurrency %s and preserves it in create and update payloads',
  (limit) => {
    const parsed = channelFormSchema.parse({ ...form, max_concurrency: limit })
    const created = transformFormDataToCreatePayload(parsed)
    expect(created.channel.max_concurrency).toBe(limit)
    expect(transformFormDataToUpdatePayload(parsed, 7).max_concurrency).toBe(
      limit
    )
  }
)

test.each([-1, 1.5, 100001])('rejects invalid concurrency %s', (limit) => {
  expect(
    channelFormSchema.safeParse({ ...form, max_concurrency: limit }).success
  ).toBe(false)
})

test('defaults old channels to unlimited and loads a saved limit', () => {
  const channel = channelSchema.parse({
    id: 7,
    type: 1,
    key: 'test-key',
    name: 'Existing channel',
    status: 1,
    group: 'default',
    models: 'chat-model',
    created_time: 0,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
  })
  expect(transformChannelToFormDefaults(channel).max_concurrency).toBe(0)
  expect(
    transformChannelToFormDefaults({ ...channel, max_concurrency: 8 })
      .max_concurrency
  ).toBe(8)
})
