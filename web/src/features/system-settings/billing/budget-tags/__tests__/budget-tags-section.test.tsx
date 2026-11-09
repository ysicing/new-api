import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { formatQuota } from '@/lib/format'

import { BudgetTagsSection } from '../budget-tags-section'

const apiMocks = vi.hoisted(() => ({
  createQuotaPoolBudgetTag: vi.fn(),
  deleteQuotaPoolBudgetTag: vi.fn(),
  getQuotaPoolBudgetStats: vi.fn(),
  listAssignableQuotaPools: vi.fn(),
  listQuotaPoolBudgetTags: vi.fn(),
  replaceQuotaPoolBudgetTagPools: vi.fn(),
  updateQuotaPoolBudgetTag: vi.fn(),
}))

vi.mock('../api', () => apiMocks)

function renderSection() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <BudgetTagsSection />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
  vi.setSystemTime(new Date('2026-09-22T08:00:00+08:00'))
  apiMocks.listQuotaPoolBudgetTags.mockResolvedValue({
    success: true,
    data: [
      { id: 1, name: '产研中心', pool_count: 1, created_at: 1, updated_at: 1 },
    ],
  })
  apiMocks.getQuotaPoolBudgetStats.mockResolvedValue({
    success: true,
    data: {
      start_month: '2026-09',
      end_month: '2026-09',
      time_zone: 'Asia/Shanghai',
      summary: { net_recharge: 1_500_000, net_consumption: 600_000 },
      months: [
        { month: '2026-09', net_recharge: 1_500_000, net_consumption: 600_000 },
      ],
      tags: [
        {
          tag_id: 1,
          name: '产研中心',
          pool_count: 1,
          net_recharge: 1_500_000,
          net_consumption: 600_000,
          months: [
            {
              month: '2026-09',
              net_recharge: 1_500_000,
              net_consumption: 600_000,
            },
          ],
          pools: [
            {
              pool_id: 7,
              pool_name: '研发额度池',
              net_recharge: 1_500_000,
              net_consumption: 600_000,
              months: [
                {
                  month: '2026-09',
                  net_recharge: 1_500_000,
                  net_consumption: 600_000,
                },
              ],
            },
          ],
        },
      ],
    },
  })
  apiMocks.listAssignableQuotaPools.mockResolvedValue({
    success: true,
    data: {
      items: [
        { id: 7, name: '研发额度池', budget_tag_id: 1 },
        { id: 8, name: '共享额度池', budget_tag_id: 0 },
      ],
    },
  })
  apiMocks.createQuotaPoolBudgetTag.mockResolvedValue({
    success: true,
    data: { id: 2, name: '市场中心' },
  })
  apiMocks.replaceQuotaPoolBudgetTagPools.mockResolvedValue({ success: true })
  apiMocks.updateQuotaPoolBudgetTag.mockResolvedValue({ success: true })
  apiMocks.deleteQuotaPoolBudgetTag.mockResolvedValue({ success: true })
})

afterEach(() => {
  vi.useRealTimers()
})

test('loads the current natural month and shows tag and pool totals', async () => {
  renderSection()

  expect(await screen.findByText('产研中心')).toBeVisible()
  expect(apiMocks.getQuotaPoolBudgetStats).toHaveBeenCalledWith({
    startMonth: '2026-09',
    endMonth: '2026-09',
  })
  expect(screen.getAllByText(formatQuota(1_500_000)).length).toBeGreaterThan(0)
  expect(screen.getAllByText(formatQuota(600_000)).length).toBeGreaterThan(0)
  expect(screen.getByText('研发额度池')).toBeVisible()
})

test('applies an inclusive multi-month range', async () => {
  const user = userEvent.setup()
  renderSection()
  await screen.findByText('产研中心')

  const start = screen.getByLabelText('Start month')
  const end = screen.getByLabelText('End month')
  await user.clear(start)
  await user.type(start, '2026-01')
  await user.clear(end)
  await user.type(end, '2026-06')
  await user.click(screen.getByRole('button', { name: 'Apply' }))

  await waitFor(() => {
    expect(apiMocks.getQuotaPoolBudgetStats).toHaveBeenLastCalledWith({
      startMonth: '2026-01',
      endMonth: '2026-06',
    })
  })
})

test('creates a budget tag', async () => {
  const user = userEvent.setup()
  renderSection()
  await screen.findByText('产研中心')

  await user.type(screen.getByLabelText('Budget tag name'), '市场中心')
  await user.click(screen.getByRole('button', { name: 'Add tag' }))
  await waitFor(() => {
    expect(apiMocks.createQuotaPoolBudgetTag).toHaveBeenCalledWith('市场中心')
  })
})

test('assigns pools from the tag card', async () => {
  const user = userEvent.setup()
  renderSection()
  await screen.findByText('产研中心')

  await user.click(screen.getByRole('button', { name: 'Manage pools' }))
  expect(await screen.findByText('Assign quota pools')).toBeVisible()
  expect(await screen.findByText('共享额度池')).toBeVisible()
  const [sharedPool] = await screen.findAllByLabelText('共享额度池')
  await user.click(sharedPool)
  await user.click(screen.getByRole('button', { name: 'Save assignments' }))

  await waitFor(() => {
    expect(apiMocks.replaceQuotaPoolBudgetTagPools).toHaveBeenCalledWith(
      1,
      [7, 8]
    )
  })
})

test('shows an explicit error instead of zero totals when statistics fail', async () => {
  apiMocks.getQuotaPoolBudgetStats.mockRejectedValue(
    new Error('database unavailable')
  )

  renderSection()

  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Unable to load budget statistics'
  )
  expect(screen.getByRole('button', { name: 'Retry' })).toBeVisible()
  expect(screen.queryByText(formatQuota(0))).not.toBeInTheDocument()
})

test('renders an unassigned tag when legacy statistics contain null arrays', async () => {
  apiMocks.listQuotaPoolBudgetTags.mockResolvedValue({
    success: true,
    data: [
      { id: 1, name: '产研中心', pool_count: 0, created_at: 1, updated_at: 1 },
    ],
  })
  apiMocks.getQuotaPoolBudgetStats.mockResolvedValue({
    success: true,
    data: {
      start_month: '2026-09',
      end_month: '2026-09',
      time_zone: 'Asia/Shanghai',
      summary: { net_recharge: 0, net_consumption: 0 },
      months: null,
      tags: [
        {
          tag_id: 1,
          name: '产研中心',
          pool_count: 0,
          net_recharge: 0,
          net_consumption: 0,
          months: null,
          pools: null,
        },
      ],
    },
  })

  renderSection()

  expect(await screen.findByText('产研中心')).toBeVisible()
  expect(screen.getByText('0 quota pools')).toBeVisible()
})
