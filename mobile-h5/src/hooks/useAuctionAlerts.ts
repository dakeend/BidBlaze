import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { getCurrentUser } from '../lib/auth'
import { playAlertTone, unlockAlertAudio } from '../lib/auction-audio'
import type { AuctionStatus, RealtimeEventRecord } from '../lib/types'

export type AuctionAlertType = 'outbid' | 'ending_soon' | 'extended' | 'ended'

export type AuctionAlert = {
  id: string
  type: AuctionAlertType
  message: string
  createdAt: number
}

type UseAuctionAlertsOptions = {
  latestEvent: RealtimeEventRecord | null
  remainingMs: number
  endTime?: string
  auctionStatus?: AuctionStatus
}

const alertDurationMs = 4200

function canVibrate() {
  return typeof navigator !== 'undefined' && 'vibrate' in navigator
}

function extendedSeconds(data: unknown) {
  const candidate = data as { extended_seconds?: number }
  return typeof candidate.extended_seconds === 'number' ? candidate.extended_seconds : 30
}

function endedMessage(data: unknown) {
  const candidate = data as { winner?: { nickname?: string } | null; final_price?: number | null }
  if (candidate.winner?.nickname) {
    return `${candidate.winner.nickname} 成交`
  }
  return '本场拍卖已结束'
}

export function useAuctionAlerts({
  latestEvent,
  remainingMs,
  endTime,
  auctionStatus,
}: UseAuctionAlertsOptions) {
  const [alerts, setAlerts] = useState<AuctionAlert[]>([])
  const processedEventIdsRef = useRef(new Set<string>())
  const alertTimersRef = useRef<number[]>([])
  const endingAlertedForRef = useRef<string | null>(null)
  const lastTickSecondRef = useRef<number | null>(null)
  const reducedMotion = useMemo(
    () =>
      typeof window !== 'undefined' &&
      window.matchMedia?.('(prefers-reduced-motion: reduce)').matches,
    [],
  )

  const dismissAlert = useCallback((id: string) => {
    setAlerts((current) => current.filter((alert) => alert.id !== id))
  }, [])

  const pushAlert = useCallback(
    (type: AuctionAlertType, message: string, idSeed: string) => {
      const id = `${type}-${idSeed}-${Date.now()}`
      setAlerts((current) => [{ id, type, message, createdAt: Date.now() }, ...current].slice(0, 4))
      const timer = window.setTimeout(() => dismissAlert(id), alertDurationMs)
      alertTimersRef.current.push(timer)
    },
    [dismissAlert],
  )

  const scheduleAlert = useCallback(
    (type: AuctionAlertType, message: string, idSeed: string) => {
      const timer = window.setTimeout(() => pushAlert(type, message, idSeed), 0)
      alertTimersRef.current.push(timer)
    },
    [pushAlert],
  )

  useEffect(() => {
    return () => {
      for (const timer of alertTimersRef.current) {
        window.clearTimeout(timer)
      }
      alertTimersRef.current = []
    }
  }, [])

  useEffect(() => {
    if (!latestEvent) {
      return
    }

    const { event, previousLeaderId, currentLeaderId } = latestEvent
    if (processedEventIdsRef.current.has(event.event_id)) {
      return
    }
    processedEventIdsRef.current.add(event.event_id)

    if (event.type === 'bid_update') {
      const currentUserId = getCurrentUser()?.id
      if (
        previousLeaderId === currentUserId &&
        currentLeaderId !== null &&
        currentLeaderId !== currentUserId
      ) {
        if (canVibrate()) {
          navigator.vibrate?.([70, 35, 70])
        }
        playAlertTone('outbid')
        scheduleAlert('outbid', '⚡ 你被超越了！', event.event_id)
      }
      return
    }

    if (event.type === 'auction_extended') {
      const seconds = extendedSeconds(event.data)
      playAlertTone('notice')
      scheduleAlert('extended', `⏰ 延时 ${seconds}s`, event.event_id)
      endingAlertedForRef.current = null
      lastTickSecondRef.current = null
      return
    }

    if (event.type === 'auction_ended') {
      playAlertTone('ended')
      scheduleAlert('ended', endedMessage(event.data), event.event_id)
    }
  }, [latestEvent, scheduleAlert])

  useEffect(() => {
    if (auctionStatus !== 'active' || !endTime || remainingMs <= 0) {
      lastTickSecondRef.current = null
      return
    }

    if (remainingMs <= 10_000) {
      if (endingAlertedForRef.current !== endTime) {
        endingAlertedForRef.current = endTime
        scheduleAlert('ending_soon', '即将结束，抓紧出价', `ending-${endTime}`)
      }

      const currentSecond = Math.ceil(remainingMs / 1000)
      if (lastTickSecondRef.current !== currentSecond) {
        lastTickSecondRef.current = currentSecond
        playAlertTone('tick')
      }
    } else {
      lastTickSecondRef.current = null
    }
  }, [auctionStatus, endTime, remainingMs, scheduleAlert])

  return {
    alerts,
    criticalEnding: auctionStatus === 'active' && remainingMs > 0 && remainingMs <= 10_000,
    dismissAlert,
    unlockAudio: unlockAlertAudio,
    reducedMotion,
  }
}
