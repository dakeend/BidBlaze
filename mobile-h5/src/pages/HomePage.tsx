import { useCallback, useEffect, useState } from 'react'
import { Package, RefreshCw, ArrowRight, Clock3, Users } from 'lucide-react'
import { apiClient } from '../lib/api-client'
import { formatMoney, formatRemaining, toServerMs } from '../lib/time'
import { useServerTime } from '../hooks/useServerTime'
import type { Auction } from '../lib/types'

type AuctionListData = {
  list: Auction[]
  total: number
  page: number
  size: number
  server_time: string
}

export function HomePage({ onEnter, onViewOrders }: { onEnter: (auctionId: number) => void; onViewOrders: () => void }) {
  const [auctions, setAuctions] = useState<Auction[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const { serverNow } = useServerTime(0)

  const fetchList = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const resp = await apiClient.get<{ code: number; msg: string; data: AuctionListData }>(
        '/api/auctions',
        { params: { size: 50 } },
      )
      if (resp.data.code === 0) {
        setAuctions(resp.data.data.list)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchList()
  }, [fetchList])

  const activeList = auctions.filter((a) => a.status === 'active')
  const pendingList = auctions.filter((a) => a.status === 'pending')
  const endedList = auctions.filter((a) => a.status === 'ended' || a.status === 'cancelled')

  return (
    <div className="home-page">
      <header className="home-header">
        <h1>🔨 直播竞拍</h1>
        <div style={{ display: 'flex', gap: 8 }}>
          <button className="icon-button" onClick={onViewOrders} aria-label="我的订单">
            <Package size={18} />
          </button>
          <button className="icon-button" onClick={fetchList} disabled={loading} aria-label="刷新">
            <RefreshCw size={18} className={loading ? 'spin' : ''} />
          </button>
        </div>
      </header>

      {error && <p className="home-error">{error}</p>}

      <section className="auction-section">
        <h2 className="section-label">
          <span className="dot active" />
          竞拍中
          <span className="count">{activeList.length}</span>
        </h2>
        {activeList.length === 0 ? (
          <p className="empty-hint">暂无进行中的拍卖</p>
        ) : (
          activeList.map((a) => (
            <AuctionCard key={a.id} auction={a} serverNow={serverNow} onClick={() => onEnter(a.id)} />
          ))
        )}
      </section>

      <section className="auction-section">
        <h2 className="section-label">
          <span className="dot pending" />
          即将开始
          <span className="count">{pendingList.length}</span>
        </h2>
        {pendingList.length === 0 ? (
          <p className="empty-hint">暂无预告拍卖</p>
        ) : (
          pendingList.map((a) => (
            <AuctionCard key={a.id} auction={a} serverNow={serverNow} onClick={() => onEnter(a.id)} />
          ))
        )}
      </section>

      <section className="auction-section">
        <h2 className="section-label">
          <span className="dot ended" />
          已结束
          <span className="count">{endedList.length}</span>
        </h2>
        {endedList.slice(0, 5).map((a) => (
          <AuctionCard key={a.id} auction={a} serverNow={serverNow} onClick={() => onEnter(a.id)} muted />
        ))}
      </section>
    </div>
  )
}

function AuctionCard({
  auction,
  serverNow,
  onClick,
  muted,
}: {
  auction: Auction
  serverNow: () => number
  onClick: () => void
  muted?: boolean
}) {
  const remainingMs = auction.end_time ? toServerMs(auction.end_time) - serverNow() : 0

  return (
    <button className={`auction-card ${muted ? 'muted' : ''}`} onClick={onClick} type="button">
      <div className="card-left">
        <strong className="card-title">{auction.title}</strong>
        <span className="card-seller">{auction.seller.nickname}</span>
        <span className="card-meta">
          {auction.status === 'active' ? (
            <>
              <Clock3 size={13} />
              {formatRemaining(remainingMs)}
            </>
          ) : auction.status === 'pending' ? (
            '即将开始'
          ) : (
            '已结束'
          )}
          <Users size={13} />
          {auction.viewer_count || 0}
        </span>
      </div>
      <div className="card-right">
        <span className="card-price">{formatMoney(auction.current_price)}</span>
        {auction.current_leader && (
          <span className="card-leader">领先: {auction.current_leader.nickname}</span>
        )}
        <ArrowRight size={16} className="card-arrow" />
      </div>
    </button>
  )
}
