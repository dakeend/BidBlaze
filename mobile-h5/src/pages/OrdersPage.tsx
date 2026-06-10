import { useCallback, useEffect, useState } from 'react'
import { ArrowLeft, CreditCard, Package } from 'lucide-react'
import { apiClient, payOrder } from '../lib/api-client'
import { formatMoney } from '../lib/time'
import type { OrderListItem } from '../lib/types'

type OrdersPageProps = {
  onBack: () => void
}

export function OrdersPage({ onBack }: OrdersPageProps) {
  const [orders, setOrders] = useState<OrderListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [payingId, setPayingId] = useState<number | null>(null)

  const fetchOrders = useCallback(async () => {
    setLoading(true)
    try {
      const resp = await apiClient.get<{
        code: number
        msg: string
        data: { list: OrderListItem[] }
      }>('/api/orders/mine')
      if (resp.data.code === 0) {
        setOrders(resp.data.data.list)
      }
    } catch {
      // ignore
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    fetchOrders()
  }, [fetchOrders])

  const handlePay = async (orderId: number) => {
    setPayingId(orderId)
    try {
      await payOrder(orderId)
      setOrders((prev) =>
        prev.map((o) => (o.id === orderId ? { ...o, status: 'paid' } : o)),
      )
    } catch {
      // 失败后恢复按钮
    } finally {
      setPayingId(null)
    }
  }

  const statusLabel: Record<string, string> = {
    pending_pay: '待支付',
    paid: '已支付',
    closed: '已关闭',
  }

  return (
    <div className="orders-page">
      <header className="orders-header">
        <button className="icon-button" type="button" aria-label="返回" onClick={onBack}>
          <ArrowLeft size={20} />
        </button>
        <h2>我的订单</h2>
        <div style={{ width: 40 }} />
      </header>

      {loading ? (
        <p className="empty-hint">加载中...</p>
      ) : orders.length === 0 ? (
        <div className="orders-empty">
          <Package size={40} />
          <p>暂无订单</p>
        </div>
      ) : (
        <div className="order-list">
          {orders.map((o) => (
            <div className="order-card" key={o.id}>
              <div className="order-top">
                <span className="order-status" data-status={o.status}>
                  {statusLabel[o.status] || o.status}
                </span>
                <span className="order-price">{formatMoney(o.final_price)}</span>
              </div>
              <div className="order-meta">
                <span>卖主：{o.seller.nickname}</span>
                <span>#{o.id}</span>
              </div>
              {o.status === 'pending_pay' && (
                <button
                  className="pay-button"
                  onClick={() => handlePay(o.id)}
                  disabled={payingId === o.id}
                >
                  <CreditCard size={14} />
                  <span>{payingId === o.id ? '支付中...' : '立即支付'}</span>
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
