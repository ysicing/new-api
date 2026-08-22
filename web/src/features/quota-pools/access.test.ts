/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { describe, expect, test } from 'vitest'

import { canAccessQuotaPools, canListAllQuotaPools } from './access'

describe('quota pool access', () => {
  test('all users can access quota pool entry when feature is enabled', () => {
    expect(canAccessQuotaPools({ role: 100, quota_pool_enabled: true })).toBe(
      true
    )
    expect(canAccessQuotaPools({ role: 2, quota_pool_enabled: true })).toBe(
      true
    )
    expect(canAccessQuotaPools({ role: 10, quota_pool_enabled: true })).toBe(
      true
    )
    expect(
      canAccessQuotaPools({
        role: 1,
        quota_pool_enabled: true,
        quota_pool_id: 0,
        quota_pool_admin: { pool_id: 7, level: 1 },
      })
    ).toBe(true)
  })

  test('rejects all users when feature is disabled', () => {
    expect(canAccessQuotaPools({ role: 1, quota_pool_enabled: false })).toBe(
      false
    )
    expect(canAccessQuotaPools({ role: 1 })).toBe(false)
  })

  test('system and quota pool administrators can list all quota pools', () => {
    expect(canListAllQuotaPools({ role: 2 })).toBe(true)
    expect(canListAllQuotaPools({ role: 10 })).toBe(true)
    expect(canListAllQuotaPools({ role: 100 })).toBe(true)
    expect(canListAllQuotaPools({ role: 1 })).toBe(false)
  })
})
