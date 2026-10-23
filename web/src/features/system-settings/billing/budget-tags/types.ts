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
export interface BudgetAmounts {
  net_recharge: number
  net_consumption: number
}

export interface BudgetMonthStat extends BudgetAmounts {
  month: string
}

export interface BudgetPoolStat extends BudgetAmounts {
  pool_id: number
  pool_name: string
  months: BudgetMonthStat[]
}

export interface BudgetTagStat extends BudgetAmounts {
  tag_id: number
  name: string
  pool_count: number
  months: BudgetMonthStat[]
  pools: BudgetPoolStat[]
}

export interface BudgetStats {
  start_month: string
  end_month: string
  time_zone: string
  summary: BudgetAmounts
  months: BudgetMonthStat[]
  tags: BudgetTagStat[]
}

export interface BudgetTag {
  id: number
  name: string
  pool_count: number
  created_at: number
  updated_at: number
}

export interface AssignableQuotaPool {
  id: number
  name: string
  budget_tag_id: number
}

export interface ApiResponse<T> {
  success: boolean
  data: T
  message?: string
}
