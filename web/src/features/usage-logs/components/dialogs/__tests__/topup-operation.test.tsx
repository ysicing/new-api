/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'

import type { UsageLog } from '../../../data/schema'
import { DetailsDialog } from '../details-dialog'

const topupOperationLog: UsageLog = {
  id: 1,
  user_id: 7,
  created_at: 1_700_000_000,
  type: 1,
  content: 'user.quota_pool_recharge',
  username: 'root-operator',
  token_name: '',
  model_name: '',
  quota: 0,
  prompt_tokens: 0,
  completion_tokens: 0,
  use_time: 0,
  is_stream: false,
  channel: 0,
  channel_name: '',
  token_id: 0,
  group: '',
  ip: '203.0.113.9',
  other: JSON.stringify({
    op: {
      action: 'user.quota_pool_recharge',
      params: { target_user_id: 25, quota: '¥10.000000 额度' },
    },
    admin_info: { admin_id: 7, admin_username: 'root-operator' },
  }),
  request_id: 'req-topup-operation',
  upstream_request_id: '',
}

test('shows structured top-up operation with its operator', () => {
  render(
    <DetailsDialog
      log={topupOperationLog}
      isAdmin
      open
      onOpenChange={() => undefined}
    />
  )

  expect(
    screen.getByText('Replenished quota for user #25: ¥10.000000 额度')
  ).toBeInTheDocument()
  expect(screen.getByText('root-operator (ID: 7)')).toBeInTheDocument()
})
