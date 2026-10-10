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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { AxiosResponse } from 'axios'
import { Toaster, toast } from 'sonner'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { TooltipProvider } from '@/components/ui/tooltip'
import { api } from '@/lib/api'
import { useSystemConfigStore } from '@/stores/system-config-store'

import type { ApiKey } from '../../types'
import { ApiKeyCell } from '../api-keys-cells'
import { ApiKeysProvider } from '../api-keys-provider'

const originalConfig = useSystemConfigStore.getState().config
let queryClient: QueryClient

function renderCell(options?: {
  serverAddress?: string
  modelLimitsEnabled?: boolean
}) {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClient.setQueryData(['status'], {
    server_address: options?.serverAddress ?? 'https://relay.example/v1/',
    chats: [],
  })
  const apiKey: ApiKey = {
    id: 7,
    name: 'My key',
    key: 'masked-********',
    status: 1,
    remain_quota: 100,
    used_quota: 0,
    unlimited_quota: false,
    expired_time: -1,
    created_time: 0,
    accessed_time: 0,
    group: 'default',
    auto_groups: null,
    allow_ips: null,
    cross_group_retry: false,
    model_limits_enabled: options?.modelLimitsEnabled ?? true,
    model_limits: 'gpt-6-sol,claude-sonnet-5',
  }
  render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <ApiKeysProvider>
          <ApiKeyCell apiKey={apiKey} />
        </ApiKeysProvider>
        <Toaster />
      </TooltipProvider>
    </QueryClientProvider>
  )
  return screen.getByRole('button', { name: 'Add to Magpie' })
}

beforeEach(() => {
  useSystemConfigStore.getState().setConfig({ systemName: 'iCode' })
  vi.spyOn(window, 'open').mockReturnValue(null)
})

afterEach(() => {
  queryClient.clear()
  toast.dismiss()
  useSystemConfigStore.setState({ config: originalConfig })
})

test('clicking the icon shows installation guidance and waits for explicit import confirmation', async () => {
  const post = vi.spyOn(api, 'post').mockResolvedValue({
    data: { success: true, data: { key: 'full-key' } },
  } as AxiosResponse)
  const action = renderCell()
  expect(screen.getByText('sk-masked-********')).toBeVisible()
  expect(action.querySelector('svg')).toHaveAttribute('aria-hidden', 'true')
  expect(action).toHaveClass('shrink-0')
  expect(post).not.toHaveBeenCalled()
  fireEvent.click(action)
  const dialog = await screen.findByRole('alertdialog', {
    name: 'Add to Magpie',
  })
  expect(dialog).toHaveAccessibleDescription(
    'If Magpie is already installed, continue to open it and import this API key. Otherwise, install Magpie first, then return here to continue.'
  )
  const install = within(dialog).getByRole('link', { name: 'Install Magpie' })
  expect(install).toHaveAttribute('href', 'https://usemagpie.ai/zh/')
  expect(install).toHaveAttribute('target', '_blank')
  expect(install).toHaveAttribute('rel', 'noopener noreferrer')
  expect(post).toHaveBeenCalledWith('/api/token/7/key')
  expect(window.open).not.toHaveBeenCalled()
  const confirm = await within(dialog).findByRole('button', {
    name: 'Continue import',
  })
  fireEvent.click(confirm)
  expect(window.open).toHaveBeenCalledWith(
    expect.stringMatching(/^magpie:\/\/import\?/),
    '_self'
  )
  const url = new URL(vi.mocked(window.open).mock.calls[0]?.[0] as string)
  expect(url.searchParams.get('name')).toBe('iCode')
  expect(url.searchParams.get('key')).toBe('sk-full-key')
  expect(url.searchParams.get('chat')).toBe('https://relay.example/v1')
  expect(url.searchParams.get('responses')).toBe('https://relay.example/v1')
  expect(url.searchParams.get('anthropic')).toBe('https://relay.example')
  expect(url.searchParams.get('models')).toBe('gpt-6-sol,claude-sonnet-5')
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
})

