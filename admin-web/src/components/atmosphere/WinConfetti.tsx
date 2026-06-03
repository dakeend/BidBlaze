// 成交：撒花 + 赢家头像放大 + 成交价高亮。纯 DOM + framer-motion，不用 lottie。
import { useEffect } from 'react'
import { AnimatePresence, motion, useReducedMotion } from 'framer-motion'
import { Avatar } from 'antd'
import { centsToYuanStr } from '../../lib/format'
import { playDing } from '../../lib/sound'

interface Props {
  show: boolean
  winnerName: string
  winnerAvatar?: string | null
  finalCents: number
  sound?: boolean
  onClose?: () => void
}

const COLORS = ['#ffd700', '#ff5252', '#40c4ff', '#69f0ae', '#e040fb', '#ffab40']

// 在模块加载时一次性生成（随机性不需要每次 render 重算，也避免 render 期调用 Math.random）。
const PIECES = Array.from({ length: 80 }, (_, i) => ({
  id: i,
  x: Math.random() * 100,
  delay: Math.random() * 0.5,
  duration: 1.8 + Math.random() * 1.4,
  color: COLORS[i % COLORS.length],
  size: 6 + Math.random() * 8,
  rotate: Math.random() * 360,
}))

export function WinConfetti({ show, winnerName, winnerAvatar, finalCents, sound = true, onClose }: Props) {
  const reduce = useReducedMotion()

  useEffect(() => {
    if (show && sound) playDing()
  }, [show, sound])

  return (
    <AnimatePresence>
      {show && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          onClick={onClose}
          style={{ position: 'fixed', inset: 0, zIndex: 1200, overflow: 'hidden', background: 'rgba(0,0,0,0.45)' }}
        >
          {!reduce &&
            PIECES.map((p) => (
              <motion.div
                key={p.id}
                initial={{ y: -40, x: `${p.x}vw`, rotate: 0, opacity: 1 }}
                animate={{ y: '110vh', rotate: p.rotate + 360 }}
                transition={{ duration: p.duration, delay: p.delay, ease: 'easeIn' }}
                style={{
                  position: 'absolute',
                  width: p.size,
                  height: p.size * 0.4,
                  background: p.color,
                  borderRadius: 2,
                }}
              />
            ))}
          <motion.div
            initial={{ scale: 0.5, opacity: 0 }}
            animate={{ scale: 1, opacity: 1 }}
            transition={{ type: 'spring', stiffness: 260, damping: 18 }}
            style={{
              position: 'absolute',
              top: '50%',
              left: '50%',
              transform: 'translate(-50%,-50%)',
              textAlign: 'center',
              color: '#fff',
            }}
          >
            <motion.div
              animate={reduce ? undefined : { scale: [1, 1.12, 1] }}
              transition={{ duration: 1.2, repeat: Infinity }}
            >
              <Avatar size={96} src={winnerAvatar || undefined} style={{ border: '3px solid #ffd700' }}>
                {winnerName.slice(0, 1)}
              </Avatar>
            </motion.div>
            <div style={{ fontSize: 22, marginTop: 12, fontWeight: 700 }}>🎉 成交！</div>
            <div style={{ fontSize: 18, marginTop: 4 }}>{winnerName}</div>
            <div style={{ fontSize: 40, marginTop: 8, fontWeight: 900, color: '#ffd700' }}>
              {centsToYuanStr(finalCents)}
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
