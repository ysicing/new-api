import type { LogOtherData } from '../types'

export type ResponseModelObservation = NonNullable<
  LogOtherData['response_model']
>

export function isResponseModelMismatch(
  observation: ResponseModelObservation | undefined
): boolean {
  if (!observation) return false
  const returned = (observation.returned_model ?? '').toLowerCase()
  if (returned.trim() === '') return false

  for (const candidate of [
    observation.requested_model,
    observation.upstream_model,
  ]) {
    const expected = (candidate ?? '').toLowerCase()
    if (expected && (returned.startsWith(expected) || returned.endsWith(expected))) {
      return false
    }
  }

  return true
}
