// 「👑 你正在领先」常驻徽章，金色光晕呼吸。
import { motion, useReducedMotion } from 'framer-motion'

export function LeaderBadge({ text = '👑 你正在领先' }: { text?: string }) {
  const reduce = useReducedMotion()
  return (
    <motion.div
      animate={reduce ? undefined : { boxShadow: ['0 0 0 0 rgba(255,193,7,0.6)', '0 0 24px 6px rgba(255,193,7,0.55)', '0 0 0 0 rgba(255,193,7,0.6)'] }}
      transition={{ duration: 1.8, repeat: Infinity, ease: 'easeInOut' }}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 6,
        padding: '6px 16px',
        borderRadius: 999,
        background: 'linear-gradient(135deg,#ffd700,#ffae00)',
        color: '#5a3b00',
        fontWeight: 700,
      }}
    >
      {text}
    </motion.div>
  )
}
