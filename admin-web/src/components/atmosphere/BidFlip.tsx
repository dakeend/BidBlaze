// 出价成功 → 价格数字翻牌动画。value 变化时新数字上滑进入。
import { AnimatePresence, motion, useReducedMotion } from 'framer-motion'
import { centsToYuanStr } from '../../lib/format'

export function BidFlip({ cents, size = 48 }: { cents: number; size?: number }) {
  const reduce = useReducedMotion()
  const text = centsToYuanStr(cents)
  return (
    <span style={{ display: 'inline-flex', overflow: 'hidden', height: size * 1.2, lineHeight: `${size * 1.2}px` }}>
      <AnimatePresence mode="popLayout" initial={false}>
        <motion.span
          key={cents}
          initial={reduce ? false : { y: size, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          exit={reduce ? undefined : { y: -size, opacity: 0 }}
          transition={{ type: 'spring', stiffness: 420, damping: 30 }}
          style={{ fontSize: size, fontWeight: 800, fontVariantNumeric: 'tabular-nums', color: '#cf1322' }}
        >
          {text}
        </motion.span>
      </AnimatePresence>
    </span>
  )
}
