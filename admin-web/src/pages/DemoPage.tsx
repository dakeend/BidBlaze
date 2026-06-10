// 氛围动画 Demo（场景 4）。方便录视频：逐个触发六种动画。
import { useState } from 'react'
import { Card, Button, Space, Row, Col, Typography, Divider } from 'antd'
import { unlockAudio } from '../lib/sound'
import { BidFlip } from '../components/atmosphere/BidFlip'
import { LeaderBadge } from '../components/atmosphere/LeaderBadge'
import { OvertakenFlash } from '../components/atmosphere/OvertakenFlash'
import { CountdownPulse } from '../components/atmosphere/CountdownPulse'
import { ExtendShock } from '../components/atmosphere/ExtendShock'
import { WinConfetti } from '../components/atmosphere/WinConfetti'

export function DemoPage() {
  const [price, setPrice] = useState(880000)
  const [leading, setLeading] = useState(true)
  const [overtaken, setOvertaken] = useState(false)
  const [extend, setExtend] = useState(false)
  const [win, setWin] = useState(false)
  const [countdownEnd, setCountdownEnd] = useState(() => new Date(Date.now() + 9000).toISOString())

  return (
    <div onPointerDown={unlockAudio}>
      <Typography.Title level={4}>氛围动画 Demo</Typography.Title>
      <Typography.Paragraph type="secondary">
        点击任意按钮触发动画（首次点击会解锁音频）。用于录制演示视频。
      </Typography.Paragraph>

      <Row gutter={16}>
        <Col span={8}>
          <Card title="1 · 出价翻牌 + 领先徽章" style={{ height: '100%' }}>
            <BidFlip cents={price} />
            <div style={{ marginTop: 12 }}>{leading && <LeaderBadge />}</div>
            <Divider />
            <Space>
              <Button onClick={() => setPrice((p) => p + 50000)}>+¥500 翻牌</Button>
              <Button onClick={() => setLeading((v) => !v)}>切换领先</Button>
            </Space>
          </Card>
        </Col>

        <Col span={8}>
          <Card title="4 · 倒计时心跳 + 滴答" style={{ height: '100%' }}>
            <CountdownPulse endTime={countdownEnd} />
            <Divider />
            <Button onClick={() => setCountdownEnd(new Date(Date.now() + 9000).toISOString())}>
              重置到 9 秒
            </Button>
          </Card>
        </Col>

        <Col span={8}>
          <Card title="触发型动画" style={{ height: '100%' }}>
            <Space orientation="vertical" style={{ width: '100%' }}>
              <Button danger block onClick={() => setOvertaken(true)}>
                3 · 被超越闪红 + 震动
              </Button>
              <Button block onClick={() => setExtend(true)}>
                5 · 延时冲击波
              </Button>
              <Button type="primary" block onClick={() => setWin(true)}>
                6 · 成交撒花
              </Button>
            </Space>
          </Card>
        </Col>
      </Row>

      <OvertakenFlash
        show={overtaken}
        diffCents={50000}
        onAct={() => setOvertaken(false)}
        onClose={() => setOvertaken(false)}
      />
      <ExtendShock show={extend} seconds={30} onClose={() => setExtend(false)} />
      <WinConfetti
        show={win}
        winnerName="买家张三"
        finalCents={price}
        onClose={() => setWin(false)}
      />
    </div>
  )
}
