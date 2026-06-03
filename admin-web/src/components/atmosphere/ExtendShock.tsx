// 延时触发：「⏰ 延时 N 秒！」从顶部弹入 + 一道冲击波。
import { useEffect } from 'react'
import { AnimatePresence, motion, useReducedMotion } from 'framer-motion'
import { playWhoosh } from '../../lib/sound'

interface Props {
  show: boolean
  seconds: number
  sound?: boolean
  onClose?: () => void
}

export function ExtendShock({ show, seconds, sound = true, onClose }: Props) {
  const reduce = useReducedMotion()

  useEffect(() => {
    if (show) {
      if (sound) playWhoosh()
      const t = setTimeout(() => onClose?.(), 1800)
      return () => clearTimeout(t)
    }
  }, [show, sound, onClose])

  return (
    <AnimatePresence>
      {show && (
        <motion.div
          initial={{ y: -80, opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          exit={{ y: -80, opacity: 0 }}
          transition={{ type: 'spring', stiffness: 300, damping: 20 }}
          style={{
            position: 'fixed',
            top: 24,
            left: '50%',
            transform: 'translateX(-50%)',
            zIndex: 1100,
            padding: '12px 28px',
            borderRadius: 999,
            background: 'linear-gradient(135deg,#7c3aed,#db2777)',
            color: '#fff',
            fontWeight: 800,
            fontSize: 18,
            boxShadow: '0 8px 30px rgba(124,58,237,0.5)',
          }}
        >
          ⏰ 延时 {seconds} 秒！
          {!reduce && (
            <motion.span
              initial={{ scale: 0, opacity: 0.6 }}
              animate={{ scale: 6, opacity: 0 }}
              transition={{ duration: 0.8 }}
              style={{
                position: 'absolute',
                inset: 0,
                borderRadius: 999,
                border: '2px solid rgba(255,255,255,0.7)',
                pointerEvents: 'none',
              }}
            />
          )}
        </motion.div>
      )}
    </AnimatePresence>
  )
}
