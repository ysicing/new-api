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
