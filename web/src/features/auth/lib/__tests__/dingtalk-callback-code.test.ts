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

import { resolveOAuthAuthorizationCode } from '../oauth-callback-code'

test('uses DingTalk authCode while preserving standard OAuth code', () => {
  expect(
    resolveOAuthAuthorizationCode('dingtalk', { authCode: 'ding-code' })
  ).toBe('ding-code')
  expect(resolveOAuthAuthorizationCode('github', { code: 'oauth-code' })).toBe(
    'oauth-code'
  )
  expect(
    resolveOAuthAuthorizationCode('github', { authCode: 'not-standard' })
  ).toBe('')
})
