import { api } from '@/lib/api'

import type {
  ApiResponse,
  PageData,
  QuotaPool,
  QuotaPoolAdminContact,
  QuotaPoolCapabilities,
  QuotaPoolMember,
  QuotaPoolOperationLog,
  QuotaPoolStats,
  QuotaPoolStatsPeriod,
  QuotaPoolTransaction,
} from './types'

export async function getQuotaPools(options?: {
  page?: number
  pageSize?: number
  keyword?: string
}) {
  const endpoint = '/api/quota_pool/'
  type QuotaPoolListResponse = ApiResponse<{
    items: QuotaPool[]
    capabilities: QuotaPoolCapabilities
    total?: number
    page?: number
    page_size?: number
  }>
  const response = options
    ? await api.get<QuotaPoolListResponse>(endpoint, {
        params: {
          p: options.page ?? 1,
          page_size: options.pageSize ?? 20,
          keyword: options.keyword ?? '',
        },
      })
    : await api.get<QuotaPoolListResponse>(endpoint)
  return response.data
}

export async function getQuotaPool(poolId: number) {
  const response = await api.get<
    ApiResponse<{
      pool: QuotaPool
      capabilities: QuotaPoolCapabilities
    }>
  >(`/api/quota_pool/${poolId}`)
  return response.data
}

export async function getSelfQuotaPool() {
  const response = await api.get<
    ApiResponse<{
      pool: QuotaPool
      capabilities: QuotaPoolCapabilities
      admin_contacts: QuotaPoolAdminContact[]
    }>
  >('/api/quota_pool/self/')
  return response.data
}

export async function getQuotaPoolMembers(
  poolId: number,
  self = false,
  options: { page?: number; pageSize?: number; keyword?: string } = {}
) {
  const endpoint = self
    ? '/api/quota_pool/self/members'
    : `/api/quota_pool/${poolId}/members`
  const response = await api.get<ApiResponse<PageData<QuotaPoolMember>>>(
    endpoint,
    {
      params: {
        p: options.page ?? 1,
        page_size: options.pageSize ?? 20,
        keyword: options.keyword ?? '',
      },
    }
  )
  return response.data
}

export async function getQuotaPoolCandidates(
  self = false,
  options: { page?: number; pageSize?: number; keyword?: string } = {}
) {
  const endpoint = self
    ? '/api/quota_pool/self/candidates'
    : '/api/quota_pool/candidates'
  const response = await api.get<ApiResponse<PageData<QuotaPoolMember>>>(
    endpoint,
    {
      params: {
        p: options.page ?? 1,
        page_size: options.pageSize ?? 20,
        keyword: options.keyword ?? '',
      },
    }
  )
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

export async function getQuotaPoolStats(
  poolId: number,
  self = false,
  period: QuotaPoolStatsPeriod = 'week'
) {
  const endpoint = self
    ? '/api/quota_pool/self/stats'
    : `/api/quota_pool/${poolId}/stats`
  const response = await api.get<ApiResponse<QuotaPoolStats>>(endpoint, {
    params: { period },
  })
  return response.data
}

export async function getQuotaPoolOperationLogs(poolId: number, self = false) {
  const endpoint = self
    ? '/api/quota_pool/self/operation_logs'
    : `/api/quota_pool/${poolId}/operation_logs`
  const response =
    await api.get<ApiResponse<PageData<QuotaPoolOperationLog>>>(endpoint)
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

export async function moveUserQuotaPool(userId: number, poolId: number) {
  const response = await api.put<ApiResponse>(
    `/api/quota_pool/users/${userId}`,
    { pool_id: poolId }
  )
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
  amount: number,
  self = false
) {
  const endpoint = self
    ? `/api/quota_pool/self/members/${userId}/reclaim`
    : `/api/quota_pool/${poolId}/members/${userId}/reclaim`
  const response = await api.post<ApiResponse>(endpoint, { amount })
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
