import { useCallback, useEffect, useRef, useState } from 'react'
import { getAuctionStatus, getEventsAfter, wsBaseUrl } from '../lib/api-client'
import { getAuthToken } from '../lib/auth'
import type { EventEnvelope } from '../lib/types'
import { useAuctionStore } from '../store/auctionStore'

const reconnectBackoff = [1000, 2000, 5000, 10000]
const pollingAfterMs = 15_000
const pollingIntervalMs = 2000
const pingIntervalMs = 25_000

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
  const manualReconnectRef = useRef<() => void>(() => undefined)

  const refreshSnapshot = useCallback(async () => {
    const snapshot = await getAuctionStatus(auctionId)
    applySnapshot(snapshot)
    return snapshot
  }, [applySnapshot, auctionId])

  useEffect(() => {
    let disposed = false
    let socket: WebSocket | null = null
    let reconnectTimer: number | null = null
    let pingTimer: number | null = null
    let pollingTimer: number | null = null
    let reconnectAttempt = 0
    let disconnectedAt = 0

    const stopPing = () => {
      if (pingTimer !== null) {
        window.clearInterval(pingTimer)
        pingTimer = null
      }
    }

    const stopPolling = () => {
      if (pollingTimer !== null) {
        window.clearInterval(pollingTimer)
        pollingTimer = null
      }
    }

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

    const startPolling = () => {
      if (pollingTimer !== null) {
        return
      }
      setConnectionState('polling')
      void refreshSnapshot()
      pollingTimer = window.setInterval(() => {
        void refreshSnapshot()
      }, pollingIntervalMs)
    }

    const scheduleReconnect = () => {
      if (disposed) {
        return
      }

      if (disconnectedAt === 0) {
        disconnectedAt = Date.now()
      }

      const offlineMs = Date.now() - disconnectedAt
      if (offlineMs >= pollingAfterMs) {
        startPolling()
      } else {
        setConnectionState('reconnecting')
      }

      const delay = reconnectBackoff[Math.min(reconnectAttempt, reconnectBackoff.length - 1)]
      reconnectAttempt += 1
      reconnectTimer = window.setTimeout(connect, delay)
    }

    const connect = () => {
      if (disposed) {
        return
      }

      stopPing()
      if (socket) {
        socket.onclose = null
        socket.onerror = null
        socket.close()
      }

      const lastSeq = useAuctionStore.getState().lastSeq
      const url = `${wsBaseUrl()}/ws/auction/${auctionId}?token=${encodeURIComponent(
        getAuthToken(),
      )}&last_seq=${lastSeq}`

      try {
        socket = new WebSocket(url)
      } catch (error) {
        setLastError(error instanceof Error ? error.message : 'WebSocket 创建失败')
        scheduleReconnect()
        return
      }

      socket.onopen = () => {
        reconnectAttempt = 0
        disconnectedAt = 0
        stopPolling()
        setLastError(null)
        setConnectionState('connected')
        pingTimer = window.setInterval(() => {
          if (socket?.readyState === WebSocket.OPEN) {
            socket.send(JSON.stringify({ type: 'ping', client_time: new Date().toISOString() }))
          }
        }, pingIntervalMs)
      }

      socket.onmessage = (message) => {
        try {
          const payload = JSON.parse(message.data)
          if (isEventEnvelope(payload)) {
            void handleServerEvent(payload)
          }
        } catch (error) {
          setLastError(error instanceof Error ? error.message : 'WebSocket 消息解析失败')
        }
      }

      socket.onerror = () => {
        setLastError('WebSocket 连接异常')
      }

      socket.onclose = () => {
        stopPing()
        if (!disposed) {
          void refreshSnapshot()
          scheduleReconnect()
        }
      }
    }

    manualReconnectRef.current = connect

    const handleVisibilityChange = () => {
      if (document.visibilityState === 'visible') {
        void refreshSnapshot()
      }
    }

    document.addEventListener('visibilitychange', handleVisibilityChange)
    void refreshSnapshot().finally(connect)

    return () => {
      disposed = true
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      stopPing()
      stopPolling()
      if (reconnectTimer !== null) {
        window.clearTimeout(reconnectTimer)
      }
      if (socket) {
        socket.onclose = null
        socket.onerror = null
        socket.close()
      }
    }
  }, [applyEvent, refreshSnapshot, auctionId, setConnectionState])

  return {
    lastError,
    reconnect: () => manualReconnectRef.current(),
  }
}
