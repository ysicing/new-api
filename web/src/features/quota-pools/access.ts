import type { AuthUser } from '@/stores/auth-store'

type QuotaPoolAccessUser = Pick<AuthUser, 'role'> &
  Partial<Pick<AuthUser, 'quota_pool_enabled' | 'quota_pool_admin'>>

export function canAccessQuotaPools(user?: QuotaPoolAccessUser | null) {
  if (!user?.quota_pool_enabled) return false
  return user.role === 2 || user.role >= 10 || Boolean(user.quota_pool_admin)
}
