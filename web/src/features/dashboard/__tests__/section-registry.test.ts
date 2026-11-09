/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { expect, test } from 'vitest'

import {
  DASHBOARD_SECTION_IDS,
  getDashboardSectionNavItems,
} from '../section-registry'

const translate = (key: string) => key

test('model statistics is a dashboard section visible to every user', () => {
  expect(DASHBOARD_SECTION_IDS).toContain('model-statistics')
  expect(
    getDashboardSectionNavItems(translate as never, { isAdmin: false })
  ).toEqual(
    expect.arrayContaining([
      expect.objectContaining({
        title: 'Model statistics',
        url: '/dashboard/model-statistics',
      }),
    ])
  )
})
