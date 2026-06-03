import { AuctionRoomPage } from './pages/AuctionRoomPage'
import './App.css'

function parseAuctionId(pathname: string): number {
  const match = pathname.match(/^\/auctions\/(\d+)/)
  if (!match) {
    return 1
  }
  return Number(match[1])
}

function App() {
  const auctionId = parseAuctionId(window.location.pathname)

  return <AuctionRoomPage auctionId={auctionId} />
}

export default App
