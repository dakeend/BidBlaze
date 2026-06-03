// 发布拍卖页（P2）。提交成功后跳「我的拍卖」并高亮新行。
import { useState } from 'react'
import { Typography, App } from 'antd'
import { useNavigate } from 'react-router-dom'
import { AuctionForm } from '../components/AuctionForm'
import { api, ApiError } from '../lib/api-client'
import type { Auction, CreateAuctionRequest } from '../lib/types'

export function AuctionCreatePage() {
  const navigate = useNavigate()
  const { message } = App.useApp()
  const [submitting, setSubmitting] = useState(false)

  const onSubmit = async (payload: CreateAuctionRequest) => {
    setSubmitting(true)
    try {
      const { auction } = await api.post<{ auction: Auction }>('/api/auctions', payload)
      message.success('发布成功')
      navigate('/auctions', { state: { highlightId: auction.id } })
    } catch (e) {
      message.error(e instanceof ApiError ? e.message : '发布失败，请重试')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <>
      <Typography.Title level={4}>发布拍卖</Typography.Title>
      <AuctionForm mode="create" submitting={submitting} onSubmit={onSubmit} />
    </>
  )
}
