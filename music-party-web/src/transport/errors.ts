import type { APIError as APIErrorPayload } from '../contracts/generated/models'

export class APIError extends Error implements APIErrorPayload {
  readonly status: number
  readonly code: string
  readonly requestId?: string
  readonly fieldErrors?: Record<string, string>

  constructor(payload: APIErrorPayload) {
    super(payload.message)
    this.name = 'APIError'
    this.status = payload.status
    this.code = payload.code
    if (payload.requestId !== undefined) this.requestId = payload.requestId
    if (payload.fieldErrors !== undefined) this.fieldErrors = payload.fieldErrors
  }
}

export function isAPIError(error: unknown): error is APIError {
  return error instanceof APIError
}
