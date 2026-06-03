// 被超越：屏幕闪红 + 震动 + 「⚡ 被超越了，加价 ¥X 反超」CTA。
import { useEffect } from 'react'
import { AnimatePresence, motion, useReducedMotion } from 'framer-motion'
import { centsToYuanStr } from '../../lib/format'

interface Props {
  show: boolean
  diffCents: number
  onAct?: () => void
  onClose?: () => void
}

export function OvertakenFlash({ show, diffCents, onAct, onClose }: Props) {
  const reduce = useReducedMotion()

  useEffect(() => {
    if (show && !reduce && typeof navigator !== 'undefined' && navigator.vibrate) {
      navigator.vibrate([60, 40, 60])
    }
  }, [show, reduce])

  return (
    <AnimatePresence>
      {show && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={reduce ? { opacity: 1 } : { opacity: [0, 0.9, 0.2, 0.6] }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.6 }}
          onClick={onClose}
          style={{
            position: 'fixed',
            inset: 0,
            background: 'radial-gradient(circle at center, rgba(255,0,0,0.35), rgba(180,0,0,0.6))',
            display: 'grid',
            placeItems: 'center',
            zIndex: 1000,
          }}
        >
          <motion.div
            initial={{ scale: 0.8 }}
            animate={reduce ? { scale: 1 } : { scale: 1, x: [0, -8, 8, -6, 6, 0] }}
            transition={{ duration: 0.5 }}
            onClick={(e) => {
              e.stopPropagation()
              onAct?.()
            }}
            style={{
              padding: '20px 32px',
              borderRadius: 16,
              background: '#fff',
              color: '#cf1322',
              fontWeight: 800,
              fontSize: 20,
              cursor: 'pointer',
              boxShadow: '0 10px 40px rgba(0,0,0,0.4)',
            }}
          >
            ⚡ 被超越了！加价 {centsToYuanStr(diffCents)} 反超
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
