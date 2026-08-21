import { createFileRoute, redirect } from '@tanstack/react-router'

import { QuotaPools } from '@/features/quota-pools'
import { canAccessQuotaPools } from '@/features/quota-pools/access'
import { useAuthStore } from '@/stores/auth-store'

export const Route = createFileRoute('/_authenticated/quota-pools/')({
  beforeLoad: () => {
    if (!canAccessQuotaPools(useAuthStore.getState().auth.user)) {
      throw redirect({ to: '/403' })
    }
  },
  component: QuotaPools,
})
