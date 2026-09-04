/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { SidebarModulesSection } from '@/features/system-settings/maintenance/sidebar-modules-section'

import { SidebarModulesCard } from '../sidebar-modules-card'

vi.mock('@/lib/api', () => ({
  api: {
    get: vi.fn().mockResolvedValue({ data: { success: false } }),
    put: vi.fn(),
  },
}))

test('describes Tools as read-only self-service in both sidebar settings', () => {
  const queryClient = new QueryClient()

  render(
    <QueryClientProvider client={queryClient}>
      <SidebarModulesCard />
      <SidebarModulesSection
        config={{ personal: { enabled: true, tools: true } }}
        initialSerialized=''
      />
    </QueryClientProvider>
  )

  expect(
    screen.getAllByText('Account self-service queries and diagnostics.')
  ).toHaveLength(2)
  expect(
    screen.queryByText('Manage auto-recharge settings.')
  ).not.toBeInTheDocument()
})
