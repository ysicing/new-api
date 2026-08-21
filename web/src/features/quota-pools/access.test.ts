/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { describe, expect, test } from 'vitest'

import { canAccessQuotaPools } from './access'

describe('quota pool access', () => {
  test('allows system and pool administrators when the feature is enabled', () => {
    expect(canAccessQuotaPools({ role: 100, quota_pool_enabled: true })).toBe(
      true
    )
    expect(canAccessQuotaPools({ role: 2, quota_pool_enabled: true })).toBe(
      true
    )
    expect(
      canAccessQuotaPools({
        role: 1,
        quota_pool_enabled: true,
        quota_pool_admin: { pool_id: 7, level: 1 },
      })
    ).toBe(true)
  })

  test('rejects ordinary users and disabled installations', () => {
    expect(canAccessQuotaPools({ role: 1, quota_pool_enabled: true })).toBe(
      false
    )
    expect(canAccessQuotaPools({ role: 100, quota_pool_enabled: false })).toBe(
      false
    )
  })
})
