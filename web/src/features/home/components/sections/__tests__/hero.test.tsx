/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { render, screen } from '@testing-library/react'
import type { AnchorHTMLAttributes } from 'react'
import { expect, test, vi } from 'vitest'

import { Hero } from '../hero'

const status = vi.hoisted(() => ({
  self_use_mode_enabled: undefined as boolean | undefined,
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status }),
}))

vi.mock('@lobehub/icons', () => ({
  CherryStudio: { Color: () => <span>Cherry Studio icon</span> },
}))

vi.mock('@tanstack/react-router', () => ({
  Link: (props: AnchorHTMLAttributes<HTMLAnchorElement> & { to: string }) => (
    <a {...props} href={props.to} />
  ),
}))

vi.mock('../../hero-terminal-demo', () => ({
  HeroTerminalDemo: () => <div>Terminal demo</div>,
}))

test.each([
  { selfUseMode: true, isAuthenticated: false, primaryAction: 'Get Started' },
  { selfUseMode: false, isAuthenticated: false, primaryAction: 'Get Started' },
  {
    selfUseMode: undefined,
    isAuthenticated: false,
    primaryAction: 'Get Started',
  },
  {
    selfUseMode: true,
    isAuthenticated: true,
    primaryAction: 'Go to Dashboard',
  },
  {
    selfUseMode: false,
    isAuthenticated: true,
    primaryAction: 'Go to Dashboard',
  },
  {
    selfUseMode: undefined,
    isAuthenticated: true,
    primaryAction: 'Go to Dashboard',
  },
])(
  'hero hides Docs with self-use mode $selfUseMode and authentication $isAuthenticated',
  ({ selfUseMode, isAuthenticated, primaryAction }) => {
    status.self_use_mode_enabled = selfUseMode
    render(<Hero isAuthenticated={isAuthenticated} />)

    expect(
      screen.getByRole('button', { name: primaryAction })
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Docs' })
    ).not.toBeInTheDocument()
  }
)
