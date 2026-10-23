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

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: null }),
}))

test.each([
  { isAuthenticated: false, primaryAction: 'Get Started' },
  { isAuthenticated: true, primaryAction: 'Go to Dashboard' },
])(
  'hero preserves upstream Docs when authenticated is $isAuthenticated',
  ({ isAuthenticated, primaryAction }) => {
    render(<Hero isAuthenticated={isAuthenticated} />)

    expect(
      screen.getByRole('button', { name: primaryAction })
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Docs' })).toHaveAttribute(
      'href',
      'https://docs.newapi.pro'
    )
  }
)
