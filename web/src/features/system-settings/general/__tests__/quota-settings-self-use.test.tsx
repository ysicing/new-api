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
import { expect, test, vi } from 'vitest'

import { QuotaSettingsSection } from '../quota-settings-section'

vi.mock('../../hooks/use-update-option', () => ({
  useUpdateOption: () => ({ mutateAsync: vi.fn(), isPending: false }),
}))
vi.mock('../../components/form-navigation-guard', () => ({
  FormNavigationGuard: () => null,
}))
vi.mock('../../components/settings-page-context', () => ({
  useSuppressSettingsSectionHeader: () => false,
  SettingsPageFormActions: () => null,
}))

const defaultValues = {
  QuotaForNewUser: 1000,
  PreConsumedQuota: 500,
  QuotaForInviter: 200,
  QuotaForInvitee: 100,
  TopUpLink: '',
  general_setting: { docs_link: '' },
  quota_setting: { enable_free_model_pre_consume: true },
}

test('hides invitation reward settings in self-use mode', () => {
  render(
    <QuotaSettingsSection
      defaultValues={defaultValues}
      complianceConfirmed={false}
      selfUseModeEnabled
    />
  )

  expect(screen.getByText('New User Quota')).toBeInTheDocument()
  expect(screen.queryByText('Inviter Reward')).not.toBeInTheDocument()
  expect(screen.queryByText('Invitee Reward')).not.toBeInTheDocument()
  expect(
    screen.queryByText(
      'Non-zero invitation rewards require compliance confirmation in Payment Gateway settings.'
    )
  ).not.toBeInTheDocument()
})

test('keeps invitation reward settings outside self-use mode', () => {
  render(
    <QuotaSettingsSection
      defaultValues={defaultValues}
      complianceConfirmed
      selfUseModeEnabled={false}
    />
  )

  expect(screen.getByText('Inviter Reward')).toBeInTheDocument()
  expect(screen.getByText('Invitee Reward')).toBeInTheDocument()
})
