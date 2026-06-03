import { useCallback, useEffect, useRef, useState } from 'react'
import { getAuctionStatus, getEventsAfter, wsBaseUrl } from '../lib/api-client'
import { getAuthToken } from '../lib/auth'
import { ConnectionManager } from '../lib/connection-manager'
import type { ConnectionState, EventEnvelope } from '../lib/types'
import { useAuctionStore } from '../store/auctionStore'

type AuctionConnectionDebug = {
  closeSocket: () => void
  rejectReconnects: () => void
  allowReconnects: () => void
  forcePolling: () => void
  reconnect: () => void
  state: () => ConnectionState
}

declare global {
  interface Window {
    __auctionConnectionDebug?: AuctionConnectionDebug
  }
}

function isEventEnvelope(value: unknown): value is EventEnvelope {
  if (!value || typeof value !== 'object') {
    return false
  }
  const candidate = value as Partial<EventEnvelope>
  return (
    typeof candidate.type === 'string' &&
    typeof candidate.auction_id === 'number' &&
    typeof candidate.seq === 'number' &&
    typeof candidate.event_id === 'string'
  )
}

export function useAuctionSocket(auctionId: number) {
  const applySnapshot = useAuctionStore((state) => state.applySnapshot)
  const applyEvent = useAuctionStore((state) => state.applyEvent)
  const setConnectionState = useAuctionStore((state) => state.setConnectionState)
  const [lastError, setLastError] = useState<string | null>(null)
  const managerRef = useRef<ConnectionManager | null>(null)

  const refreshSnapshot = useCallback(async () => {
    const snapshot = await getAuctionStatus(auctionId)
    applySnapshot(snapshot)
    return snapshot
  }, [applySnapshot, auctionId])

  useEffect(() => {
    let disposed = false

    const compensate = async (afterSeq: number) => {
      let cursor = afterSeq

      for (let page = 0; page < 5; page += 1) {
        const result = await getEventsAfter(auctionId, cursor)
        if (disposed) {
          return
        }
        if (result.snapshot_required) {
          await refreshSnapshot()
          return
        }

        for (const event of result.events) {
          applyEvent(event)
          if (event.type !== 'viewer_count') {
            cursor = Math.max(cursor, event.seq)
          }
        }

        if (!result.has_more) {
          return
        }
      }

      await refreshSnapshot()
    }

    const handleServerEvent = async (event: EventEnvelope) => {
      if (event.type !== 'snapshot' && event.type !== 'viewer_count') {
        const lastSeq = useAuctionStore.getState().lastSeq
        if (event.seq > lastSeq + 1) {
          await compensate(lastSeq)
          if (disposed) {
            return
          }
          if (event.seq <= useAuctionStore.getState().lastSeq) {
            return
          }
        }
      }
      applyEvent(event)
    }

    const manager = new ConnectionManager({
      buildUrl: () => {
        const lastSeq = useAuctionStore.getState().lastSeq
        return `${wsBaseUrl()}/ws/auction/${auctionId}?token=${encodeURIComponent(
          getAuthToken(),
        )}&last_seq=${lastSeq}`
      },
      refreshStatus: async () => {
        await refreshSnapshot()
      },
      onMessage: (payload) => {
        if (isEventEnvelope(payload)) {
          void handleServerEvent(payload)
        }
      },
      onStateChange: setConnectionState,
      onError: setLastError,
    })

    managerRef.current = manager
    const debugControls: AuctionConnectionDebug = {
      closeSocket: () => manager.closeSocketForTest(),
      rejectReconnects: () => manager.rejectReconnectsForTest(),
      allowReconnects: () => manager.allowReconnectsForTest(),
      forcePolling: () => manager.forcePollingForTest(),
      reconnect: () => manager.reconnect(),
      state: () => manager.getState(),
    }

    if (import.meta.env.DEV) {
      window.__auctionConnectionDebug = debugControls
    }

    manager.start()

    return () => {
      disposed = true
      manager.stop()
      if (window.__auctionConnectionDebug === debugControls) {
        delete window.__auctionConnectionDebug
      }
      if (managerRef.current === manager) {
        managerRef.current = null
      }
    }
  }, [applyEvent, refreshSnapshot, auctionId, setConnectionState])

  return {
    lastError,
    reconnect: () => managerRef.current?.reconnect(),
  }
}
