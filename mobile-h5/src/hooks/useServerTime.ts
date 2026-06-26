import { useCallback, useEffect, useState } from 'react'
import { getAuctionStatus } from '../lib/api-client'
import { createServerOffset } from '../lib/time'

export function useServerTime(auctionId: number) {
  const [serverOffset, setServerOffset] = useState(0)
  const [lastCalibratedAt, setLastCalibratedAt] = useState<number | null>(null)

  const calibrate = useCallback((serverTime: string) => {
    setServerOffset(createServerOffset(serverTime))
    setLastCalibratedAt(Date.now())
  }, [])

  const refresh = useCallback(async () => {
    if (!auctionId || auctionId <= 0) return
    try {
      const snapshot = await getAuctionStatus(auctionId)
      calibrate(snapshot.server_time)
      return snapshot
    } catch {
      // 静默失败，不影响页面渲染
    }
  }, [auctionId, calibrate])

  const serverNow = useCallback(() => Date.now() + serverOffset, [serverOffset])

  useEffect(() => {
    const kickoff = window.setTimeout(() => {
      void refresh()
    }, 0)
    const timer = window.setInterval(() => {
      void refresh()
    }, 30_000)

    return () => {
      window.clearTimeout(kickoff)
      window.clearInterval(timer)
    }
  }, [refresh])

  return {
    serverOffset,
    lastCalibratedAt,
    serverNow,
    calibrate,
    refresh,
  }
}
