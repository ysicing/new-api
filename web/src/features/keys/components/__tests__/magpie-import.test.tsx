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
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
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
let target: {
  opener: Window | null
  closed: boolean
  close: ReturnType<typeof vi.fn>
  location: { replace: ReturnType<typeof vi.fn> }
}

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
  target = {
    opener: window,
    closed: false,
    close: vi.fn(),
    location: { replace: vi.fn() },
  }
  vi.spyOn(window, 'open').mockReturnValue(target as unknown as Window)
})

afterEach(() => {
  queryClient.clear()
  toast.dismiss()
  useSystemConfigStore.setState({ config: originalConfig })
})

test('the key cell exposes a Magpie icon without fetching the full key until clicked', async () => {
  const post = vi.spyOn(api, 'post').mockResolvedValue({
    data: { success: true, data: { key: 'full-key' } },
  } as AxiosResponse)
  const action = renderCell()
  expect(screen.getByText('sk-masked-********')).toBeVisible()
  expect(action).toBeEnabled()
  expect(action.querySelector('svg')).toHaveAttribute('aria-hidden', 'true')
  expect(action).toHaveClass('shrink-0')
  expect(post).not.toHaveBeenCalled()
  fireEvent.click(action)
  await waitFor(() => expect(target.location.replace).toHaveBeenCalled())

  expect(post).toHaveBeenCalledWith('/api/token/7/key')
  expect(window.open).toHaveBeenCalledWith('about:blank', '_blank')
  expect(target.opener).toBeNull()
  const url = new URL(target.location.replace.mock.calls[0]?.[0] as string)
  expect(url.search).toBe('')
  const params = new URLSearchParams(url.hash.slice(1))
  expect(params.get('name')).toBe('iCode')
  expect(params.get('key')).toBe('sk-full-key')
  expect(params.get('chat')).toBe('https://relay.example/v1')
  expect(params.get('responses')).toBe('https://relay.example/v1')
  expect(params.get('anthropic')).toBe('https://relay.example')
  expect(params.get('models')).toBe('gpt-6-sol,claude-sonnet-5')
})

test('import opens a window before awaiting the key and disables repeated clicks while loading', async () => {
  let finishRequest!: (response: AxiosResponse) => void
  const post = vi.spyOn(api, 'post').mockReturnValue(
    new Promise<AxiosResponse>((resolve) => {
      finishRequest = resolve
    })
  )
  const action = renderCell({ modelLimitsEnabled: false })
  fireEvent.click(action)
  expect(window.open).toHaveBeenCalledWith('about:blank', '_blank')
  expect(target.location.replace).not.toHaveBeenCalled()
  expect(action).toBeDisabled()
  fireEvent.click(action)
  expect(window.open).toHaveBeenCalledTimes(1)
  expect(post).toHaveBeenCalledTimes(1)
  await act(async () => {
    finishRequest({
      data: { success: true, data: { key: 'full-key' } },
    } as AxiosResponse)
  })
  expect(action).toBeEnabled()
  const url = new URL(target.location.replace.mock.calls[0]?.[0] as string)
  expect(new URLSearchParams(url.hash.slice(1)).has('models')).toBe(false)
})

test('a failed key lookup closes the temporary window and allows retry', async () => {
  vi.spyOn(api, 'post').mockResolvedValue({
    data: { success: false, message: 'Key unavailable' },
  } as AxiosResponse)
  const action = renderCell()
  fireEvent.click(action)
  await screen.findByText('Key unavailable')
  await waitFor(() => expect(target.close).toHaveBeenCalled())
  expect(action).toBeEnabled()
  expect(target.location.replace).not.toHaveBeenCalled()
})

test('an invalid API address closes the temporary window and shows an error', async () => {
  vi.spyOn(api, 'post').mockResolvedValue({
    data: { success: true, data: { key: 'full-key' } },
  } as AxiosResponse)
  const action = renderCell({
    serverAddress: 'https://relay.example?query=value',
  })
  fireEvent.click(action)
  await screen.findByText(
    'Invalid API address. Please contact your administrator.'
  )
  expect(target.close).toHaveBeenCalled()
  expect(target.location.replace).not.toHaveBeenCalled()
})

test('a blocked pop-up shows guidance without fetching the key', async () => {
  const post = vi.spyOn(api, 'post')
  vi.mocked(window.open).mockReturnValue(null)
  const action = renderCell()
  fireEvent.click(action)
  await screen.findByText('Pop-up blocked. Please allow pop-ups and try again.')
  expect(post).not.toHaveBeenCalled()
})

test('the Magpie icon supports keyboard activation and reuses the cached key', async () => {
  const post = vi.spyOn(api, 'post').mockResolvedValue({
    data: { success: true, data: { key: 'full-key' } },
  } as AxiosResponse)
  const user = userEvent.setup()
  const action = renderCell()
  action.focus()
  expect(action).toHaveFocus()
  await user.keyboard('{Enter}')
  await waitFor(() => expect(target.location.replace).toHaveBeenCalledTimes(1))
  await user.keyboard(' ')
  await waitFor(() => expect(target.location.replace).toHaveBeenCalledTimes(2))
  expect(post).toHaveBeenCalledTimes(1)
})

test('closing the temporary window during key loading cancels navigation', async () => {
  let finishRequest!: (response: AxiosResponse) => void
  vi.spyOn(api, 'post').mockReturnValue(
    new Promise<AxiosResponse>((resolve) => {
      finishRequest = resolve
    })
  )
  const action = renderCell()
  fireEvent.click(action)
  target.closed = true
  await act(async () => {
    finishRequest({
      data: { success: true, data: { key: 'full-key' } },
    } as AxiosResponse)
  })
  expect(target.location.replace).not.toHaveBeenCalled()
})
