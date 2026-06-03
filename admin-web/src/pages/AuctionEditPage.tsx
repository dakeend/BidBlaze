// 编辑拍卖页（P2，仅 pending 可改）。PUT /api/auctions/:id。
import { useEffect, useState } from 'react'
import { Typography, App, Spin, Alert, Result, Button } from 'antd'
import { useNavigate, useParams } from 'react-router-dom'
import { AuctionForm } from '../components/AuctionForm'
import { api, ApiError } from '../lib/api-client'
import type { Auction, CreateAuctionRequest } from '../lib/types'

export function AuctionEditPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { message } = App.useApp()
  const [auction, setAuction] = useState<Auction | null>(null)
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let alive = true
    // eslint-disable-next-line react-hooks/set-state-in-effect -- 标准的挂载即拉取，loading 同步置位是预期行为
    setLoading(true)
    api
      .get<{ auction: Auction }>(`/api/auctions/${id}`)
      .then((d) => alive && setAuction(d.auction))
      .catch((e) => alive && setError(e instanceof ApiError ? e.message : '加载失败'))
      .finally(() => alive && setLoading(false))
    return () => {
      alive = false
    }
  }, [id])

  const onSubmit = async (payload: CreateAuctionRequest) => {
    setSubmitting(true)
    try {
      await api.put<{ auction: Auction }>(`/api/auctions/${id}`, payload)
      message.success('保存成功')
      navigate('/auctions', { state: { highlightId: Number(id) } })
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '保存失败，请重试')
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) return <Spin size="large" style={{ display: 'block', marginTop: 80 }} />
  if (error || !auction)
    return (
      <Result
        status="error"
        title="加载失败"
        subTitle={error ?? '拍卖不存在'}
        extra={<Button onClick={() => navigate('/auctions')}>返回列表</Button>}
      />
    )

  return (
    <>
      <Typography.Title level={4}>编辑拍卖 #{auction.id}</Typography.Title>
      {auction.status !== 'pending' && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message="该拍卖已不是「未开始」状态，保存会被服务端拒绝。仅未开始的拍卖可修改。"
        />
      )}
      <AuctionForm mode="edit" initial={auction} submitting={submitting} onSubmit={onSubmit} />
    </>
  )
}
