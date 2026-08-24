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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'

import { DingTalkSection } from '../dingtalk-section'

const updateMock = vi.hoisted(() => ({
  mutateAsync: vi.fn(),
}))

vi.mock('../../hooks/use-update-option', () => ({
  useUpdateOption: () => ({
    mutateAsync: updateMock.mutateAsync,
    isPending: false,
  }),
}))

test('saves complete DingTalk settings before enabling login', async () => {
  const user = userEvent.setup()
  render(
    <DingTalkSection
      serverAddress='https://api.example.com/'
      defaultValues={{
        'dingtalk.enabled': false,
        'dingtalk.corp_id': '',
        'dingtalk.client_id': '',
        'dingtalk.client_secret': '',
        'dingtalk.robot_code': '',
      }}
    />
  )

  expect(
    screen.getByText('https://api.example.com/oauth/dingtalk')
  ).toBeInTheDocument()
  await user.type(screen.getByLabelText('Corp ID'), 'corp-1')
  await user.type(screen.getByLabelText('Client ID / AppKey'), 'app-key')
  await user.type(
    screen.getByLabelText('Client Secret / AppSecret'),
    'app-secret'
  )
  await user.type(screen.getByLabelText('Robot Code'), 'robot-code')
  await user.click(screen.getByRole('switch', { name: 'DingTalk login' }))
  await user.click(screen.getByRole('button', { name: 'Save' }))

  expect(updateMock.mutateAsync.mock.calls).toEqual([
    [{ key: 'dingtalk.corp_id', value: 'corp-1' }],
    [{ key: 'dingtalk.client_id', value: 'app-key' }],
    [{ key: 'dingtalk.client_secret', value: 'app-secret' }],
    [{ key: 'dingtalk.robot_code', value: 'robot-code' }],
    [{ key: 'dingtalk.enabled', value: true }],
  ])
})
