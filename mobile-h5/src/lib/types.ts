export type AuctionStatus = 'pending' | 'active' | 'ended' | 'cancelled'

export type UserBrief = {
  id: number
  nickname: string
  avatar: string | null
}

export type Auction = {
  id: number
  title: string
  description?: string
  cover_url?: string | null
  images?: string[]
  stream_url?: string | null
  start_price: number
  price_step: number
  ceiling_price?: number | null
  current_price: number
  current_leader: UserBrief | null
  start_time: string
  end_time: string
  original_end_time?: string
  extend_seconds?: number
  extend_threshold?: number
  status: AuctionStatus
  viewer_count: number
  bid_count?: number
  seller?: UserBrief
  created_at?: string
  updated_at?: string
}

export type Bid = {
  id: number
  auction_id: number
  user: UserBrief
  amount: number
  status: 'accepted' | 'rejected'
  reject_reason?: string | null
  idempotency_key?: string
  created_at: string
}

export type EventType =
  | 'snapshot'
  | 'bid_update'
  | 'auction_extended'
  | 'auction_started'
  | 'auction_ended'
  | 'auction_cancelled'
  | 'viewer_count'

export type EventEnvelope<TData = Record<string, unknown>> = {
  type: EventType
  event_id: string
  auction_id: number
  seq: number
  server_time: string
  data: TData
}

export type AuctionSnapshot = {
  auction: Auction
  top_bids: Bid[]
  last_event_seq: number
  server_time: string
}

export type EventListData = {
  events: EventEnvelope[]
  has_more: boolean
  snapshot_required: boolean
  server_time: string
}

export type APIResponse<T> = {
  code: number
  msg: string
  data: T
}

export type BidSuccessData = {
  bid: Bid
  auction_version: number
  current_price: number
  current_leader: UserBrief
  extended: boolean
  new_end_time: string
  server_time: string
  ceiling_hit?: boolean
  order_id?: number
}

export type BidFailureData = {
  min_acceptable_amount?: number
  current_price?: number
  price_step?: number
  ceiling_price?: number
  server_time?: string
}

export type ConnectionState = 'connected' | 'reconnecting' | 'polling' | 'disconnected'

export type BidButtonState = 'idle' | 'pending' | 'cooldown' | 'disabled'
