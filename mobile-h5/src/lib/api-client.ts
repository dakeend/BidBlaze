import axios from 'axios'
import { getAuthToken, getCurrentUser } from './auth'
import type {
  APIResponse,
  AuctionSnapshot,
  BidFailureData,
  BidSuccessData,
  EventListData,
} from './types'
import { createMockBidResult, createMockSnapshot } from '../mocks/auction-fixture'

const apiBase = import.meta.env.VITE_API_BASE || 'http://localhost:8080'

export const apiClient = axios.create({
  baseURL: apiBase,
  timeout: 8000,
})

apiClient.interceptors.request.use((config) => {
  config.headers.Authorization = `Bearer ${getAuthToken()}`
  config.headers['X-Client-Type'] = 'mobile_h5'
  config.headers['X-Request-Id'] = `req-${Date.now()}-${Math.random().toString(16).slice(2)}`
  return config
})

function idempotencyKey(scope: string, auctionId: number): string {
  return `${scope}-${auctionId}-${Date.now()}-${Math.random().toString(16).slice(2)}`
}

export async function getAuctionStatus(auctionId: number): Promise<AuctionSnapshot> {
  try {
    const response = await apiClient.get<APIResponse<AuctionSnapshot>>(
      `/api/auctions/${auctionId}/status`,
    )
    return response.data.data
  } catch {
    return createMockSnapshot(auctionId)
  }
}

export async function getEventsAfter(
  auctionId: number,
  afterSeq: number,
  limit = 100,
): Promise<EventListData> {
  try {
    const response = await apiClient.get<APIResponse<EventListData>>(
      `/api/auctions/${auctionId}/events`,
      {
        params: { after_seq: afterSeq, limit },
      },
    )
    return response.data.data
  } catch {
    return {
      events: [],
      has_more: false,
      snapshot_required: true,
      server_time: new Date().toISOString(),
    }
  }
}

export async function placeBid(
  auctionId: number,
  amount: number,
): Promise<APIResponse<BidSuccessData | BidFailureData | null>> {
  try {
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
  } catch {
    const data = createMockBidResult(auctionId, amount, getCurrentUser())
    return {
      code: 0,
      msg: 'ok',
      data,
    }
  }
}

export function wsBaseUrl(): string {
  return import.meta.env.VITE_WS_BASE || apiBase.replace(/^http/, 'ws')
}
