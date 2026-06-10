import axios, { type AxiosProgressEvent } from 'axios'
import { getAuthToken } from './auth'
import type {
  APIResponse,
  AuctionSnapshot,
  BidFailureData,
  BidSuccessData,
  EventListData,
  UploadResult,
} from './types'

export class ApiCallError extends Error {
  code: number
  constructor(code: number, message: string) {
    super(message)
    this.name = 'ApiCallError'
    this.code = code
  }
}

const apiBase = resolveApiBase()

export const apiClient = axios.create({
  baseURL: apiBase,
  timeout: 8000,
})

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

apiClient.interceptors.request.use((config) => {
  config.headers.Authorization = `Bearer ${getAuthToken()}`
  config.headers['X-Client-Type'] = 'mobile_h5'
  config.headers['X-Request-Id'] = `req-${Date.now()}-${Math.random().toString(16).slice(2)}`
  return config
})

apiClient.interceptors.response.use(
  (resp) => {
    const body = resp.data as Partial<APIResponse<unknown>> | undefined
    if (body && typeof body.code === 'number' && body.code !== 0) {
      throw new ApiCallError(body.code, body.msg || 'api error')
    }
    return resp
  },
  (error) => {
    const body = error?.response?.data as Partial<APIResponse<unknown>> | undefined
    const msg = body?.msg || error?.message || 'network error'
    throw new ApiCallError(body?.code ?? -1, msg)
  },
)

function idempotencyKey(scope: string, auctionId: number): string {
  return `${scope}-${auctionId}-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

export async function getAuctionStatus(auctionId: number): Promise<AuctionSnapshot> {
  const response = await apiClient.get<APIResponse<AuctionSnapshot>>(
    `/api/auctions/${auctionId}/status`,
  )
  return response.data.data
}

export async function getEventsAfter(
  auctionId: number,
  afterSeq: number,
  limit = 100,
): Promise<EventListData> {
  const response = await apiClient.get<APIResponse<EventListData>>(
    `/api/auctions/${auctionId}/events`,
    {
      params: { after_seq: afterSeq, limit },
    },
  )
  return response.data.data
}

export async function placeBid(
  auctionId: number,
  amount: number,
): Promise<APIResponse<BidSuccessData | BidFailureData | null>> {
  const response = await apiClient.post<APIResponse<BidSuccessData | BidFailureData | null>>(
    `/api/auctions/${auctionId}/bid`,
    { amount },
    {
      headers: {
        'Idempotency-Key': idempotencyKey('bid', auctionId),
      },
    },
  )
  return response.data
}

export async function payOrder(orderId: number): Promise<{ status: string; paid_at: string | null }> {
  const response = await apiClient.post<APIResponse<{ order: { status: string; paid_at: string | null } }>>(
    `/api/orders/${orderId}/pay`,
  )
  return response.data.data.order
}

export async function uploadImage(
  file: File,
  onProgress?: (progress: number) => void,
): Promise<UploadResult> {
  const formData = new FormData()
  formData.append('file', file)

  const response = await apiClient.post<APIResponse<UploadResult>>('/api/uploads', formData, {
    onUploadProgress: (event: AxiosProgressEvent) => {
      if (!event.total) {
        return
      }
      onProgress?.(Math.round((event.loaded / event.total) * 100))
    },
  })

  if (response.data.code !== 0) {
    throw new Error(response.data.msg || 'upload failed')
  }

  onProgress?.(100)
  return response.data.data
}

export function wsBaseUrl(): string {
  const explicitBase = import.meta.env.VITE_WS_BASE?.trim()
  if (explicitBase) {
    return explicitBase.replace(/\/$/, '')
  }
  if (import.meta.env.DEV && typeof window !== 'undefined') {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${protocol}//${window.location.host}`
  }
  return apiBase.replace(/^http/, 'ws')
}
