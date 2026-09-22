/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { expect, test } from 'vitest'

import {
  CHANNEL_TYPES,
  CHANNEL_TYPE_TYPESAFE,
  MODEL_FETCHABLE_TYPES,
} from '../../constants'
import { getChannelTypeConfig } from '../channel-type-config'
import { getChannelTypeIcon } from '../channel-utils'

test('exposes TypeSafe Jev as a model-fetchable channel provider', () => {
  expect(CHANNEL_TYPES[CHANNEL_TYPE_TYPESAFE]).toBe('TypeSafe')
  expect(MODEL_FETCHABLE_TYPES.has(CHANNEL_TYPE_TYPESAFE)).toBe(true)
  expect(getChannelTypeConfig(CHANNEL_TYPE_TYPESAFE)).toMatchObject({
    id: CHANNEL_TYPE_TYPESAFE,
    icon: 'TypeSafe',
    supportedModels: ['jev-1.13.0', 'jev-latest', 'jev-preview'],
  })
  expect(getChannelTypeIcon(CHANNEL_TYPE_TYPESAFE)).toBe('TypeSafe')
})
