import i18next from 'i18next'
import { expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { updateSystemOption } from '../api'

test('saving announcements sends the current interface language', async () => {
  const previousLanguage = i18next.language
  await i18next.changeLanguage('zhTW')
  const put = vi.spyOn(api, 'put').mockResolvedValue({ data: { success: true } })
  const request = { key: 'console_setting.announcements', value: '[]' }

  try {
    await updateSystemOption(request)

    expect(put).toHaveBeenCalledWith('/api/option/', request, {
      headers: { 'Accept-Language': 'zh-TW' },
    })
  } finally {
    put.mockRestore()
    await i18next.changeLanguage(previousLanguage)
  }
})
