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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { DingTalkSection } from '../dingtalk-section'

const updateMock = vi.hoisted(() => ({
  mutateAsync: vi.fn(),
}))

const apiMocks = vi.hoisted(() => ({
  searchDingTalkTestUsers: vi.fn(),
  sendDingTalkTestMessage: vi.fn(),
  sendDingTalkAnnouncementGroupTestMessage: vi.fn(),
}))

const toastMocks = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
}))

vi.mock('../../hooks/use-update-option', () => ({
  useUpdateOption: () => ({
    mutateAsync: updateMock.mutateAsync,
    isPending: false,
  }),
}))

vi.mock('../dingtalk-api', () => apiMocks)

vi.mock('sonner', () => ({ toast: toastMocks }))

beforeEach(() => {
  updateMock.mutateAsync.mockReset()
  updateMock.mutateAsync.mockResolvedValue({ success: true })
  apiMocks.searchDingTalkTestUsers.mockReset()
  apiMocks.searchDingTalkTestUsers.mockResolvedValue({
    success: true,
    data: { items: [], total: 0, page: 1, page_size: 20 },
  })
  apiMocks.sendDingTalkTestMessage.mockReset()
  apiMocks.sendDingTalkAnnouncementGroupTestMessage.mockReset()
  toastMocks.success.mockReset()
  toastMocks.error.mockReset()
})

function renderSection(
  defaultValues = {
    'dingtalk.enabled': false,
    'dingtalk.corp_id': '',
    'dingtalk.client_id': '',
    'dingtalk.client_secret': '',
    'dingtalk.announcement_group_open_conversation_id': '',
  }
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <DingTalkSection
        serverAddress='https://api.example.com/'
        defaultValues={defaultValues}
      />
    </QueryClientProvider>
  )
}

test('saves complete DingTalk settings before enabling login', async () => {
  const user = userEvent.setup()
  renderSection()

  expect(
    screen.getByText('https://api.example.com/oauth/dingtalk')
  ).toBeInTheDocument()
  await user.type(screen.getByLabelText('Corp ID'), 'corp-1')
  await user.type(screen.getByLabelText('Client ID / AppKey'), 'app-key')
  await user.type(
    screen.getByLabelText('Client Secret / AppSecret'),
    'app-secret'
  )
  expect(screen.queryByLabelText('Robot Code')).not.toBeInTheDocument()
  await user.click(screen.getByRole('switch', { name: 'DingTalk login' }))
  await user.click(screen.getByRole('button', { name: 'Save' }))

  expect(updateMock.mutateAsync.mock.calls).toEqual([
    [{ key: 'dingtalk.corp_id', value: 'corp-1' }],
    [{ key: 'dingtalk.client_id', value: 'app-key' }],
    [{ key: 'dingtalk.client_secret', value: 'app-secret' }],
    [{ key: 'dingtalk.enabled', value: true }],
  ])
})

