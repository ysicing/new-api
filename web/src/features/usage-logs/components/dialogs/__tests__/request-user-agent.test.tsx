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

const requestLog: UsageLog = {
  id: 1,
  user_id: 42,
  created_at: 1_700_000_000,
  type: 2,
  content: '',
  username: 'alice',
  token_name: 'default-token',
  model_name: 'gpt-5',
  quota: 100,
  prompt_tokens: 10,
  completion_tokens: 5,
  use_time: 2,
  is_stream: false,
  channel: 7,
  channel_name: 'openai',
  token_id: 3,
  group: 'default',
  ip: '203.0.113.9',
  other: JSON.stringify({ user_agent: 'codex-cli/1.2' }),
  request_id: 'req-1',
  upstream_request_id: '',
}

test('shows model request User-Agent only to administrators', () => {
  const adminView = render(
    <DetailsDialog
      log={requestLog}
      isAdmin
      open
      onOpenChange={() => undefined}
    />
  )

  expect(screen.getByText('User Agent')).toBeInTheDocument()
  expect(screen.getByText('codex-cli/1.2')).toBeInTheDocument()

  adminView.unmount()
  render(
    <DetailsDialog
      log={requestLog}
      isAdmin={false}
      open
      onOpenChange={() => undefined}
    />
  )

  expect(screen.queryByText('User Agent')).not.toBeInTheDocument()
  expect(screen.queryByText('codex-cli/1.2')).not.toBeInTheDocument()
})
