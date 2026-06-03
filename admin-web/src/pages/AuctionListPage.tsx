// 我的拍卖列表（P3）。状态 Tab + 表格 + 剩余时间每秒刷新 + 取消二次确认。
import { useCallback, useEffect, useState } from 'react'
import { Table, Tabs, Tag, Button, Space, Image, Tooltip, Typography, App } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useLocation, useNavigate } from 'react-router-dom'
import { api, ApiError } from '../lib/api-client'
import { syncServerTime } from '../lib/time'
import { centsToYuanStr } from '../lib/format'
import { Countdown } from '../components/Countdown'
import type { Auction, AuctionListData, AuctionStatus } from '../lib/types'

const STATUS_META: Record<AuctionStatus, { label: string; color: string }> = {
  pending: { label: '未开始', color: 'default' },
  active: { label: '进行中', color: 'processing' },
  ended: { label: '已结束', color: 'success' },
  cancelled: { label: '已取消', color: 'error' },
}

const TABS = [
  { key: 'all', label: '全部' },
  { key: 'pending', label: '未开始' },
  { key: 'active', label: '进行中' },
  { key: 'ended', label: '已结束' },
  { key: 'cancelled', label: '已取消' },
]

export function AuctionListPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const { message, modal } = App.useApp()
  const highlightId = (location.state as { highlightId?: number } | null)?.highlightId

  const [tab, setTab] = useState('all')
  const [data, setData] = useState<Auction[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [size] = useState(10)
  const [loading, setLoading] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const params: Record<string, string | number> = { seller_id: 'me', page, size }
      if (tab !== 'all') params.status = tab
      const res = await api.get<AuctionListData>('/api/auctions', { params })
      if (res.server_time) syncServerTime(res.server_time)
      setData(res.list)
      setTotal(res.total)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '加载失败')
    } finally {
      setLoading(false)
    }
  }, [tab, page, size, message])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect -- 标准的挂载/切 tab 拉取
    load()
  }, [load])

  const cancelAuction = (a: Auction) => {
    modal.confirm({
      title: `确认取消「${a.title}」？`,
      content: '取消后将立即向所有买家广播，且不可恢复。进行中的拍卖取消后买家将无法继续出价。',
      okText: '确认取消',
      okType: 'danger',
      cancelText: '再想想',
      onOk: async () => {
        try {
          await api.post(`/api/auctions/${a.id}/cancel`)
          message.success('已取消')
          load()
        } catch (e) {
          message.error(e instanceof ApiError ? e.message : '取消失败')
        }
      },
    })
  }

  const columns: ColumnsType<Auction> = [
    {
      title: '封面',
      dataIndex: 'cover_url',
      width: 72,
      render: (url: string | null) =>
        url ? (
          <Image src={url} width={48} height={48} style={{ objectFit: 'cover', borderRadius: 4 }} />
        ) : (
          <div style={{ width: 48, height: 48, background: '#f0f0f0', borderRadius: 4 }} />
        ),
    },
    {
      title: '标题',
      dataIndex: 'title',
      ellipsis: true,
      render: (t: string, r) => (
        <Space direction="vertical" size={0}>
          <Typography.Text strong>{t}</Typography.Text>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            #{r.id}
          </Typography.Text>
        </Space>
      ),
    },
    {
      title: '当前价',
      dataIndex: 'current_price',
      width: 120,
      render: (v: number) => <Typography.Text strong>{centsToYuanStr(v)}</Typography.Text>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (s: AuctionStatus) => <Tag color={STATUS_META[s].color}>{STATUS_META[s].label}</Tag>,
    },
    {
      title: '剩余时间',
      width: 120,
      render: (_, r) =>
        r.status === 'active' ? (
          <Countdown endTime={r.end_time} />
        ) : r.status === 'pending' ? (
          <Typography.Text type="secondary">未开始</Typography.Text>
        ) : (
          '—'
        ),
    },
    {
      title: '出价数',
      dataIndex: 'bid_count',
      width: 80,
      render: (v?: number) => v ?? 0,
    },
    {
      title: '操作',
      width: 220,
      render: (_, r) => {
        const editBtn =
          r.status === 'pending' ? (
            <Button size="small" onClick={() => navigate(`/auctions/${r.id}/edit`)}>
              修改
            </Button>
          ) : (
            <Tooltip title="进行中 / 已结束的拍卖不可修改">
              <Button size="small" disabled>
                修改
              </Button>
            </Tooltip>
          )
        return (
          <Space>
            {editBtn}
            {r.status === 'active' && (
              <Button size="small" type="primary" ghost onClick={() => navigate(`/monitor/${r.id}`)}>
                进直播间
              </Button>
            )}
            {(r.status === 'pending' || r.status === 'active') && (
              <Button size="small" danger onClick={() => cancelAuction(r)}>
                取消
              </Button>
            )}
          </Space>
        )
      },
    },
  ]

  return (
    <>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Typography.Title level={4} style={{ margin: 0 }}>
          我的拍卖
        </Typography.Title>
        <Button type="primary" onClick={() => navigate('/auctions/new')}>
          + 发布拍卖
        </Button>
      </div>
      <Tabs
        activeKey={tab}
        items={TABS}
        onChange={(k) => {
          setPage(1)
          setTab(k)
        }}
      />
      <Table<Auction>
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={data}
        rowClassName={(r) => (r.id === highlightId ? 'row-highlight' : '')}
        pagination={{
          current: page,
          pageSize: size,
          total,
          onChange: setPage,
          showTotal: (t) => `共 ${t} 条`,
        }}
      />
      <style>{`
        .row-highlight > td { background: #fffbe6 !important; transition: background 2s ease; }
      `}</style>
    </>
  )
}
