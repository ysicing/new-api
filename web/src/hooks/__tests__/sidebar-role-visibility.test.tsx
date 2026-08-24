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
import { renderHook } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { useSidebarView } from '../use-sidebar-view'

vi.mock('@tanstack/react-router', () => ({
  useLocation: ({
    select,
  }: {
    select: (location: { pathname: string }) => unknown
  }) => select({ pathname: '/dashboard/overview' }),
}))

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: {} }),
}))

afterEach(() => {
  useAuthStore.getState().auth.reset()
})

test('sub-admin navigation hides channels and system settings entries', () => {
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'sub-admin',
    role: ROLE.ADMIN,
  })

  const { result } = renderHook(() => useSidebarView())
  const adminGroup = result.current.navGroups.find(
    (group) => group.id === 'admin'
  )
  const adminEntryTitles = adminGroup?.items.map((item) => item.title)

  expect(adminEntryTitles).not.toContain('Channels')
  expect(adminEntryTitles).not.toContain('System Settings')
  expect(adminEntryTitles).toContain('Users')
})
