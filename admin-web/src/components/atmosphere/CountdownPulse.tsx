// 倒计时 ≤10s：数字变红 + 心跳缩放 + 每秒滴答音。
import { useEffect, useRef, useState } from 'react'
import { motion, useReducedMotion } from 'framer-motion'
import { fmtRemaining, remainingMs } from '../../lib/time'
import { playTick } from '../../lib/sound'

interface Props {
  endTime: string | null | undefined
  /** ≤ 此秒数进入心跳 + 滴答（默认 10s） */
  thresholdSec?: number
  sound?: boolean
}

export function CountdownPulse({ endTime, thresholdSec = 10, sound = true }: Props) {
  const reduce = useReducedMotion()
  const [ms, setMs] = useState(() => remainingMs(endTime))
  const raf = useRef<number>(0)
  const lastSec = useRef(-1)

  useEffect(() => {
    const tick = () => {
      const left = remainingMs(endTime)
      setMs(left)
      const sec = Math.ceil(left / 1000)
      if (sound && sec !== lastSec.current && sec > 0 && sec <= thresholdSec) {
        playTick()
      }
      lastSec.current = sec
      raf.current = requestAnimationFrame(tick)
    }
    raf.current = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf.current)
  }, [endTime, thresholdSec, sound])

  const danger = ms > 0 && ms <= thresholdSec * 1000
  return (
    <motion.div
      animate={danger && !reduce ? { scale: [1, 1.18, 1] } : { scale: 1 }}
      transition={{ duration: 1, repeat: danger ? Infinity : 0 }}
      style={{
        fontSize: 40,
        fontWeight: 800,
        fontVariantNumeric: 'tabular-nums',
        color: danger ? '#cf1322' : '#222',
      }}
    >
      {ms <= 0 ? '00:00' : fmtRemaining(ms)}
    </motion.div>
  )
}
