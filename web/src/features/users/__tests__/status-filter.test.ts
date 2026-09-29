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
