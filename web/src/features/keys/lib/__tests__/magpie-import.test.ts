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

import { buildMagpieImportUrl } from '../magpie-import'

test.each([
  ['https://relay.example', 'https://relay.example'],
  ['https://relay.example/v1/', 'https://relay.example'],
  ['https://relay.example/gateway/v1', 'https://relay.example/gateway'],
  ['http://localhost:3000/', 'http://localhost:3000'],
])(
  'importing from %s provides the three native API base URLs',
  (address, base) => {
    const result = buildMagpieImportUrl({
      name: 'iCode',
      serverAddress: address,
      apiKey: 'sk-example',
    })
    expect(result).not.toBeNull()
    const url = new URL(result ?? '')
    expect(url.protocol).toBe('magpie:')
    expect(url.host).toBe('import')
    expect(url.hash).toBe('')
    const params = url.searchParams
    expect(params.get('chat')).toBe(`${base}/v1`)
    expect(params.get('responses')).toBe(`${base}/v1`)
    expect(params.get('anthropic')).toBe(base)
    expect(params.get('key')).toBe('sk-example')
    expect(params.has('models')).toBe(false)
  }
)

test('native import encodes special characters without injecting parameters or fragments', () => {
  const key = 'sk-test&name=other?#密钥'
  const result = buildMagpieImportUrl({
    name: '研发 & 测试',
    serverAddress: 'https://relay.example',
    apiKey: key,
    modelLimits: 'gpt-6-sol, claude-sonnet-5, gpt-6-sol,',
  })
  const url = new URL(result ?? '')
  const params = url.searchParams
  expect(url.protocol).toBe('magpie:')
  expect(url.host).toBe('import')
  expect(url.hash).toBe('')
  expect(params.get('key')).toBe(key)
  expect(params.get('name')).toBe('研发 & 测试')
  expect(params.get('models')).toBe('gpt-6-sol,claude-sonnet-5')
})

test('an unprefixed key is normalized and provider names respect the 80-character limit', () => {
  const result = buildMagpieImportUrl({
    name: 'a'.repeat(81),
    serverAddress: 'https://relay.example',
    apiKey: 'example',
  })
  const params = new URL(result ?? '').searchParams
  expect(params.get('key')).toBe('sk-example')
  expect(params.get('name')).toBe('a'.repeat(80))
})

test.each([
  'not-a-url',
  'javascript:alert(1)',
  'file:///tmp/relay',
  'https://user:password@relay.example',
  'https://relay.example?key=example',
  'https://relay.example#fragment',
  'https://relay.example?',
  'https://relay.example#',
])(
  'invalid API address %s does not produce an import link',
  (serverAddress) => {
    expect(
      buildMagpieImportUrl({
        name: 'iCode',
        serverAddress,
        apiKey: 'sk-example',
      })
    ).toBeNull()
  }
)

test('an empty key does not produce an import link', () => {
  expect(
    buildMagpieImportUrl({
      name: 'iCode',
      serverAddress: 'https://relay.example',
      apiKey: ' ',
    })
  ).toBeNull()
})
