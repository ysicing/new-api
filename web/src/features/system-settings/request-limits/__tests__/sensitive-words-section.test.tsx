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
import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { SensitiveWordsSection } from '../sensitive-words-section'

const updateMock = vi.hoisted(() => ({
  mutateAsync: vi.fn(),
}))

vi.mock('../../hooks/use-update-option', () => ({
  useUpdateOption: () => ({
    mutateAsync: updateMock.mutateAsync,
    isPending: false,
  }),
}))

vi.mock('../../components/settings-page-context', () => ({
  useSuppressSettingsSectionHeader: () => false,
  SettingsPageFormActions: (props: {
    onSave: () => void
    saveLabel?: string
  }) => (
    <button type='button' onClick={props.onSave}>
      {props.saveLabel ?? 'Save'}
    </button>
  ),
}))

beforeEach(() => {
  updateMock.mutateAsync.mockReset()
  updateMock.mutateAsync.mockResolvedValue({ success: true })
})

function renderSection() {
  return render(
    <SensitiveWordsSection
      defaultValues={{
        CheckSensitiveEnabled: true,
        CheckSensitiveOnPromptEnabled: true,
        SensitiveWords: 'blocked-word',
        SensitiveWordContactMessage: '',
      }}
    />
  )
}

test('saves the configurable false-positive handling message', async () => {
  const user = userEvent.setup()
  renderSection()

  await user.type(
    screen.getByLabelText('False-positive handling message'),
    'If this is a false positive, contact Zhang San on DingTalk.'
  )
  await user.click(screen.getByRole('button', { name: 'Save sensitive words' }))

  expect(updateMock.mutateAsync).toHaveBeenCalledWith({
    key: 'SensitiveWordContactMessage',
    value: 'If this is a false positive, contact Zhang San on DingTalk.',
  })
})

test('rejects a false-positive handling message longer than 500 characters', async () => {
  renderSection()

  fireEvent.change(screen.getByLabelText('False-positive handling message'), {
    target: { value: 'x'.repeat(501) },
  })
  await userEvent.click(
    screen.getByRole('button', { name: 'Save sensitive words' })
  )

  expect(
    await screen.findByText('Message must be 500 characters or fewer')
  ).toBeInTheDocument()
  expect(updateMock.mutateAsync).not.toHaveBeenCalled()
})
