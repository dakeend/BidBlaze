import { useEffect, useMemo, useRef, useState } from 'react'
import {
  ArrowLeft,
  Clock3,
  Crown,
  RefreshCw,
  Send,
  Users,
  Video,
  Wifi,
  WifiOff,
} from 'lucide-react'
import heroImg from '../assets/hero.png'
import { useAuctionAlerts } from '../hooks/useAuctionAlerts'
import { useAuctionSocket } from '../hooks/useAuctionSocket'
import { useBidButton } from '../hooks/useBidButton'
import { useServerTime } from '../hooks/useServerTime'
import { ApiCallError, getAuctionStatus, payOrder } from '../lib/api-client'
import { getCurrentUser } from '../lib/auth'
import { formatMoney, formatRemaining, toServerMs } from '../lib/time'
import { useAuctionStore } from '../store/auctionStore'

type AuctionRoomPageProps = {
  auctionId: number
  onBack?: () => void
}

function connectionText(state: string): string {
  if (state === 'connected') {
    return '已连接'
  }
  if (state === 'reconnecting') {
    return '连接中'
  }
  if (state === 'polling') {
    return '同步中'
  }
  return '已断开'
}

function connectionIcon(state: string) {
  if (state === 'connected') {
    return <Wifi size={15} aria-hidden="true" />
  }
  if (state === 'polling' || state === 'reconnecting') {
    return <RefreshCw size={15} aria-hidden="true" />
  }
  return <WifiOff size={15} aria-hidden="true" />
}

function statusText(status: string | undefined): string {
  if (status === 'active') {
    return '竞拍中'
  }
  if (status === 'pending') {
    return '未开始'
  }
  if (status === 'ended') {
    return '已结束'
  }
  if (status === 'cancelled') {
    return '已取消'
  }
  return '加载中'
}

