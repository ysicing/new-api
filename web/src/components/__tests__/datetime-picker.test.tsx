/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'

import { DateTimePicker } from '../datetime-picker'

test('provides distinct accessible names for date, time, and clear controls', () => {
  render(
    <DateTimePicker
      value={new Date(2026, 8, 3, 12, 30)}
      dateAriaLabel='Start date'
      timeAriaLabel='Start time'
      clearAriaLabel='Clear start time'
    />
  )

  expect(screen.getByRole('button', { name: 'Start date' })).toBeInTheDocument()
  expect(screen.getByLabelText('Start time')).toHaveValue('12:30')
  expect(
    screen.getByRole('button', { name: 'Clear start time' })
  ).toBeInTheDocument()
})
