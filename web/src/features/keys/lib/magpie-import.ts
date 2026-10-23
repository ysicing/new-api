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
  tokenId: number
  tokenName?: string
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
  if (!key || !Number.isSafeInteger(options.tokenId) || options.tokenId <= 0) {
    return null
  }

  const baseUrl = address.href.replace(/\/+$/, '').replace(/\/v1$/, '')
  // 用公开服务地址划分命名空间，避免不同部署的同号密钥互相替换。
  // ID 不含密钥内容，也不依赖可修改的系统名称和密钥名称；同步计算保留点击手势。
  let serverHash = 0xcbf29ce484222325n
  for (const char of baseUrl) {
    serverHash = BigInt.asUintN(
      64,
      (serverHash ^ BigInt(char.charCodeAt(0))) * 0x100000001b3n
    )
  }
  const systemName = options.name.trim() || 'New API'
  const tokenName = options.tokenName?.trim()
  const providerName = tokenName ? `${systemName} - ${tokenName}` : systemName
  const nameSuffix = ` (#${options.tokenId})`
  const params = new URLSearchParams({
    id: `new-api-${serverHash.toString(16)}-${options.tokenId}`,
    name: `${[...providerName].slice(0, 80 - nameSuffix.length).join('')}${nameSuffix}`,
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
  // 直接交给本机 Magpie，避免导入流程依赖官网可访问。
  return `magpie://import?${params.toString()}`
}
