/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { WalletAdd01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { TitledCard } from '@/components/ui/titled-card'

import { AutoRechargeEligibilityDialog } from './components/auto-recharge-eligibility-dialog'

export function Tools() {
  const { t } = useTranslation()
  const [eligibilityOpen, setEligibilityOpen] = useState(false)

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Tools')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-3xl flex-col gap-4'>
          <p className='text-muted-foreground text-sm'>
            {t('Account self-service queries and diagnostics.')}
          </p>
          <TitledCard
            title={t('Automatic recharge eligibility')}
            description={t(
              'Check whether your quota is currently eligible for automatic recharge.'
            )}
            icon={<HugeiconsIcon icon={WalletAdd01Icon} strokeWidth={2} />}
            action={
              <Button type='button' onClick={() => setEligibilityOpen(true)}>
                <HugeiconsIcon
                  icon={WalletAdd01Icon}
                  strokeWidth={2}
                  data-icon='inline-start'
                />
                {t('Check automatic recharge eligibility')}
              </Button>
            }
          >
            <p className='text-muted-foreground text-sm'>
              {t(
                'This tool only reviews your eligibility. It does not change your balance or recharge settings.'
              )}
            </p>
          </TitledCard>
        </div>
        <AutoRechargeEligibilityDialog
          open={eligibilityOpen}
          onOpenChange={setEligibilityOpen}
        />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
