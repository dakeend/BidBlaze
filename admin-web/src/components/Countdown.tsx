// 倒计时显示：基于 server_time 偏移 + requestAnimationFrame（避免 setInterval 漂移）。
import { useEffect, useRef, useState } from 'react'
import { fmtRemaining, remainingMs } from '../lib/time'

interface Props {
  endTime: string | null | undefined
  /** 剩余 ≤ 此毫秒数变红（默认 10s） */
  dangerMs?: number
  onEnd?: () => void
}

export function Countdown({ endTime, dangerMs = 10000, onEnd }: Props) {
  const [ms, setMs] = useState(() => remainingMs(endTime))
  const raf = useRef<number>(0)
  const ended = useRef(false)

  useEffect(() => {
    ended.current = false
    const tick = () => {
      const left = remainingMs(endTime)
      setMs(left)
      if (left <= 0 && !ended.current) {
        ended.current = true
        onEnd?.()
      }
      raf.current = requestAnimationFrame(tick)
    }
    raf.current = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf.current)
  }, [endTime, onEnd])

  const danger = ms > 0 && ms <= dangerMs
  return (
    <span style={{ fontVariantNumeric: 'tabular-nums', color: danger ? '#cf1322' : undefined, fontWeight: danger ? 700 : 400 }}>
      {ms <= 0 ? '已结束' : fmtRemaining(ms)}
    </span>
  )
}
