/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { render, screen } from '@testing-library/react'
import type { AnchorHTMLAttributes } from 'react'
import { describe, expect, test, vi } from 'vitest'

import { Hero } from '../hero'

const status = vi.hoisted(() => ({
  docs_link: undefined as string | undefined,
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

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status }),
}))

describe.each([undefined, 'https://example.com/docs', '/docs'])(
  'homepage with docs link %s',
  (docsLink) => {
    test.each([
      { isAuthenticated: false, primaryAction: 'Get Started' },
      { isAuthenticated: true, primaryAction: 'Go to Dashboard' },
    ])(
      'hides Docs and preserves primary actions when authenticated is $isAuthenticated',
      ({ isAuthenticated, primaryAction }) => {
        status.docs_link = docsLink
        render(<Hero isAuthenticated={isAuthenticated} />)

        expect(
          screen.getByRole('button', { name: primaryAction })
        ).toBeVisible()
        expect(
          screen.queryByRole('button', { name: 'Docs' })
        ).not.toBeInTheDocument()
        expect(screen.getByText('Docs')).not.toBeVisible()
        expect(screen.getByText('Docs').closest('a')).toHaveAttribute(
          'href',
          docsLink || 'https://docs.newapi.pro'
        )
        if (!isAuthenticated) {
          expect(
            screen.getByRole('button', { name: 'View Pricing' })
          ).toBeVisible()
        }
      }
    )
  }
)
