import { useCallback, useEffect, useState } from 'react'
import { AuctionRoomPage } from './pages/AuctionRoomPage'
import { HomePage } from './pages/HomePage'
import { LoginPage } from './pages/LoginPage'
import { OrdersPage } from './pages/OrdersPage'
import { fetchMe, isLoggedIn } from './lib/auth'
import './App.css'

function parseAuctionId(pathname: string): number | null {
  const match = pathname.match(/^\/auctions\/(\d+)/)
  if (!match) {
    return null
  }
  return Number(match[1])
}

/** 所有页面统一包裹在居中手机框内，模拟移动端展示 */
function PhoneFrame({ children }: { children: React.ReactNode }) {
  return (
    <main className="auction-shell">
      <section className="auction-phone">{children}</section>
    </main>
  )
}

function App() {
  const [ready, setReady] = useState(false)
  const [loggedIn, setLoggedIn] = useState(false)
  const [page, setPage] = useState<'home' | 'room' | 'orders'>('home')
  const [auctionId, setAuctionId] = useState<number>(1)

  useEffect(() => {
    if (isLoggedIn()) {
      fetchMe().then((user) => {
        if (user) {
          setLoggedIn(true)
        }
        setReady(true)
      })
    } else {
      setReady(true)
    }
  }, [])

  useEffect(() => {
    const id = parseAuctionId(window.location.pathname)
    if (id) {
      setAuctionId(id)
      setPage('room')
    }
  }, [])

  const handleLogin = useCallback(() => {
    setLoggedIn(true)
  }, [])

  const handleEnterRoom = useCallback((id: number) => {
    setAuctionId(id)
    setPage('room')
    window.history.pushState({}, '', `/auctions/${id}`)
  }, [])

  const handleBack = useCallback(() => {
    setPage('home')
    window.history.pushState({}, '', '/')
  }, [])

  if (!ready) {
    return (
      <PhoneFrame>
        <div style={{ display: 'grid', placeItems: 'center', height: '100vh' }}>
          <p>加载中...</p>
        </div>
      </PhoneFrame>
    )
  }

  if (!loggedIn) {
    return (
      <PhoneFrame>
        <LoginPage onLogin={handleLogin} />
      </PhoneFrame>
    )
  }

  if (page === 'room') {
    return (
      <PhoneFrame>
        <AuctionRoomPage auctionId={auctionId} onBack={handleBack} />
      </PhoneFrame>
    )
  }

  if (page === 'orders') {
    return (
      <PhoneFrame>
        <OrdersPage onBack={handleBack} />
      </PhoneFrame>
    )
  }

  return (
    <PhoneFrame>
      <HomePage onEnter={handleEnterRoom} onViewOrders={() => setPage('orders')} />
    </PhoneFrame>
  )
}

export default App
