import { useMemo, useState } from 'react'
import { getCurrentUser } from '../lib/auth'
import { ApiCallError, placeBid } from '../lib/api-client'
import { unlockAlertAudio } from '../lib/auction-audio'
import { formatMoney } from '../lib/time'
import type { BidButtonState, BidFailureData, BidSuccessData } from '../lib/types'
import { useAuctionStore } from '../store/auctionStore'

function isBidSuccess(data: BidSuccessData | BidFailureData | null): data is BidSuccessData {
  return Boolean(data && 'bid' in data && 'current_leader' in data)
}

function failureMessage(code: number, data: BidFailureData | null): string {
  if (code === 2101) {
    return `出价低于最低有效价 ${formatMoney(data?.min_acceptable_amount)}`
  }
  if (code === 2102) {
    return `已达封顶价 ${formatMoney(data?.ceiling_price)}`
  }
  if (code === 2103) {
    return '竞争失败，已刷新当前状态'
  }
  if (code === 1004) {
    return '请求过于频繁，请稍后再试'
  }
  return '出价失败，请稍后再试'
}

export function useBidButton(auctionId: number) {
  const auction = useAuctionStore((state) => state.auction)
  const applyBidResponse = useAuctionStore((state) => state.applyBidResponse)
  const [phase, setPhase] = useState<BidButtonState>('idle')
  const [message, setMessage] = useState<string | null>(null)

  const disabledReason = useMemo(() => {
    const user = getCurrentUser()
    if (!auction) {
      return '加载拍卖中'
    }
    if (phase === 'pending') {
      return '出价提交中'
    }
    if (phase === 'cooldown') {
      return '请稍等'
    }
    if (auction.status === 'pending') {
      return '拍卖未开始'
    }
    if (auction.status === 'ended') {
      return '拍卖已结束'
    }
    if (auction.status === 'cancelled') {
      return '拍卖已取消'
    }
    if (user && auction.current_leader?.id === user.id) {
      return '你已是领先者'
    }
    return null
  }, [auction, phase])

  const buttonState: BidButtonState = disabledReason ? 'disabled' : phase

  const submitBid = async (amount: number) => {
    if (!auction || !auction.id) {
      return { ok: false, message: '拍卖数据加载中' }
    }
    if (disabledReason || phase === 'pending') {
      return { ok: false, message: disabledReason ?? '当前不可出价' }
    }
    if (amount <= 0) {
      return { ok: false, message: '出价金额必须大于 0' }
    }

    setMessage(null)
    setPhase('pending')
    void unlockAlertAudio()

    try {
      const response = await placeBid(auctionId, amount)
      if (response.code === 0 && isBidSuccess(response.data)) {
        applyBidResponse(
          response.data.bid,
          response.data.current_price,
          response.data.current_leader,
          response.data.server_time,
        )
        setPhase('cooldown')
        window.setTimeout(() => setPhase('idle'), 1000)
        setMessage('出价已提交')
        return { ok: true, message: '出价已提交' }
      }

      const data = response.data as BidFailureData | null
      const nextMessage = failureMessage(response.code, data)
      setMessage(nextMessage)
      setPhase(response.code === 1004 ? 'cooldown' : 'idle')
      if (response.code === 1004) {
        window.setTimeout(() => setPhase('idle'), 1000)
      }
      return {
        ok: false,
        message: nextMessage,
        nextAmount: data?.min_acceptable_amount,
      }
    } catch (err) {
      const nextMessage = err instanceof ApiCallError ? err.message : '网络错误，请稍后重试'
      setMessage(nextMessage)
      setPhase('idle')
      return { ok: false, message: nextMessage }
    }
  }

  return {
    buttonState,
    disabledReason,
    message,
    submitBid,
  }
}
