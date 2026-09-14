export type RotaErrorBody = { error?: { code?: string; message?: string } }

export class RotaApiError extends Error {
  readonly status: number
  readonly code?: string
  readonly response: { status: number; data: RotaErrorBody }

  constructor(status: number, body: RotaErrorBody) {
    super(body.error?.message || `Rota request failed (${status})`)
    this.name = 'RotaApiError'
    this.status = status
    this.code = body.error?.code
    this.response = { status, data: body }
  }
}

type RequestOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  body?: unknown
  params?: Record<string, string | number | boolean | null | undefined>
  signal?: AbortSignal
}

function buildURL(path: string, params?: RequestOptions['params']) {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params ?? {})) {
    if (value !== undefined && value !== null) query.set(key, String(value))
  }
  return `/api/rota${path}${query.size ? `?${query.toString()}` : ''}`
}

async function rotaFetch(path: string, options: RequestOptions = {}) {
  const url = buildURL(path, options.params)
  const headers = new Headers({ Accept: 'application/json, application/problem+json' })
  let body: BodyInit | undefined
  if (options.body !== undefined) {
    headers.set('Content-Type', 'application/json')
    body = JSON.stringify(options.body)
  }
  const response = await fetch(url, {
    method: options.method ?? 'GET',
    credentials: 'same-origin',
    cache: 'no-store',
    headers,
    body,
    signal: options.signal,
  })
  if (!response.ok) {
    let data: RotaErrorBody = {}
    try { data = (await response.json()) as RotaErrorBody } catch { /* preserve status */ }
    throw new RotaApiError(response.status, data)
  }
  return response
}

export async function rotaRequest<T>(path: string, options: RequestOptions = {}): Promise<{ data: T }> {
  const response = await rotaFetch(path, options)
  if (response.status === 204) return { data: undefined as T }
  const data = (await response.json()) as T
  return { data }
}

export async function rotaRequestBlob<T>(path: string, options: RequestOptions = {}): Promise<{ data: T }> {
  const response = await rotaFetch(path, options)
  return { data: (await response.blob()) as T }
}
