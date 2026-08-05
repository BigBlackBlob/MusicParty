import axios, { AxiosError, AxiosHeaders, type AxiosInstance, type AxiosRequestConfig } from 'axios'
import type { APIError as APIErrorPayload } from '../contracts/generated/models'
import { APIError } from './errors'

const SAFE_METHODS = new Set(['get', 'head', 'options'])

function cookie(name: string): string | undefined {
  const prefix = `${name}=`
  const value = document.cookie.split('; ').find((item) => item.startsWith(prefix))
  return value ? decodeURIComponent(value.slice(prefix.length)) : undefined
}

function errorPayload(error: AxiosError<Partial<APIErrorPayload>>): APIErrorPayload {
  const body = error.response?.data
  return {
    status: error.response?.status ?? 0,
    code: body?.code || (error.code === AxiosError.ERR_CANCELED ? 'REQUEST_CANCELLED' : 'NETWORK_ERROR'),
    message: body?.message || error.message || 'Request failed',
    ...(body?.requestId ? { requestId: body.requestId } : {}),
    ...(body?.fieldErrors ? { fieldErrors: body.fieldErrors } : {})
  }
}

const axiosInstance = axios.create({
  timeout: 10_000,
  withCredentials: true
})

axiosInstance.interceptors.request?.use((config) => {
  if (!SAFE_METHODS.has((config.method || 'get').toLowerCase())) {
    const csrfToken = cookie('MP_CSRF')
    if (csrfToken) {
      const headers = AxiosHeaders.from(config.headers)
      headers.set('X-CSRF-Token', csrfToken)
      config.headers = headers
    }
  }
  return config
})

axiosInstance.interceptors.response?.use(
  (response) => response.data,
  (error: unknown) => {
    if (axios.isAxiosError<Partial<APIErrorPayload>>(error)) {
      return Promise.reject(new APIError(errorPayload(error)))
    }
    return Promise.reject(error)
  }
)

export interface RequestOptions extends AxiosRequestConfig {
  signal?: AbortSignal
}

export interface DataClient extends Omit<AxiosInstance, 'get' | 'delete' | 'post' | 'put' | 'patch'> {
  get<T>(url: string, config?: RequestOptions): Promise<T>
  delete<T = void>(url: string, config?: RequestOptions): Promise<T>
  post<T, B = unknown>(url: string, body?: B, config?: RequestOptions): Promise<T>
  put<T, B = unknown>(url: string, body?: B, config?: RequestOptions): Promise<T>
  patch<T, B = unknown>(url: string, body?: B, config?: RequestOptions): Promise<T>
}

// The response interceptor unwraps AxiosResponse.data at runtime.
export const httpClient = axiosInstance as DataClient
