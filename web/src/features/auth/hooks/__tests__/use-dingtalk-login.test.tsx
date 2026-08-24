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
import { act, renderHook } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { createOAuthFlow, logout } from '../../api'
import { useOAuthLogin } from '../use-oauth-login'

vi.mock('../../api', () => ({
  createOAuthFlow: vi.fn(),
  logout: vi.fn(),
  telegramLogin: vi.fn(),
}))

vi.mock('@/lib/api', () => ({
  clearAuthentication: vi.fn(),
  isAuthBundle: vi.fn(),
}))

vi.mock('../use-auth-redirect', () => ({
  useAuthRedirect: () => ({ handleLoginSuccess: vi.fn() }),
}))

test('starts DingTalk hosted QR login with an OAuth state', async () => {
  vi.mocked(logout).mockResolvedValue({ success: true, message: '' })
  vi.mocked(createOAuthFlow).mockResolvedValue('state-token')
  const open = vi.spyOn(window, 'open').mockImplementation(() => null)
  const { result } = renderHook(() =>
    useOAuthLogin({
      dingtalk_login: true,
      dingtalk_client_id: 'app-key',
      server_address: 'https://api.example.com/',
    })
  )

  await act(async () => {
    await result.current.handleDingTalkLogin()
  })

  expect(createOAuthFlow).toHaveBeenCalledWith('dingtalk', 'login')
  const [target, mode] = open.mock.calls[0]
  const url = new URL(String(target))
  expect(mode).toBe('_self')
  expect(url.origin + url.pathname).toBe(
    'https://login.dingtalk.com/oauth2/auth'
  )
  expect(url.searchParams.get('client_id')).toBe('app-key')
  expect(url.searchParams.get('state')).toBe('state-token')
  expect(url.searchParams.get('redirect_uri')).toBe(
    'https://api.example.com/oauth/dingtalk'
  )
})
