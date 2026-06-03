// 卖家订单列表（P4）。GET /api/orders/seller，状态过滤 + 详情。
import { useCallback, useEffect, useState } from 'react'
import { Table, Tabs, Tag, Button, Typography, App, Descriptions, Modal } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { api, ApiError } from '../lib/api-client'
import { centsToYuanStr } from '../lib/format'
import { fmt } from '../lib/time'
import type { Order, OrderStatus } from '../lib/types'

const STATUS_META: Record<OrderStatus, { label: string; color: string }> = {
  pending_pay: { label: '待支付', color: 'warning' },
  paid: { label: '已支付', color: 'success' },
  closed: { label: '已关闭', color: 'default' },
}

const TABS = [
  { key: 'all', label: '全部' },
  { key: 'pending_pay', label: '待支付' },
  { key: 'paid', label: '已支付' },
  { key: 'closed', label: '已关闭' },
]

interface OrderListData {
  list: Order[]
  total: number
  page: number
  size: number
}

export function OrdersPage() {
  const { message } = App.useApp()
  const [tab, setTab] = useState('all')
  const [data, setData] = useState<Order[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [size] = useState(10)
  const [loading, setLoading] = useState(false)
  const [detail, setDetail] = useState<Order | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const params: Record<string, string | number> = { page, size }
      if (tab !== 'all') params.status = tab
      const res = await api.get<OrderListData>('/api/orders/seller', { params })
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

  const openDetail = async (id: number) => {
    try {
      const { order } = await api.get<{ order: Order }>(`/api/orders/${id}`)
      setDetail(order)
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '加载详情失败')
    }
  }

  const columns: ColumnsType<Order> = [
    { title: '订单号', dataIndex: 'id', width: 100, render: (v: number) => `#${v}` },
    { title: '拍卖', dataIndex: 'auction_id', width: 100, render: (v: number) => `#${v}` },
    {
      title: '成交价',
      dataIndex: 'final_price',
      width: 140,
      render: (v: number) => <Typography.Text strong>{centsToYuanStr(v)}</Typography.Text>,
    },
    { title: '买家', dataIndex: ['winner', 'nickname'], width: 140 },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (s: OrderStatus) => <Tag color={STATUS_META[s].color}>{STATUS_META[s].label}</Tag>,
    },
    { title: '成交时间', dataIndex: 'created_at', render: (v: string) => fmt(v) },
    {
      title: '操作',
      width: 100,
      render: (_, r) => (
        <Button size="small" onClick={() => openDetail(r.id)}>
          详情
        </Button>
      ),
    },
  ]

  return (
    <>
      <Typography.Title level={4}>卖家订单</Typography.Title>
      <Tabs
        activeKey={tab}
        items={TABS}
        onChange={(k) => {
          setPage(1)
          setTab(k)
        }}
      />
      <Table<Order>
        rowKey="id"
        loading={loading}
        columns={columns}
        dataSource={data}
        pagination={{ current: page, pageSize: size, total, onChange: setPage, showTotal: (t) => `共 ${t} 条` }}
      />
      <Modal open={!!detail} title={`订单 #${detail?.id}`} footer={null} onCancel={() => setDetail(null)}>
        {detail && (
          <Descriptions column={1} bordered size="small">
            <Descriptions.Item label="拍卖">#{detail.auction_id}</Descriptions.Item>
            <Descriptions.Item label="成交价">{centsToYuanStr(detail.final_price)}</Descriptions.Item>
            <Descriptions.Item label="买家">{detail.winner.nickname}</Descriptions.Item>
            <Descriptions.Item label="状态">
              <Tag color={STATUS_META[detail.status].color}>{STATUS_META[detail.status].label}</Tag>
            </Descriptions.Item>
            <Descriptions.Item label="成交时间">{fmt(detail.created_at)}</Descriptions.Item>
            <Descriptions.Item label="支付时间">{detail.paid_at ? fmt(detail.paid_at) : '—'}</Descriptions.Item>
          </Descriptions>
        )}
      </Modal>
    </>
  )
}
