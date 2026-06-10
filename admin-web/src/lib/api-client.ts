// Axios 实例 + 拦截器。统一注入鉴权头、X-Request-Id、X-Client-Type、
// 写接口的 Idempotency-Key；统一按合同 §1.2 解析错误码；401 全局跳登录。
import axios from 'axios'
import type { AxiosInstance, AxiosRequestConfig, InternalAxiosRequestConfig } from 'axios'
import { messageForCode, isAuthError } from './error-codes'
import { syncServerTime } from './time'
import type { ApiEnvelope } from './types'

const TOKEN_KEY = 'auction_token'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}
export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}
export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

// --- ID 生成 ---
function rid(): string {
  // crypto.randomUUID 在所有现代浏览器可用；兜底用时间戳。
  return typeof crypto !== 'undefined' && crypto.randomUUID
    ? crypto.randomUUID()
    : `r-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

/** 生成业务幂等 key，写接口（出价/支付/创建）必带。 */
export function genIdempotencyKey(prefix = 'op'): string {
  return `${prefix}-${rid()}`
}

// --- 统一错误 ---
export class ApiError extends Error {
  code: number
  httpStatus: number | null
  data: unknown
  constructor(code: number, message: string, httpStatus: number | null, data: unknown) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.httpStatus = httpStatus
    this.data = data
  }
}

// 401 → 由 auth 层注入登出回调，避免 lib 依赖 React 路由。
let onUnauthorized: (() => void) | null = null
export function setUnauthorizedHandler(fn: () => void): void {
  onUnauthorized = fn
}

const CLIENT_TYPE = import.meta.env.VITE_CLIENT_TYPE || 'admin'
const apiBase = resolveApiBase()

function resolveApiBase(): string {
  const explicitBase = import.meta.env.VITE_API_BASE?.trim()
  if (explicitBase) {
    return explicitBase.replace(/\/$/, '')
  }
  if (import.meta.env.DEV) {
    return ''
  }
  return 'http://localhost:8080'
}

export const http: AxiosInstance = axios.create({
  baseURL: apiBase,
  timeout: 15000,
})

http.interceptors.request.use((config: InternalAxiosRequestConfig) => {
  const token = getToken()
  if (token) config.headers.set('Authorization', `Bearer ${token}`)
  config.headers.set('X-Request-Id', rid())
  config.headers.set('X-Client-Type', CLIENT_TYPE)
  return config
})

http.interceptors.response.use(
  (resp) => {
    const body = resp.data as Partial<ApiEnvelope<unknown>> | undefined
    // 任意返回 server_time 的接口都用于校准时钟。
    const st = (body?.data as { server_time?: string } | undefined)?.server_time
    if (st) syncServerTime(st)
    // 业务错误码：HTTP 200 但 code!=0（如 2002/2101/2103）。
    if (body && typeof body.code === 'number' && body.code !== 0) {
      const err = new ApiError(body.code, messageForCode(body.code, body.msg), resp.status, body.data)
      if (isAuthError(body.code)) onUnauthorized?.()
      return Promise.reject(err)
    }
    return resp
  },
  (error) => {
    // 传输层错误（4xx/5xx）。优先取响应体里的 code。
    const status = error?.response?.status ?? null
    const body = error?.response?.data as Partial<ApiEnvelope<unknown>> | undefined
    const code = typeof body?.code === 'number' ? body.code : status === 401 ? 1002 : 9999
    const apiErr = new ApiError(code, messageForCode(code, body?.msg), status, body?.data)
    if (status === 401 || isAuthError(code)) onUnauthorized?.()
    return Promise.reject(apiErr)
  },
)

// --- 便捷方法：成功时直接解出 envelope.data ---
async function unwrap<T>(p: Promise<{ data: ApiEnvelope<T> }>): Promise<T> {
  const resp = await p
  return resp.data.data
}

export const api = {
  get: <T>(url: string, config?: AxiosRequestConfig) => unwrap<T>(http.get(url, config)),
  post: <T>(url: string, data?: unknown, config?: AxiosRequestConfig) =>
    unwrap<T>(http.post(url, data, config)),
  put: <T>(url: string, data?: unknown, config?: AxiosRequestConfig) =>
    unwrap<T>(http.put(url, data, config)),
}
