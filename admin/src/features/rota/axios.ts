import { rotaRequest, rotaRequestBlob, RotaApiError } from './api'

type QueryOptions = {
  params?: Record<string, string | number | undefined>
  signal?: AbortSignal
  responseType?: 'json' | 'blob'
}

type BodyOptions = { signal?: AbortSignal }

const api = {
  get: <T>(path: string, options?: QueryOptions) =>
    options?.responseType === 'blob'
      ? rotaRequestBlob<T>(path, { params: options.params, signal: options.signal })
      : rotaRequest<T>(path, { method: 'GET', params: options?.params, signal: options?.signal }),
  post: <T = unknown, _D = unknown, _C = unknown>(path: string, body?: unknown, options?: BodyOptions) =>
    rotaRequest<T>(path, { method: 'POST', body, signal: options?.signal }),
  put: <T = unknown, _D = unknown, _C = unknown>(path: string, body?: unknown, options?: BodyOptions) =>
    rotaRequest<T>(path, { method: 'PUT', body, signal: options?.signal }),
  patch: <T = unknown, _D = unknown, _C = unknown>(path: string, body?: unknown, options?: BodyOptions) =>
    rotaRequest<T>(path, { method: 'PATCH', body, signal: options?.signal }),
  delete: <T = unknown>(path: string, options?: BodyOptions) =>
    rotaRequest<T>(path, { method: 'DELETE', signal: options?.signal }),
}

export { RotaApiError }
export default api
