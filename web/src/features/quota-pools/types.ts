export interface QuotaPoolCapabilities {
  can_view: boolean
  can_edit: boolean
  can_edit_monthly_refill: boolean
  can_refill: boolean
  can_manage_members: boolean
  can_remove_members: boolean
  can_manage_admins: boolean
  can_delete: boolean
}

export interface QuotaPool {
  id: number
  name: string
  pool_type: 'normal' | 'default' | 'new_user'
  enabled: boolean
  is_default: boolean
  base_quota: number
  quota: number
  auto_recharge_amount: number
  weekly_limit: number
  monthly_limit: number
  monthly_refill_enabled: boolean
  monthly_refill_top_up: boolean
  monthly_refill_amount: number
  monthly_refill_day: number
  last_refill_month: number
  member_count?: number
  admin_count?: number
  system_auto_recharge?: {
    enabled: boolean
    interval: number
    threshold: number
    amount: number
    weekly_limit: number
    monthly_limit: number
  }
}

export interface QuotaPoolMember {
  id: number
  username: string
  display_name: string
  email: string
  department: string
  role: number
  status: number
  quota: number
  used_quota: number
  quota_pool_id: number
  quota_pool_admin: boolean
  reclaim_amounts?: number[]
}

export interface QuotaPoolAdminContact {
  id: number
  username: string
  display_name: string
  email: string
}

export interface QuotaPoolDirectoryItem {
  id: number
  name: string
  admin_contacts: QuotaPoolAdminContact[]
}

export interface QuotaPoolTransaction {
  id: number
  pool_id: number
  type: string
  amount: number
  quota_before: number
  quota_after: number
  user_id: number
  operator_id: number
  user_name: string
  operator_name: string
  created_at: number
}

export interface QuotaPoolOperationLog {
  id: number
  user_id: number
  username: string
  content: string
  other: string
  created_at: number
}

export interface QuotaPoolUsageStat {
  user_id: number
  username: string
  used_quota: number
  gpt_quota: number
  claude_quota: number
  deepseek_quota: number
  gemini_quota: number
  qwen_quota: number
  other_quota: number
}

export interface QuotaPoolStats {
  usage: QuotaPoolUsageStat[]
  recharge: Array<{ type: string; count: number; amount: number }>
  total_usage: number
  total_refill: number
  total_allocate: number
}

export type QuotaPoolStatsPeriod = 'week' | 'month'

export interface PageData<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data?: T
}
