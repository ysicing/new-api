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
export type RechargeQueryPeriod = 'week' | 'month'

export type QuotaPoolRechargeRecord = {
  id: number
  pool_id: number
  pool_name: string
  user_id: number
  user_name: string
  user_email: string
  operator_id: number
  operator_name: string
  type: 'allocate_auto' | 'allocate_manual'
  amount: number
  created_at: number
}

export type AutoRechargeLimitUsage = {
  used: number
  limit: number
}

export type AutoRechargeEligibility = {
  eligible: boolean
  reason?: string
  user_id: number
  username: string
  email: string
  user_quota: number
  threshold: number
  pool_id: number
  pool_name: string
  pool_quota: number | null
  amount: number
  weekly: AutoRechargeLimitUsage
  monthly: AutoRechargeLimitUsage
}

export type RechargeRecordsResponse = {
  success: boolean
  message: string
  data?: {
    page: number
    page_size: number
    total: number
    items: QuotaPoolRechargeRecord[]
  }
}

export type RechargeEligibilityResponse = {
  success: boolean
  message: string
  data?: AutoRechargeEligibility
}
