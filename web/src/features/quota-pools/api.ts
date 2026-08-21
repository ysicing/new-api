import { api } from '@/lib/api'

import type {
  ApiResponse,
  PageData,
  QuotaPool,
  QuotaPoolCapabilities,
  QuotaPoolMember,
  QuotaPoolStats,
  QuotaPoolTransaction,
} from './types'

export async function getQuotaPools() {
  const response = await api.get<
    ApiResponse<{
      items: QuotaPool[]
      capabilities: QuotaPoolCapabilities
    }>
  >('/api/quota_pool/')
  return response.data
}

export async function getSelfQuotaPool() {
  const response = await api.get<
    ApiResponse<{
      pool: QuotaPool
      capabilities: QuotaPoolCapabilities
    }>
  >('/api/quota_pool/self/')
  return response.data
}

export async function getQuotaPoolMembers(poolId: number, self = false) {
  const endpoint = self
    ? '/api/quota_pool/self/members'
    : `/api/quota_pool/${poolId}/members`
  const response =
    await api.get<ApiResponse<PageData<QuotaPoolMember>>>(endpoint)
  return response.data
}

export async function getQuotaPoolTransactions(poolId: number, self = false) {
  const endpoint = self
    ? '/api/quota_pool/self/transactions'
    : `/api/quota_pool/${poolId}/transactions`
  const response =
    await api.get<ApiResponse<PageData<QuotaPoolTransaction>>>(endpoint)
  return response.data
}

export async function getQuotaPoolStats(poolId: number, self = false) {
  const endpoint = self
    ? '/api/quota_pool/self/stats'
    : `/api/quota_pool/${poolId}/stats`
  const response = await api.get<ApiResponse<QuotaPoolStats>>(endpoint)
  return response.data
}

export async function createQuotaPool(input: {
  name: string
  base_quota: number
}) {
  const response = await api.post<ApiResponse<QuotaPool>>(
    '/api/quota_pool/',
    input
  )
  return response.data
}

export async function refillQuotaPool(poolId: number, amount: number) {
  const response = await api.post<ApiResponse>(
    `/api/quota_pool/${poolId}/refill`,
    { amount }
  )
  return response.data
}

export async function addQuotaPoolMember(
  poolId: number,
  userId: number,
  self = false
) {
  const endpoint = self
    ? '/api/quota_pool/self/members'
    : `/api/quota_pool/${poolId}/members`
  const response = await api.post<ApiResponse>(endpoint, { user_id: userId })
  return response.data
}

export async function rechargeQuotaPoolMember(
  poolId: number,
  userId: number,
  self = false
) {
  const endpoint = self
    ? `/api/quota_pool/self/members/${userId}/recharge`
    : `/api/quota_pool/${poolId}/members/${userId}/recharge`
  const response = await api.post<ApiResponse>(endpoint)
  return response.data
}

export async function reclaimQuotaPoolMember(
  poolId: number,
  userId: number,
  self = false
) {
  const endpoint = self
    ? `/api/quota_pool/self/members/${userId}/reclaim`
    : `/api/quota_pool/${poolId}/members/${userId}/reclaim`
  const response = await api.post<ApiResponse>(endpoint)
  return response.data
}

export async function setQuotaPoolAdmin(
  poolId: number,
  userId: number,
  level: 1 | 2,
  self = false
) {
  const endpoint = self
    ? '/api/quota_pool/self/admins'
    : `/api/quota_pool/${poolId}/admins`
  const response = await api.post<ApiResponse>(endpoint, {
    user_id: userId,
    level,
  })
  return response.data
}

export async function revokeQuotaPoolAdmin(
  poolId: number,
  userId: number,
  self = false
) {
  const endpoint = self
    ? `/api/quota_pool/self/admins/${userId}`
    : `/api/quota_pool/${poolId}/admins/${userId}`
  const response = await api.delete<ApiResponse>(endpoint)
  return response.data
}

export async function updateQuotaPool(
  poolId: number,
  values: Record<string, number | boolean | string>,
  self = false
) {
  const endpoint = self ? '/api/quota_pool/self/' : `/api/quota_pool/${poolId}`
  const response = await api.put<ApiResponse>(endpoint, values)
  return response.data
}
