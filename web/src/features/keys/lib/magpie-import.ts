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
export function buildMagpieImportUrl(options: {
  name: string
  serverAddress: string
  apiKey: string
  modelLimits?: string
}): string | null {
  let address: URL
  try {
    address = new URL(options.serverAddress.trim())
  } catch {
    return null
  }
  // 本机/内网 HTTP 的范围由 Magpie 校验；地址不能夹带认证或额外请求参数。
  if (
    !['https:', 'http:'].includes(address.protocol) ||
    address.username ||
    address.password ||
    address.href.includes('?') ||
    address.href.includes('#')
  ) {
    return null
  }
  const key = options.apiKey.trim()
  if (!key) return null

  const baseUrl = address.href.replace(/\/+$/, '').replace(/\/v1$/, '')
  const params = new URLSearchParams({
    name: options.name.trim().slice(0, 80) || 'New API',
    chat: `${baseUrl}/v1`,
    responses: `${baseUrl}/v1`,
    anthropic: baseUrl,
    key: key.startsWith('sk-') ? key : `sk-${key}`,
  })
  const models = [
    ...new Set(
      (options.modelLimits ?? '')
        .split(',')
        .map((model) => model.trim())
        .filter(Boolean)
    ),
  ]
  if (models.length > 0) params.set('models', models.join(','))
  // 密钥只放在 fragment 中，避免进入 Magpie 网站的请求和访问日志。
  return `https://usemagpie.ai/import#${params.toString()}`
}
