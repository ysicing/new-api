/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { renderHook } from '@testing-library/react'
import { expect, test } from 'vitest'

import { useUsersColumns } from '../users-columns'

test('self-use mode hides invitation information in the user list', () => {
  const { result } = renderHook(() => useUsersColumns(false))
  const columnIds = result.current.map((column) => {
    if (column.id) return column.id
    if ('accessorKey' in column) return String(column.accessorKey)
    return undefined
  })

  expect(columnIds).not.toContain('invite_info')
})

test('ordinary mode preserves upstream invitation information', () => {
  const { result } = renderHook(() => useUsersColumns())
  expect(result.current.some((column) => column.id === 'invite_info')).toBe(
    true
  )
})
