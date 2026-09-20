import { render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, describe, expect, test, vi } from 'vitest'

import { ModelBadge } from '../model-badge'

vi.mock('@lobehub/icons', () => ({
  OpenAI: { Color: () => null },
}))

describe('model badge response model diagnostics', () => {
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      'Response model mismatch': 'Response model mismatch',
      'Response model: {{model}}': 'Response model: {{model}}',
      'Request Model:': 'Request Model:',
      'Upstream Model:': 'Upstream Model:',
      'Response Model:': 'Response Model:',
    })
  })

  test('shows a warning when the upstream response model differs', () => {
    render(
      <ModelBadge
        modelName='gpt-6-astra'
        responseModel={{
          requested_model: 'gpt-6-astra',
          upstream_model: 'mapped-gpt-6-astra',
          returned_model: 'gpt-5.6-luna',
        }}
      />
    )

    expect(screen.getByText('Response model mismatch')).toBeVisible()
  })
})
