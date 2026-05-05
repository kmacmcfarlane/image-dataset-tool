/** Stable UI error shape for normalized API errors */
export interface ApiError {
  status: number
  code: string
  message: string
}

/** Options for the typed fetch wrapper */
export interface FetchOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  body?: unknown
  headers?: Record<string, string>
  signal?: AbortSignal
}
