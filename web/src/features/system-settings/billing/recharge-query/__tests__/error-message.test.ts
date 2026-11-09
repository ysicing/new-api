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
import i18next from 'i18next'
import { beforeAll, expect, test } from 'vitest'

import { createServerError } from '@/lib/server-error-message'

import { rechargeQueryErrorMessage } from '../error-message'

beforeAll(async () => {
  await i18next.init({
    lng: 'en',
    fallbackLng: 'en',
    nsSeparator: false,
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

test('maps a backend recharge error code to localized UI copy', () => {
  const error = {
    isAxiosError: true,
    message: 'Request failed with status code 404',
    response: {
      data: {
        code: 'RECHARGE_USER_NOT_FOUND',
        message: '未找到匹配用户',
      },
    },
  }

  const message = rechargeQueryErrorMessage(
    error,
    (key: string) => key,
    'Fallback error'
  )

  expect(message).toBe('No matching user was found.')
})

test('prefers a structured translated message over the code fallback after wrapping', () => {
  const error = createServerError({
    success: false,
    code: 'RECHARGE_USER_NOT_FOUND',
    message: 'Legacy details',
    message_key: 'No matching user: {{name}}',
    message_params: { name: 'alice' },
  })

  expect(rechargeQueryErrorMessage(error, (key) => key, 'Fallback')).toBe(
    'No matching user: alice'
  )
})

test('unknown raw backend details use the safe fallback', () => {
  expect(
    rechargeQueryErrorMessage(
      {
        isAxiosError: true,
        response: { data: { message: 'Lookup unavailable' } },
      },
      (key) => key,
      'Fallback'
    )
  ).toBe('Fallback')
})

test('uses localized fallback for errors without a stable backend code', () => {
  const message = rechargeQueryErrorMessage(
    new Error('Network Error'),
    (key: string) => key,
    'Localized fallback'
  )

  expect(message).toBe('Localized fallback')
})