test('loading the full key disables confirmation while keeping installation and cancellation available', async () => {
  let finishRequest!: (response: AxiosResponse) => void
  vi.spyOn(api, 'post').mockReturnValue(
    new Promise<AxiosResponse>((resolve) => {
      finishRequest = resolve
    })
  )
  fireEvent.click(renderCell({ modelLimitsEnabled: false }))
  const dialog = await screen.findByRole('alertdialog')
  const loading = within(dialog).getByRole('button', { name: 'Loading...' })
  expect(loading).toBeDisabled()
  fireEvent.click(loading)
  expect(window.open).not.toHaveBeenCalled()
  expect(within(dialog).getByRole('button', { name: 'Cancel' })).toBeEnabled()
  expect(
    within(dialog).getByRole('link', { name: 'Install Magpie' })
  ).toBeVisible()
  await act(async () => {
    finishRequest({
      data: { success: true, data: { key: 'full-key' } },
    } as AxiosResponse)
  })
  expect(window.open).not.toHaveBeenCalled()
  fireEvent.click(
    within(dialog).getByRole('button', { name: 'Continue import' })
  )
  const url = new URL(vi.mocked(window.open).mock.calls[0]?.[0] as string)
  expect(url.searchParams.has('models')).toBe(false)
})

test('cancelling during key loading prevents a later response from opening Magpie', async () => {
  let finishRequest!: (response: AxiosResponse) => void
  vi.spyOn(api, 'post').mockReturnValue(
    new Promise<AxiosResponse>((resolve) => {
      finishRequest = resolve
    })
  )
  const user = userEvent.setup()
  const trigger = renderCell()
  await user.click(trigger)
  const dialog = await screen.findByRole('alertdialog')
  await user.click(within(dialog).getByRole('button', { name: 'Cancel' }))
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
  await act(async () => {
    finishRequest({
      data: { success: true, data: { key: 'full-key' } },
    } as AxiosResponse)
  })
  expect(window.open).not.toHaveBeenCalled()
  await waitFor(() => expect(trigger).toHaveFocus())
})

test('a failed key lookup permits retry and still requires confirmation after retry succeeds', async () => {
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValueOnce({
      data: { success: false, message: 'Key unavailable' },
    } as AxiosResponse)
    .mockResolvedValueOnce({
      data: { success: true, data: { key: 'full-key' } },
    } as AxiosResponse)
  fireEvent.click(renderCell())
  const dialog = await screen.findByRole('alertdialog')
  await screen.findByText('Key unavailable')
  fireEvent.click(within(dialog).getByRole('button', { name: 'Retry' }))
  const confirm = await within(dialog).findByRole('button', {
    name: 'Continue import',
  })
  expect(post).toHaveBeenCalledTimes(2)
  expect(window.open).not.toHaveBeenCalled()
  fireEvent.click(confirm)
  expect(window.open).toHaveBeenCalledTimes(1)
})

test('an invalid API address shows an error and keeps installation guidance open', async () => {
  vi.spyOn(api, 'post').mockResolvedValue({
    data: { success: true, data: { key: 'full-key' } },
  } as AxiosResponse)
  fireEvent.click(
    renderCell({ serverAddress: 'https://relay.example?query=value' })
  )
  const dialog = await screen.findByRole('alertdialog')
  fireEvent.click(
    await within(dialog).findByRole('button', { name: 'Continue import' })
  )
  await screen.findByText(
    'Invalid API address. Please contact your administrator.'
  )
  expect(dialog).toBeVisible()
  expect(window.open).not.toHaveBeenCalled()
})

test('keyboard users can open and confirm the dialog while subsequent imports reuse the cached key', async () => {
  const post = vi.spyOn(api, 'post').mockResolvedValue({
    data: { success: true, data: { key: 'full-key' } },
  } as AxiosResponse)
  const user = userEvent.setup()
  const trigger = renderCell()
  trigger.focus()
  await user.keyboard('{Enter}')
  let dialog = await screen.findByRole('alertdialog')
  const confirm = await within(dialog).findByRole('button', {
    name: 'Continue import',
  })
  confirm.focus()
  await user.keyboard('{Enter}')
  expect(window.open).toHaveBeenCalledTimes(1)
  await waitFor(() =>
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  )
  trigger.focus()
  await user.keyboard(' ')
  dialog = await screen.findByRole('alertdialog')
  await user.click(
    within(dialog).getByRole('button', { name: 'Continue import' })
  )
  expect(window.open).toHaveBeenCalledTimes(2)
  expect(post).toHaveBeenCalledTimes(1)
})
