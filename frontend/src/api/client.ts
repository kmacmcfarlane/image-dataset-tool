import type { ApiError, FetchOptions } from './types'

/**
 * Typed fetch wrapper for backend API calls.
 * All backend calls should go through this module per DEVELOPMENT_PRACTICES 4.2.
 *
 * Uses Vite proxy (/api/* and /v1/*) in dev mode.
 */

/** Event bus for API errors — components can subscribe to display banners/toasts */
type ErrorHandler = (error: ApiError) => void
const errorHandlers: ErrorHandler[] = []

export function onApiError(handler: ErrorHandler): () => void {
  errorHandlers.push(handler)
  return () => {
    const idx = errorHandlers.indexOf(handler)
    if (idx >= 0) errorHandlers.splice(idx, 1)
  }
}

function notifyError(error: ApiError): void {
  for (const handler of errorHandlers) {
    handler(error)
  }
}

/**
 * Normalize fetch errors into a stable ApiError shape.
 */
async function normalizeError(response: Response): Promise<ApiError> {
  let code = 'unknown_error'
  let message = `HTTP ${response.status}`

  try {
    const body = await response.json() as Record<string, unknown>
    if (typeof body.code === 'string') code = body.code
    if (typeof body.message === 'string') message = body.message
  } catch {
    // Response body was not JSON; use defaults
  }

  return { status: response.status, code, message }
}

/**
 * Typed fetch wrapper. Returns parsed JSON on success,
 * throws ApiError on failure and notifies error handlers.
 */
export async function apiFetch<T>(path: string, options: FetchOptions = {}): Promise<T> {
  const { method = 'GET', body, headers = {}, signal } = options

  const requestHeaders: Record<string, string> = {
    ...headers,
  }

  if (body !== undefined) {
    requestHeaders['Content-Type'] = 'application/json'
  }

  const response = await fetch(path, {
    method,
    headers: requestHeaders,
    body: body !== undefined ? JSON.stringify(body) : undefined,
    signal,
  })

  if (!response.ok) {
    const error = await normalizeError(response)
    notifyError(error)
    throw error
  }

  // 204 No Content
  if (response.status === 204) {
    return undefined as T
  }

  return response.json() as Promise<T>
}
