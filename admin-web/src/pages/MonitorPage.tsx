// PC 直播间监控页（P5）。只读视图，轮询 /status（联调后可替换为共享 useAuctionSocket）。
// 实时：当前价 / leader / 出价流 / viewer_count / 剩余时间；卖家可一键取消（active）。
import { useCallback, useEffect, useRef, useState } from 'react'
import { Card, Row, Col, Statistic, Button, Avatar, Tag, Typography, App, Result } from 'antd'
import { AnimatePresence, motion } from 'framer-motion'
import { useNavigate, useParams } from 'react-router-dom'
import { api, ApiError } from '../lib/api-client'
import { syncServerTime } from '../lib/time'
import { centsToYuanStr } from '../lib/format'
import { BidFlip } from '../components/atmosphere/BidFlip'
import { LeaderBadge } from '../components/atmosphere/LeaderBadge'
import { CountdownPulse } from '../components/atmosphere/CountdownPulse'
import { ExtendShock } from '../components/atmosphere/ExtendShock'
import { WinConfetti } from '../components/atmosphere/WinConfetti'
import heroImg from '../assets/hero.png'
import type { AuctionSnapshot } from '../lib/types'

export function MonitorPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { message, modal } = App.useApp()
  const [snap, setSnap] = useState<AuctionSnapshot | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [extendShow, setExtendShow] = useState(false)
  const [winShow, setWinShow] = useState(false)
  const prevEnd = useRef<string | null>(null)
  const prevStatus = useRef<string | null>(null)

  const poll = useCallback(async () => {
    try {
      const s = await api.get<AuctionSnapshot>(`/api/auctions/${id}/status`)
      if (s.server_time) syncServerTime(s.server_time)
      // 检测延时（end_time 变大）
      if (prevEnd.current && s.auction.end_time > prevEnd.current) setExtendShow(true)
      prevEnd.current = s.auction.end_time
      // 检测成交
      if (prevStatus.current === 'active' && s.auction.status === 'ended') setWinShow(true)
      prevStatus.current = s.auction.status
      setSnap(s)
    } catch (e) {
      setError(e instanceof ApiError ? e.message : '加载失败')
    }
  }, [id])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- 挂载即拉取 + 轮询
    poll()
    const t = setInterval(poll, 1500)
    return () => clearInterval(t)
  }, [poll])

  const cancel = () => {
    if (!snap) return
    modal.confirm({
      title: '确认取消该拍卖？',
      content: '取消后将立即向所有买家广播，且不可恢复。',
      okText: '确认取消',
      okType: 'danger',
      onOk: async () => {
        try {
          await api.post(`/api/auctions/${id}/cancel`)
          message.success('已取消')
          poll()
        } catch (e) {
          message.error(e instanceof ApiError ? e.message : '取消失败')
        }
      },
    })
  }

  if (error)
    return (
      <Result status="error" title="加载失败" subTitle={error} extra={<Button onClick={() => navigate('/auctions')}>返回列表</Button>} />
    )
  if (!snap) return <Card loading style={{ marginTop: 24 }} />

  const a = snap.auction
  const isActive = a.status === 'active'

  return (
    <>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          🔴 监控 · {a.title} <Tag>{a.status}</Tag>
        </Typography.Title>
        <div>
          <Button onClick={() => navigate('/auctions')} style={{ marginRight: 8 }}>
            返回列表
          </Button>
          {isActive && (
            <Button danger onClick={cancel}>
              取消拍卖
            </Button>
          )}
        </div>
      </div>

      {/* 直播画面区 */}
      <Card
        style={{ marginBottom: 16, overflow: 'hidden', padding: 0 }}
        bodyStyle={{ padding: 0, position: 'relative' }}
      >
        <div style={{ position: 'relative', width: '100%', height: 320, background: '#1a1a2e', overflow: 'hidden' }}>
          {a.stream_url ? (
            <video src={a.stream_url} style={{ width: '100%', height: '100%', objectFit: 'cover' }} playsInline muted autoPlay loop controls />
          ) : (
            <>
              <img
                src={heroImg}
                alt="直播占位画面"
                style={{ width: '100%', height: '100%', objectFit: 'cover', opacity: 0.7 }}
              />
              <div
                style={{
                  position: 'absolute',
                  top: 12,
                  left: 12,
                  background: 'rgba(220,38,38,0.85)',
                  color: '#fff',
                  padding: '2px 10px',
                  borderRadius: 4,
                  fontSize: 12,
                  fontWeight: 700,
                }}
              >
                🔴 LIVE 占位
              </div>
            </>
          )}
          {/* 数据叠层 */}
          <div
            style={{
              position: 'absolute',
              bottom: 0,
              left: 0,
              right: 0,
              display: 'grid',
              gridTemplateColumns: '1fr 1fr 1fr',
              padding: '16px 20px',
              background: 'linear-gradient(transparent, rgba(0,0,0,0.7))',
              color: '#fff',
            }}
          >
            <div>
              <div style={{ fontSize: 12, opacity: 0.8 }}>当前价</div>
              <div style={{ fontSize: 24, fontWeight: 900 }}>{centsToYuanStr(a.current_price)}</div>
            </div>
            <div style={{ textAlign: 'center' }}>
              <div style={{ fontSize: 12, opacity: 0.8 }}>剩余时间</div>
              <div style={{ fontSize: 24, fontWeight: 900 }}>
                {isActive ? <CountdownPulse endTime={a.end_time} sound={false} /> : '—'}
              </div>
            </div>
            <div style={{ textAlign: 'right' }}>
              <div style={{ fontSize: 12, opacity: 0.8 }}>领先者</div>
              <div style={{ fontSize: 20, fontWeight: 800 }}>
                {a.current_leader ? (
                  <>
                    <span style={{ color: '#fbbf24', marginRight: 4 }}>👑</span>
                    {a.current_leader.nickname}
                  </>
                ) : (
                  '暂无'
                )}
              </div>
            </div>
          </div>
        </div>
      </Card>

      <Row gutter={16}>
        <Col span={8}>
          <Card title="当前价">
            <BidFlip cents={a.current_price} />
            <div style={{ marginTop: 12 }}>
              {a.current_leader ? (
                <LeaderBadge text={`👑 领先：${a.current_leader.nickname}`} />
              ) : (
                <Typography.Text type="secondary">暂无出价</Typography.Text>
              )}
            </div>
          </Card>
        </Col>
        <Col span={8}>
          <Card title="剩余时间">
            {isActive ? <CountdownPulse endTime={a.end_time} sound={false} /> : <Typography.Text>—</Typography.Text>}
          </Card>
        </Col>
        <Col span={8}>
          <Card title="实时数据">
            <Row>
              <Col span={12}>
                <Statistic title="在线人数" value={a.viewer_count ?? 0} />
              </Col>
              <Col span={12}>
                <Statistic title="出价数" value={a.bid_count ?? 0} />
              </Col>
            </Row>
          </Card>
        </Col>
      </Row>

      <Card title="出价流" style={{ marginTop: 16 }}>
        {snap.top_bids.length === 0 ? (
          <Typography.Text type="secondary">暂无出价</Typography.Text>
        ) : (
          <div style={{ display: 'grid' }}>
            <AnimatePresence initial={false}>
              {snap.top_bids.map((b) => (
                <motion.div
                  key={b.id}
                  initial={{ opacity: 0, x: 20 }}
                  animate={{ opacity: 1, x: 0 }}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    gap: 16,
                    padding: '12px 0',
                    borderBottom: '1px solid #f0f0f0',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', minWidth: 0, gap: 12 }}>
                    <Avatar src={b.user.avatar || undefined}>{b.user.nickname.slice(0, 1)}</Avatar>
                    <div style={{ minWidth: 0 }}>
                      <Typography.Text strong ellipsis>
                        {b.user.nickname}
                      </Typography.Text>
                      <div style={{ display: 'flex', gap: 12 }}>
                        <Typography.Text type="secondary">{centsToYuanStr(b.amount)}</Typography.Text>
                        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                          {new Date(b.created_at).toLocaleTimeString('zh-CN')}
                        </Typography.Text>
                      </div>
                    </div>
                  </div>
                  <Tag color={b.status === 'accepted' ? 'success' : 'error'}>{b.status}</Tag>
                </motion.div>
              ))}
            </AnimatePresence>
          </div>
        )}
      </Card>

      <ExtendShock show={extendShow} seconds={a.extend_seconds} onClose={() => setExtendShow(false)} />
      <WinConfetti
        show={winShow}
        winnerName={a.current_leader?.nickname ?? '—'}
        winnerAvatar={a.current_leader?.avatar}
        finalCents={a.current_price}
        onClose={() => setWinShow(false)}
      />
    </>
  )
}
