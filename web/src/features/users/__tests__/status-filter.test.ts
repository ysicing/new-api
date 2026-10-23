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
import { expect, test } from 'vitest'

import { getUserStatusOptions, USER_STATUS } from '../constants'

test('status filter offers frozen users as a separate option', () => {
  const options = getUserStatusOptions((key) => key)

  expect(options).toContainEqual({
    label: 'Quota frozen',
    value: String(USER_STATUS.FROZEN),
  })
  expect(
    options.find((option) => option.value === String(USER_STATUS.ENABLED))
  ).toEqual({ label: 'Enabled', value: '1' })
})
