import type { AuctionSnapshot, Bid, EventEnvelope, UserBrief } from '../lib/types'
import { nowIso } from '../lib/time'

const seller: UserBrief = {
  id: 1,
  nickname: '主播阿明',
  avatar: null,
}

const userOne: UserBrief = {
  id: 2,
  nickname: '买家张三',
  avatar: null,
}

const userTwo: UserBrief = {
  id: 3,
  nickname: '买家李四',
  avatar: null,
}

let mockSeq = 18
let mockBidId = 124
let mockPrice = 95000
let mockLeader: UserBrief | null = userTwo

function futureIso(offsetMs: number): string {
  return new Date(Date.now() + offsetMs).toISOString()
}

export function createMockSnapshot(auctionId: number): AuctionSnapshot {
  const bids: Bid[] = [
    {
      id: 123,
      auction_id: auctionId,
      user: userTwo,
      amount: mockPrice,
      status: 'accepted',
      reject_reason: null,
      idempotency_key: 'mock-bid-123',
      created_at: new Date(Date.now() - 18_000).toISOString(),
    },
    {
      id: 122,
      auction_id: auctionId,
      user: userOne,
      amount: mockPrice - 5000,
      status: 'accepted',
      reject_reason: null,
      idempotency_key: 'mock-bid-122',
      created_at: new Date(Date.now() - 48_000).toISOString(),
    },
  ]

  return {
    auction: {
      id: auctionId,
      title: '天然翡翠吊坠',
      description: '和田玉籽料，配 18K 金扣，直播间同步竞拍。',
      cover_url: null,
      images: [],
      stream_url: null,
      start_price: 0,
      price_step: 5000,
      ceiling_price: 500000,
      current_price: mockPrice,
      current_leader: mockLeader,
      start_time: futureIso(-120_000),
      end_time: futureIso(180_000),
      original_end_time: futureIso(180_000),
      extend_seconds: 30,
      extend_threshold: 30,
      status: 'active',
      viewer_count: 873,
      bid_count: bids.length,
      seller,
      created_at: futureIso(-600_000),
      updated_at: nowIso(),
    },
    top_bids: bids,
    last_event_seq: mockSeq,
    server_time: nowIso(),
  }
}

export function createMockBidResult(auctionId: number, amount: number, user: UserBrief) {
  mockSeq += 1
  mockBidId += 1
  mockPrice = amount
  mockLeader = user

  const bid: Bid = {
    id: mockBidId,
    auction_id: auctionId,
    user,
    amount,
    status: 'accepted',
    reject_reason: null,
    idempotency_key: `mock-bid-${mockBidId}`,
    created_at: nowIso(),
  }

  return {
    bid,
    auction_version: mockSeq,
    current_price: amount,
    current_leader: user,
    extended: false,
    new_end_time: futureIso(180_000),
    server_time: nowIso(),
  }
}

export function createMockBidEvent(auctionId: number, bid: Bid): EventEnvelope {
  return {
    type: 'bid_update',
    event_id: `evt_${auctionId}_${mockSeq}`,
    auction_id: auctionId,
    seq: mockSeq,
    server_time: nowIso(),
    data: {
      auction_version: mockSeq,
      current_price: bid.amount,
      current_leader: bid.user,
      latest_bid: bid,
      top_bids: [bid],
    },
  }
}
