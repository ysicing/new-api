import { api } from '@/lib/api'
import { requireServerSuccess } from '@/lib/server-error-message'

import type {
  ApiResponse,
  AssignableQuotaPool,
  BudgetStats,
  BudgetTag,
} from './types'

export async function listQuotaPoolBudgetTags() {
  const response = await api.get<ApiResponse<BudgetTag[]>>(
    '/api/quota_pool/budget_tags'
  )
  return requireServerSuccess(response.data)
}

export async function getQuotaPoolBudgetStats(params: {
  startMonth: string
  endMonth: string
}) {
  const response = await api.get<ApiResponse<BudgetStats>>(
    '/api/quota_pool/budget_stats',
    {
      params: {
        start_month: params.startMonth,
        end_month: params.endMonth,
      },
    }
  )
  return requireServerSuccess(response.data)
}

export async function listAssignableQuotaPools() {
  const response =
    await api.get<ApiResponse<{ items: AssignableQuotaPool[] }>>(
      '/api/quota_pool/'
    )
  return requireServerSuccess(response.data)
}

export async function createQuotaPoolBudgetTag(name: string) {
  const response = await api.post<ApiResponse<BudgetTag>>(
    '/api/quota_pool/budget_tags',
    { name }
  )
  return requireServerSuccess(response.data)
}

export async function updateQuotaPoolBudgetTag(id: number, name: string) {
  const response = await api.put<ApiResponse<BudgetTag>>(
    `/api/quota_pool/budget_tags/${id}`,
    { name }
  )
  return requireServerSuccess(response.data)
}

export async function deleteQuotaPoolBudgetTag(id: number) {
  const response = await api.delete<ApiResponse<null>>(
    `/api/quota_pool/budget_tags/${id}`
  )
  return requireServerSuccess(response.data)
}

export async function replaceQuotaPoolBudgetTagPools(
  id: number,
  poolIds: number[]
) {
  const response = await api.put<ApiResponse<null>>(
    `/api/quota_pool/budget_tags/${id}/pools`,
    { pool_ids: poolIds }
  )
  return requireServerSuccess(response.data)
}