export function AuctionRoomPage({ auctionId, onBack }: AuctionRoomPageProps) {
  const auction = useAuctionStore((state) => state.auction)
  const bids = useAuctionStore((state) => state.bids)
  const viewerCount = useAuctionStore((state) => state.viewerCount)
  const connectionState = useAuctionStore((state) => state.connectionState)
  const latestEvent = useAuctionStore((state) => state.lastRealtimeEvent)
  const ended = useAuctionStore((state) => state.ended)
  const closeEndedModal = useAuctionStore((state) => state.closeEndedModal)
  const { serverNow } = useServerTime(auctionId)
  const { lastError, reconnect } = useAuctionSocket(auctionId)
  const { buttonState, disabledReason, message, submitBid } = useBidButton(auctionId)

  // 直拉拍卖数据 + 兜底轮询，彻底解决卡「加载拍卖中」的问题
  const applySnapshot = useAuctionStore((s) => s.applySnapshot)
  useEffect(() => {
    let cancelled = false
    let attempt = 0
    const load = () => {
      getAuctionStatus(auctionId)
        .then((snap) => {
          if (!cancelled) applySnapshot(snap)
        })
        .catch(() => {
          // 失败后递增重试：0.3s, 0.6s, 1.2s, 2.4s...
          if (!cancelled) {
            attempt++
            setTimeout(load, Math.min(300 * Math.pow(2, attempt), 5000))
          }
        })
    }
    load()
    return () => { cancelled = true }
  }, [auctionId, applySnapshot])

  const [remainingMs, setRemainingMs] = useState(0)
  const [bidAmount, setBidAmount] = useState<number | null>(null)
  const { alerts, criticalEnding, dismissAlert } = useAuctionAlerts({
    latestEvent,
    remainingMs,
    endTime: auction?.end_time,
    auctionStatus: auction?.status,
  })

  const fenToYuan = (fen: number) => fen / 100
  const yuanToFen = (yuan: number) => Math.round(yuan * 100)

  const minBidAmount = useMemo(() => {
    if (!auction) {
      return 0
    }
    if (auction.current_price > 0) {
      return auction.current_price + auction.price_step
    }
    return Math.max(auction.start_price, auction.price_step)
  }, [auction])

  // bidAmount 存储分，bidAmountValue 也是分，输入框用元
  const bidAmountFen = bidAmount ?? minBidAmount

  useEffect(() => {
    let frame = 0

    const tick = () => {
      if (auction?.end_time) {
        setRemainingMs(toServerMs(auction.end_time) - serverNow())
      }
      frame = window.requestAnimationFrame(tick)
    }

    tick()
    return () => window.cancelAnimationFrame(frame)
  }, [auction?.end_time, serverNow])

  const submit = async () => {
    const amount = Math.max(bidAmountFen, minBidAmount)
    const result = await submitBid(amount)
    if (!result.ok && result.nextAmount) {
      setBidAmount(result.nextAmount)
    }
  }

  const placeStepBid = () => {
    setBidAmount(null)
    void submitBid(minBidAmount)
  }

  const [paying, setPaying] = useState(false)
  const [paid, setPaid] = useState(false)
  const currentUser = getCurrentUser()

  const handlePay = async () => {
    if (!ended.orderId) return
    setPaying(true)
    try {
      await payOrder(ended.orderId)
      setPaid(true)
    } catch (err) {
      // ignore — 可以重试
    } finally {
      setPaying(false)
    }
  }

  return (
    <>
      <header className="room-topbar">
          <button className="icon-button" type="button" aria-label="返回" onClick={() => onBack?.()}>
            <ArrowLeft size={20} aria-hidden="true" />
          </button>
          <div className="viewer-pill" aria-label="在线人数">
            <Users size={15} aria-hidden="true" />
            <span>{viewerCount || auction?.viewer_count || 0}</span>
          </div>
          <button className={`connection-pill ${connectionState}`} type="button" onClick={reconnect}>
            {connectionIcon(connectionState)}
            <span>{connectionText(connectionState)}</span>
          </button>
        </header>

        <div className="alert-stack" aria-live="polite" aria-atomic="false">
          {alerts.map((alert) => (
            <button
              className={`auction-alert-toast ${alert.type}`}
              key={alert.id}
              type="button"
              onClick={() => dismissAlert(alert.id)}
              aria-label="关闭提醒"
            >
              {alert.message}
            </button>
          ))}
        </div>

        <section className={`live-stage ${criticalEnding ? 'critical-ending' : ''}`}>
          {auction?.stream_url ? (
            <video className="live-video" src={auction.stream_url} poster={heroImg} playsInline muted autoPlay />
          ) : (
            <div className="live-placeholder">
              <img src={heroImg} alt="拍卖商品直播占位画面" />
              <div className="live-badge">
                <Video size={16} aria-hidden="true" />
                <span>直播占位</span>
              </div>
            </div>
          )}

          <div className="auction-overlay">
            <div>
              <span className="metric-label">当前价</span>
              <strong>{formatMoney(auction?.current_price)}</strong>
            </div>
            <div>
              <span className="metric-label">倒计时</span>
              <strong className={remainingMs <= 10_000 ? 'urgent-time' : ''}>
                {formatRemaining(remainingMs)}
              </strong>
            </div>
            <div>
              <span className="metric-label">领先者</span>
              <strong className="leader-name">
                {auction?.current_leader ? (
                  <>
                    <Crown size={15} aria-hidden="true" />
                    {auction.current_leader.nickname}
                  </>
                ) : (
                  '暂无'
                )}
              </strong>
            </div>
          </div>
        </section>

        <section className="auction-info">
          <div>
            <p className="auction-status">{statusText(auction?.status)}</p>
            <h1>{auction?.title ?? '加载拍卖中'}</h1>
          </div>
          <div className="time-card">
            <Clock3 size={17} aria-hidden="true" />
            <span>{auction?.end_time ? `结束 ${new Date(auction.end_time).toLocaleTimeString('zh-CN')}` : '校准中'}</span>
          </div>
        </section>

        <section className="bid-feed" aria-label="出价记录">
          <div className="section-title">
            <span>出价记录</span>
            <span>{bids.length} 条</span>
          </div>
          <div className="bid-list">
            {bids.map((bid, index) => (
              <article className="bid-row" key={bid.id} style={{ animationDelay: `${Math.min(index, 6) * 35}ms` }}>
                <div>
                  <strong>{bid.user.nickname}</strong>
                  <span>{new Date(bid.created_at).toLocaleTimeString('zh-CN')}</span>
                </div>
                <b>{formatMoney(bid.amount)}</b>
              </article>
            ))}
          </div>
        </section>

        <footer className="bid-dock">
          {(message || lastError || disabledReason) && (
            <p className={`dock-message ${lastError ? 'error' : ''}`}>{lastError || message || disabledReason}</p>
          )}
          <div className="bid-controls">
            <label className="amount-field">
              <span>出价金额</span>
              <input
                type="number"
                min={fenToYuan(minBidAmount)}
                step={fenToYuan(auction?.price_step || 100)}
                value={fenToYuan(bidAmountFen)}
                onChange={(event) => setBidAmount(yuanToFen(Number(event.target.value)))}
              />
            </label>
            <button
              className="step-button"
              type="button"
              disabled={Boolean(disabledReason)}
              onClick={placeStepBid}
            >
              加一手
            </button>
          </div>
          <button
            className={`primary-bid ${buttonState}`}
            type="button"
            disabled={buttonState !== 'idle'}
            onClick={submit}
          >
            <Send size={18} aria-hidden="true" />
            <span>{buttonState === 'pending' ? '提交中' : disabledReason || '立即出价'}</span>
          </button>
        </footer>

        {ended.open && (
          <div className="modal-layer" role="dialog" aria-modal="true" aria-label="成交结果">
            <div className="result-modal">
              <span className="modal-kicker">拍卖结束</span>
              <h2>{ended.winner ? `${ended.winner.nickname} 成交` : '本场流拍'}</h2>
              <p>{ended.finalPrice ? `成交价 ${formatMoney(ended.finalPrice)}` : '暂无有效出价'}</p>
              {ended.orderId && <p>订单号 {ended.orderId}</p>}
              {paid && <p style={{ color: '#10b981', fontWeight: 800 }}>✅ 支付成功</p>}
              {ended.winner && currentUser && ended.winner.id === currentUser.id && ended.orderId && !paid && (
                <button type="button" onClick={handlePay} disabled={paying}>
                  {paying ? '支付中...' : '💳 立即支付'}
                </button>
              )}
              <button type="button" onClick={closeEndedModal}>
                {paid ? '关闭' : '知道了'}
              </button>
            </div>
          </div>
        )}
    </>
  )
}
