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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { AnchorHTMLAttributes } from 'react'
import { expect, test, vi } from 'vitest'

import type { SystemStatus } from '@/features/auth/types'

import { UserAuthForm } from '../user-auth-form'

const statusMock = vi.hoisted(() => ({
  value: null as SystemStatus | null,
}))

vi.mock('@tanstack/react-router', () => ({
  Link: (props: AnchorHTMLAttributes<HTMLAnchorElement> & { to: string }) => (
    <a {...props} href={props.to} />
  ),
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: statusMock.value }),
}))

vi.mock('@/features/auth/hooks/use-auth-redirect', () => ({
  useAuthRedirect: () => ({
    handleLoginSuccess: vi.fn(),
    redirectTo2FA: vi.fn(),
  }),
}))

vi.mock('@/features/auth/hooks/use-turnstile', () => ({
  useTurnstile: () => ({
    isTurnstileEnabled: false,
    turnstileSiteKey: '',
    turnstileToken: '',
    setTurnstileToken: vi.fn(),
    validateTurnstile: () => true,
  }),
}))

vi.mock('@/lib/passkey', () => ({
  buildAssertionResult: vi.fn(),
  prepareCredentialRequestOptions: vi.fn(),
  isPasskeySupported: () => Promise.resolve(false),
}))

test('uses LDAP by default when LDAP and password login are enabled', async () => {
  statusMock.value = {
    ldap_login: true,
    password_login_enabled: true,
  }
  const user = userEvent.setup()

  render(<UserAuthForm />)

  const passwordLoginButton = screen.getByRole('button', {
    name: 'Use password login',
  })
  expect(passwordLoginButton).toBeInTheDocument()
  expect(
    screen.getByRole('button', { name: 'Sign in with LDAP' })
  ).toHaveAttribute('type', 'submit')

  await user.click(passwordLoginButton)

  expect(
    screen.getByRole('button', { name: 'Sign in with LDAP' })
  ).toHaveAttribute('type', 'button')
  expect(screen.getByRole('button', { name: 'Sign in' })).toHaveAttribute(
    'type',
    'submit'
  )
})
