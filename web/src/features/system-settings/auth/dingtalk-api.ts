/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { api } from '@/lib/api'

export type DingTalkTestUser = {
  id: number
  username: string
  display_name: string
  email: string
  department: string
  dingtalk_bound: boolean
}

type ApiResponse<T> = {
  success: boolean
  message?: string
  data?: T
}

export type DingTalkTestUserPage = {
  items: DingTalkTestUser[]
  total: number
  page: number
  page_size: number
}

export async function searchDingTalkTestUsers(keyword: string) {
  const response = await api.get<ApiResponse<DingTalkTestUserPage>>(
    '/api/dingtalk/test-users',
    {
      params: { keyword, p: 1, page_size: 20 },
    }
  )
  return response.data
}

export async function sendDingTalkTestMessage(userId: number) {
  const response = await api.post<ApiResponse<{ bound_now: boolean }>>(
    '/api/dingtalk/test-message',
    { user_id: userId }
  )
  return response.data
}

export async function sendDingTalkAnnouncementGroupTestMessage() {
  const response = await api.post<ApiResponse<null>>(
    '/api/dingtalk/test-group-message'
  )
  return response.data
}
