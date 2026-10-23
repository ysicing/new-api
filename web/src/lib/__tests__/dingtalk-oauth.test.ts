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

import { buildDingTalkOAuthUrl } from '../oauth'

test('builds DingTalk hosted QR authorization URL', () => {
  const url = new URL(
    buildDingTalkOAuthUrl('app-key', 'state-token', 'https://api.example.com/')
  )

  expect(url.origin + url.pathname).toBe(
    'https://login.dingtalk.com/oauth2/auth'
  )
  expect(url.searchParams.get('client_id')).toBe('app-key')
  expect(url.searchParams.get('redirect_uri')).toBe(
    'https://api.example.com/oauth/dingtalk'
  )
  expect(url.searchParams.get('response_type')).toBe('code')
  expect(url.searchParams.get('scope')).toBe('openid corpid')
  expect(url.searchParams.get('state')).toBe('state-token')
  expect(url.searchParams.get('prompt')).toBe('consent')
})
