import { create } from 'zustand'
import type {
  Auction,
  AuctionSnapshot,
  Bid,
  ConnectionState,
  EventEnvelope,
  RealtimeEventRecord,
  UserBrief,
} from '../lib/types'

type AuctionRoomState = {
  auction: Auction | null
  bids: Bid[]
  viewerCount: number
  lastSeq: number
  seenEventIds: Set<string>
  connectionState: ConnectionState
  lastServerTime: string | null
  lastRealtimeEvent: RealtimeEventRecord | null
  ended: {
    open: boolean
    winner: UserBrief | null
    finalPrice: number | null
    orderId: number | null
  }
  applySnapshot: (snapshot: AuctionSnapshot) => void
  applyEvent: (event: EventEnvelope) => void
  applyBidResponse: (bid: Bid, currentPrice: number, leader: UserBrief, serverTime: string) => void
  setConnectionState: (state: ConnectionState) => void
  closeEndedModal: () => void
}

function mergeBid(bids: Bid[], bid: Bid): Bid[] {
  const next = [bid, ...bids.filter((item) => item.id !== bid.id)]
  return next.slice(0, 30)
}

export const useAuctionStore = create<AuctionRoomState>((set, get) => ({
  auction: null,
  bids: [],
  viewerCount: 0,
  lastSeq: 0,
  seenEventIds: new Set(),
  connectionState: 'disconnected',
  lastServerTime: null,
  lastRealtimeEvent: null,
  ended: {
    open: false,
    winner: null,
    finalPrice: null,
    orderId: null,
  },
  applySnapshot: (snapshot) => {
    set({
      auction: snapshot.auction,
      bids: snapshot.top_bids,
      viewerCount: snapshot.auction.viewer_count,
      lastSeq: snapshot.last_event_seq,
      seenEventIds: new Set(),
      lastServerTime: snapshot.server_time,
      lastRealtimeEvent: null,
      ended: {
        open: snapshot.auction.status === 'ended',
        winner: snapshot.auction.current_leader,
        finalPrice: snapshot.auction.status === 'ended' ? snapshot.auction.current_price : null,
        orderId: null,
      },
    })
  },
  applyEvent: (event) => {
    const state = get()
    if (state.seenEventIds.has(event.event_id)) {
      return
    }

    const seenEventIds = new Set(state.seenEventIds)
    seenEventIds.add(event.event_id)

    if (event.type === 'viewer_count') {
      const data = event.data as { viewer_count?: number }
      set({
        viewerCount: data.viewer_count ?? state.viewerCount,
        seenEventIds,
        lastServerTime: event.server_time,
      })
      return
    }

    if (event.seq <= state.lastSeq && event.type !== 'snapshot') {
      set({ seenEventIds, lastServerTime: event.server_time })
      return
    }

    if (event.type === 'snapshot') {
      const data = event.data as Partial<AuctionSnapshot>
      if (data.auction) {
        set({
          auction: data.auction,
          bids: data.top_bids ?? state.bids,
          viewerCount: data.auction.viewer_count ?? state.viewerCount,
          lastSeq: event.seq,
          seenEventIds,
          lastServerTime: event.server_time,
        })
      }
      return
    }

    if (event.type === 'bid_update') {
      const data = event.data as {
        current_price?: number
        current_leader?: UserBrief
        latest_bid?: Bid
        top_bids?: Bid[]
      }
      const previousPrice = state.auction?.current_price ?? null
      const currentPrice = data.current_price ?? previousPrice
      const previousLeaderId = state.auction?.current_leader?.id ?? null
      const currentLeaderId = data.current_leader?.id ?? previousLeaderId
      set({
        auction: state.auction
          ? {
              ...state.auction,
              current_price: currentPrice ?? state.auction.current_price,
              current_leader: data.current_leader ?? state.auction.current_leader,
              bid_count: (state.auction.bid_count ?? 0) + (data.latest_bid ? 1 : 0),
            }
          : state.auction,
        bids: data.latest_bid
          ? mergeBid(state.bids, data.latest_bid)
          : data.top_bids
            ? data.top_bids
            : state.bids,
        lastSeq: event.seq,
        seenEventIds,
        lastServerTime: event.server_time,
        lastRealtimeEvent: {
          event,
          previousLeaderId,
          currentLeaderId,
          previousPrice,
          currentPrice,
        },
      })
      return
    }

    if (event.type === 'auction_extended') {
      const data = event.data as { new_end_time?: string }
      set({
        auction: state.auction
          ? {
              ...state.auction,
              end_time: data.new_end_time ?? state.auction.end_time,
            }
          : state.auction,
        lastSeq: event.seq,
        seenEventIds,
        lastServerTime: event.server_time,
        lastRealtimeEvent: {
          event,
          previousLeaderId: state.auction?.current_leader?.id ?? null,
          currentLeaderId: state.auction?.current_leader?.id ?? null,
          previousPrice: state.auction?.current_price ?? null,
          currentPrice: state.auction?.current_price ?? null,
        },
      })
      return
    }

    if (event.type === 'auction_started') {
      const data = event.data as { auction?: Auction }
      set({
        auction: data.auction ?? (state.auction ? { ...state.auction, status: 'active' } : null),
        lastSeq: event.seq,
        seenEventIds,
        lastServerTime: event.server_time,
        lastRealtimeEvent: {
          event,
          previousLeaderId: state.auction?.current_leader?.id ?? null,
          currentLeaderId: state.auction?.current_leader?.id ?? null,
          previousPrice: state.auction?.current_price ?? null,
          currentPrice: state.auction?.current_price ?? null,
        },
      })
      return
    }

    if (event.type === 'auction_ended') {
      const data = event.data as {
        auction?: Auction
        winner?: UserBrief | null
        final_price?: number | null
        order_id?: number | null
      }
      set({
        auction: data.auction ?? (state.auction ? { ...state.auction, status: 'ended' } : null),
        lastSeq: event.seq,
        seenEventIds,
        lastServerTime: event.server_time,
        ended: {
          open: true,
          winner: data.winner ?? null,
          finalPrice: data.final_price ?? null,
          orderId: data.order_id ?? null,
        },
        lastRealtimeEvent: {
          event,
          previousLeaderId: state.auction?.current_leader?.id ?? null,
          currentLeaderId: data.winner?.id ?? state.auction?.current_leader?.id ?? null,
          previousPrice: state.auction?.current_price ?? null,
          currentPrice: data.final_price ?? state.auction?.current_price ?? null,
        },
      })
      return
    }

    if (event.type === 'auction_cancelled') {
      set({
        auction: state.auction ? { ...state.auction, status: 'cancelled' } : null,
        lastSeq: event.seq,
        seenEventIds,
        lastServerTime: event.server_time,
        lastRealtimeEvent: {
          event,
          previousLeaderId: state.auction?.current_leader?.id ?? null,
          currentLeaderId: state.auction?.current_leader?.id ?? null,
          previousPrice: state.auction?.current_price ?? null,
          currentPrice: state.auction?.current_price ?? null,
        },
      })
    }
  },
  applyBidResponse: (bid, currentPrice, leader, serverTime) => {
    const state = get()
    set({
      auction: state.auction
        ? {
            ...state.auction,
            current_price: currentPrice,
            current_leader: leader,
            bid_count: (state.auction.bid_count ?? 0) + 1,
          }
        : state.auction,
      bids: mergeBid(state.bids, bid),
      lastServerTime: serverTime,
    })
  },
  setConnectionState: (connectionState) => set({ connectionState }),
  closeEndedModal: () =>
    set((state) => ({
      ended: {
        ...state.ended,
        open: false,
      },
    })),
}))
