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

import { DingTalkNotificationsSection } from '../dingtalk-notifications-section'

const apiMocks = vi.hoisted(() => ({
  listDingTalkNotifications: vi.fn(),
}))

vi.mock('../../api', () => apiMocks)

beforeEach(() => {
  apiMocks.listDingTalkNotifications.mockReset()
  apiMocks.listDingTalkNotifications.mockResolvedValue({
    success: true,
    data: {
      page: 1,
      page_size: 20,
      total: 2,
      items: [
        {
          id: 2,
          event_type: 'new_user_quota_exhausted',
          dedupe_key: 'new_user_quota_exhausted:2',
          user_id: 2,
          username: 'alice',
          recipient: 'alice',
          title: '体验额度已用完',
          content: '当前体验额度已经用完',
          status: 'succeeded',
          error: '',
          metadata: '{"pool_id":7}',
          sent_at: 200,
          created_at: 190,
          updated_at: 200,
        },
        {
          id: 1,
          event_type: 'new_user_quota_exhausted',
          dedupe_key: 'new_user_quota_exhausted:1',
          user_id: 1,
          username: 'bob',
          recipient: 'bob',
          title: '体验额度已用完',
          content: '当前体验额度已经用完',
          status: 'failed',
          error: 'invalid staff id',
          metadata: '{}',
          sent_at: 100,
          created_at: 90,
          updated_at: 100,
        },
      ],
    },
  })
})

test('lists DingTalk delivery outcomes and applies operations filters', async () => {
  const user = userEvent.setup()
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <DingTalkNotificationsSection />
    </QueryClientProvider>
  )

  expect(await screen.findAllByText('alice')).toHaveLength(2)
  expect(screen.getByText('invalid staff id')).toBeInTheDocument()
  expect(screen.getByText('{"pool_id":7}')).toBeInTheDocument()
  expect(screen.getAllByText('体验额度已用完')).toHaveLength(2)
  expect(screen.getAllByText(/Sent at/)).toHaveLength(2)

  await user.selectOptions(screen.getByRole('combobox', { name: 'Status' }), [
    'failed',
  ])
  await user.type(
    screen.getByRole('searchbox', { name: 'User or recipient' }),
    'alice'
  )
  await user.click(screen.getByRole('button', { name: 'Search' }))

  await waitFor(() => {
    expect(apiMocks.listDingTalkNotifications).toHaveBeenLastCalledWith(
      expect.objectContaining({ status: 'failed', keyword: 'alice' })
    )
  })
})