test('saves settings before sending a Bot test to the selected user', async () => {
  apiMocks.searchDingTalkTestUsers.mockResolvedValue({
    success: true,
    data: {
      items: [
        {
          id: 12,
          username: 'alice',
          display_name: 'Alice',
          email: 'alice@example.com',
          department: '研发部',
          dingtalk_bound: false,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    },
  })
  apiMocks.sendDingTalkTestMessage.mockResolvedValue({
    success: true,
    data: { bound_now: true },
  })
  const user = userEvent.setup()
  renderSection({
    'dingtalk.enabled': true,
    'dingtalk.corp_id': 'corp-1',
    'dingtalk.client_id': 'old-key',
    'dingtalk.client_secret': '',
    'dingtalk.announcement_group_open_conversation_id': '',
  })

  const clientId = screen.getByLabelText('Client ID / AppKey')
  await user.clear(clientId)
  await user.type(clientId, 'new-key')
  await user.type(
    screen.getByPlaceholderText(
      'Search user ID, username, display name, email or department'
    ),
    'alice'
  )
  await user.click(await screen.findByRole('option', { name: /alice/ }))
  await user.click(
    screen.getByRole('button', { name: 'Save and send test message' })
  )

  expect(updateMock.mutateAsync).toHaveBeenCalledWith({
    key: 'dingtalk.client_id',
    value: 'new-key',
  })
  expect(apiMocks.sendDingTalkTestMessage).toHaveBeenCalledWith(12)
  expect(updateMock.mutateAsync.mock.invocationCallOrder[0]).toBeLessThan(
    apiMocks.sendDingTalkTestMessage.mock.invocationCallOrder[0]
  )
  expect(toastMocks.success).toHaveBeenCalledWith(
    'DingTalk test message sent successfully'
  )
  expect(toastMocks.success).toHaveBeenCalledWith(
    'DingTalk account automatically bound by email'
  )
})

test('saves the announcement group before sending a group test message', async () => {
  apiMocks.sendDingTalkAnnouncementGroupTestMessage.mockResolvedValue({
    success: true,
  })
  const user = userEvent.setup()
  renderSection({
    'dingtalk.enabled': true,
    'dingtalk.corp_id': 'corp-1',
    'dingtalk.client_id': 'app-key',
    'dingtalk.client_secret': '',
    'dingtalk.announcement_group_open_conversation_id': '',
  })

  await user.type(
    screen.getByLabelText('Announcement group openConversationId'),
    'cid-announcement'
  )
  await user.click(
    screen.getByRole('button', { name: 'Save and send group test message' })
  )

  await waitFor(() => {
    expect(updateMock.mutateAsync).toHaveBeenCalledWith({
      key: 'dingtalk.announcement_group_open_conversation_id',
      value: 'cid-announcement',
    })
  })
  expect(apiMocks.sendDingTalkAnnouncementGroupTestMessage).toHaveBeenCalled()
})

test('shows binding state and debounces DingTalk user search', async () => {
  apiMocks.searchDingTalkTestUsers.mockResolvedValue({
    success: true,
    data: {
      items: [
        {
          id: 12,
          username: 'alice',
          display_name: 'Alice',
          email: 'alice@example.com',
          department: '研发部',
          dingtalk_bound: true,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    },
  })
  const user = userEvent.setup()
  renderSection()

  await user.type(
    screen.getByPlaceholderText(
      'Search user ID, username, display name, email or department'
    ),
    'alice'
  )

  await waitFor(() =>
    expect(apiMocks.searchDingTalkTestUsers).toHaveBeenLastCalledWith('alice')
  )
  expect(await screen.findByText('DingTalk bound')).toBeInTheDocument()
})

test('requires a selected recipient before testing', () => {
  renderSection()

  expect(
    screen.getByRole('button', { name: 'Save and send test message' })
  ).toBeDisabled()
})

test('keeps the selected recipient when test delivery fails', async () => {
  apiMocks.searchDingTalkTestUsers.mockResolvedValue({
    success: true,
    data: {
      items: [
        {
          id: 12,
          username: 'alice',
          display_name: 'Alice',
          email: 'alice@example.com',
          department: '研发部',
          dingtalk_bound: false,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    },
  })
  apiMocks.sendDingTalkTestMessage.mockRejectedValue(
    new Error('DingTalk directory unavailable')
  )
  const user = userEvent.setup()
  renderSection()

  const input = screen.getByPlaceholderText(
    'Search user ID, username, display name, email or department'
  )
  await user.type(input, 'alice')
  await user.click(await screen.findByRole('option', { name: /alice/ }))
  await user.click(
    screen.getByRole('button', { name: 'Save and send test message' })
  )

  await waitFor(() =>
    expect(toastMocks.error).toHaveBeenCalledWith(
      'DingTalk directory unavailable'
    )
  )
  expect(input).toHaveValue(
    'alice (ID:12) — Alice / alice@example.com / 研发部'
  )
})

test('does not send a test message when saving settings fails', async () => {
  apiMocks.searchDingTalkTestUsers.mockResolvedValue({
    success: true,
    data: {
      items: [
        {
          id: 12,
          username: 'alice',
          display_name: 'Alice',
          email: 'alice@example.com',
          department: '研发部',
          dingtalk_bound: true,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    },
  })
  updateMock.mutateAsync.mockResolvedValue({
    success: false,
    message: 'Failed to save settings',
  })
  const user = userEvent.setup()
  renderSection()

  await user.type(screen.getByLabelText('Client ID / AppKey'), 'new-key')
  await user.type(
    screen.getByPlaceholderText(
      'Search user ID, username, display name, email or department'
    ),
    'alice'
  )
  await user.click(await screen.findByRole('option', { name: /alice/ }))
  await user.click(
    screen.getByRole('button', { name: 'Save and send test message' })
  )

  await waitFor(() => expect(updateMock.mutateAsync).toHaveBeenCalled())
  expect(apiMocks.sendDingTalkTestMessage).not.toHaveBeenCalled()
})
