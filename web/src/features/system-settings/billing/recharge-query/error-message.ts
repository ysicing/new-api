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
import axios from 'axios'

const ERROR_LABELS: Record<string, string> = {
  RECHARGE_IDENTIFIER_REQUIRED: 'Enter a user ID, username, or email.',
  RECHARGE_USER_NOT_FOUND: 'No matching user was found.',
  RECHARGE_USER_AMBIGUOUS: 'Multiple users matched. Use the user ID to query.',
  RECHARGE_QUERY_INTERNAL: 'We could not check recharge eligibility.',
  RECHARGE_RECORDS_QUERY_FAILED: 'We could not load recharge records.',
}

export function rechargeQueryErrorMessage(
  error: unknown,
  translate: (key: string) => string,
  fallback: string
): string {
  if (!axios.isAxiosError<{ code?: string }>(error)) {
    return fallback
  }
  const key = ERROR_LABELS[error.response?.data?.code ?? '']
  if (key) return translate(key)
  return fallback
}
