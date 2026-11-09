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
import { useForm } from 'react-hook-form'
import { expect, test, vi } from 'vitest'

import { Form } from '@/components/ui/form'

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  type ChannelFormValues,
  transformFormDataToCreatePayload,
} from '../../lib/channel-form'
import { ChannelMagpieSetting } from '../channel-magpie-setting'

function SettingsForm(props: { onSubmit: (settings: string) => void }) {
  const form = useForm<ChannelFormValues>({
    defaultValues: CHANNEL_FORM_DEFAULT_VALUES,
  })
  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit((values) =>
          props.onSubmit(
            transformFormDataToCreatePayload(values).channel.settings ?? '{}'
          )
        )}
      >
        <ChannelMagpieSetting control={form.control} />
        <button type='submit'>Save</button>
      </form>
    </Form>
  )
}

test('defaults Magpie monitoring off and saves the user selection via mouse and keyboard', async () => {
  const onSubmit = vi.fn()
  const user = userEvent.setup()
  render(<SettingsForm onSubmit={onSubmit} />)
  const toggle = screen.getByRole('switch', {
    name: 'Magpie channel',
  })
  expect(toggle).not.toBeChecked()
  await user.click(screen.getByRole('button', { name: 'Save' }))
  expect(JSON.parse(onSubmit.mock.lastCall?.[0] ?? '{}').magpie_enabled).toBe(
    false
  )
  await user.click(toggle)
  expect(toggle).toBeChecked()
  await user.click(screen.getByRole('button', { name: 'Save' }))
  expect(JSON.parse(onSubmit.mock.lastCall?.[0] ?? '{}').magpie_enabled).toBe(
    true
  )
  toggle.focus()
  await user.keyboard(' ')
  expect(toggle).not.toBeChecked()
  await user.click(screen.getByRole('button', { name: 'Save' }))
  expect(JSON.parse(onSubmit.mock.lastCall?.[0] ?? '{}').magpie_enabled).toBe(
    false
  )
})
