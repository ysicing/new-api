import { describe, expect, it } from 'vitest'

import { isResponseModelMismatch } from '../response-model'

describe('isResponseModelMismatch', () => {
  it.each([
    {
      name: 'matches the requested model',
      observation: {
        requested_model: 'gpt-6-astra',
        upstream_model: 'mapped-gpt-6-astra',
        returned_model: 'gpt-6-astra',
      },
      expected: false,
    },
    {
      name: 'accepts a dated response model',
      observation: {
        requested_model: 'gpt-6-astra',
        upstream_model: 'mapped-gpt-6-astra',
        returned_model: 'gpt-6-astra-20260918',
      },
      expected: false,
    },
    {
      name: 'accepts a provider path response model',
      observation: {
        requested_model: 'gpt-6-astra',
        upstream_model: 'mapped-gpt-6-astra',
        returned_model: 'vendor/gpt-6-astra',
      },
      expected: false,
    },
    {
      name: 'flags a different response model',
      observation: {
        requested_model: 'gpt-6-astra',
        upstream_model: 'mapped-gpt-6-astra',
        returned_model: 'gpt-5.6-luna',
      },
      expected: true,
    },
  ])('$name', ({ observation, expected }) => {
    expect(isResponseModelMismatch(observation)).toBe(expected)
  })
})
