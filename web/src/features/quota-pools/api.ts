import { api } from '@/lib/api'

import type {
  ApiResponse,
  PageData,
  QuotaPool,
  QuotaPoolAdminContact,
  QuotaPoolDirectoryItem,
  QuotaPoolCapabilities,
  QuotaPoolMember,
  QuotaPoolOperationLog,
  QuotaPoolStats,
  QuotaPoolStatsRange,
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
      available_pools: QuotaPoolDirectoryItem[]
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
  range: QuotaPoolStatsRange = { preset: 'rolling_7d' }
) {
  const endpoint = self
    ? '/api/quota_pool/self/stats'
    : `/api/quota_pool/${poolId}/stats`
  const response = await api.get<ApiResponse<QuotaPoolStats>>(endpoint, {
    params: range,
  })
  return response.data
}

export async function exportQuotaPoolStats(
  poolId: number,
  self: boolean,
  range: QuotaPoolStatsRange,
  format: 'markdown' | 'xlsx'
) {
  const endpoint = self
    ? '/api/quota_pool/self/stats/export'
    : `/api/quota_pool/${poolId}/stats/export`
  const response = await api.get<Blob>(endpoint, {
    params: { ...range, format },
    responseType: 'blob',
  })
  const disposition = String(response.headers['content-disposition'] ?? '')
  const encodedFilename = disposition.match(/filename\*=UTF-8''([^;]+)/i)?.[1]
  return {
    blob: response.data,
    filename: encodedFilename
      ? decodeURIComponent(encodedFilename)
      : `quota_pool_stats.${format === 'xlsx' ? 'xlsx' : 'md'}`,
  }
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
  amount: number,
  self = false
) {
  const endpoint = self
    ? `/api/quota_pool/self/members/${userId}/recharge`
    : `/api/quota_pool/${poolId}/members/${userId}/recharge`
  const response = await api.post<ApiResponse>(endpoint, { amount })
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

export async function removeQuotaPoolMember(
  poolId: number,
  userId: number,
  self = false
) {
  const endpoint = self
    ? `/api/quota_pool/self/members/${userId}`
    : `/api/quota_pool/${poolId}/members/${userId}`
  const response = await api.delete<ApiResponse>(endpoint)
  return response.data
}

export async function setQuotaPoolAdmin(poolId: number, userId: number) {
  const response = await api.post<ApiResponse>(
    `/api/quota_pool/${poolId}/admins`,
    {
      user_id: userId,
    }
  )
  return response.data
}

export async function revokeQuotaPoolAdmin(poolId: number, userId: number) {
  const response = await api.delete<ApiResponse>(
    `/api/quota_pool/${poolId}/admins/${userId}`
  )
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
