// Curated public type surface for admin-web.
// Raw types are generated from docs/api/openapi.yaml via `npm run gen:types`
// (openapi.d.ts). Do NOT hand-edit openapi.d.ts; edit the contract + regenerate.
import type { components } from './openapi'

type S = components['schemas']

// --- Entities ---
export type UserBrief = S['UserBrief']
export type Auction = S['Auction']
export type Bid = S['Bid']
export type Order = S['Order']
export type UploadData = S['UploadData']

// --- Requests ---
export type LoginRequest = S['LoginRequest']
export type CreateAuctionRequest = S['CreateAuctionRequest']
export type UpdateAuctionRequest = S['UpdateAuctionRequest']

// --- Response data payloads ---
export type LoginData = S['LoginData']
export type AuctionListData = S['AuctionListData']
export type AuctionSnapshot = S['AuctionSnapshot']
export type EventEnvelope = S['EventEnvelope']
export type EventListData = S['EventListData']

// --- Status unions (string literals; erasableSyntaxOnly forbids enums) ---
export type AuctionStatus = S['AuctionStatus']
export type OrderStatus = S['OrderStatus']

export const AUCTION_STATUSES: AuctionStatus[] = [
  'pending',
  'active',
  'ended',
  'cancelled',
]
export const ORDER_STATUSES: OrderStatus[] = ['pending_pay', 'paid', 'closed']

// Generic envelope: every endpoint wraps payload in { code, msg, data }.
export interface ApiEnvelope<T> {
  code: number
  msg: string
  data: T
}
