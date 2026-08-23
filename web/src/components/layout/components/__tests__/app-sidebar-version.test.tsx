/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { render, screen } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { SidebarProvider } from '@/components/ui/sidebar'

import { AppSidebar } from '../app-sidebar'

vi.mock('@/context/layout-provider', () => ({
  useLayout: () => ({ collapsible: 'icon', variant: 'sidebar' }),
}))

vi.mock('@/hooks/use-sidebar-view', () => ({
  useSidebarView: () => ({ key: 'root', view: null, navGroups: [] }),
}))

afterEach(() => {
  vi.unstubAllEnvs()
})

test('shows the injected build version at the bottom of the expanded sidebar', () => {
  vi.stubEnv('PUBLIC_BUILD_VERSION', 'rc.25-test')
  const { container } = render(
    <SidebarProvider>
      <AppSidebar />
    </SidebarProvider>
  )

  expect(screen.getByText('Build rc.25-test')).toBeInTheDocument()
  expect(container.querySelector('[data-sidebar="footer"]')).toHaveClass(
    'group-data-[collapsible=icon]:hidden'
  )
})

test('does not reserve sidebar footer space without an injected version', () => {
  vi.stubEnv('PUBLIC_BUILD_VERSION', '')
  const { container } = render(
    <SidebarProvider>
      <AppSidebar />
    </SidebarProvider>
  )

  expect(container.querySelector('[data-sidebar="footer"]')).toBeNull()
})
