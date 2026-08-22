import { ROLE } from '@/lib/roles'
import type { AuthUser } from '@/stores/auth-store'

type QuotaPoolAccessUser = Pick<AuthUser, 'role'> &
  Partial<
    Pick<AuthUser, 'quota_pool_enabled' | 'quota_pool_admin' | 'quota_pool_id'>
  >

export function canAccessQuotaPools(user?: QuotaPoolAccessUser | null) {
  return Boolean(user && user.quota_pool_enabled)
}

export function canListAllQuotaPools(user?: QuotaPoolAccessUser | null) {
  return Boolean(
    user &&
    (user.role === ROLE.QUOTA_POOL_SUPER_ADMIN || user.role >= ROLE.ADMIN)
  )
}
