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

export function shouldShowQuotaPoolList(user?: QuotaPoolAccessUser | null) {
  return canListAllQuotaPools(user) || Boolean(user?.quota_pool_admin)
}
