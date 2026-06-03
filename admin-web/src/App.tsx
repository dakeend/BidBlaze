import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { AuthGate } from './components/AuthGate'
import { AppLayout } from './components/AppLayout'
import { LoginPage } from './pages/LoginPage'
import { AuctionListPage } from './pages/AuctionListPage'
import { AuctionCreatePage } from './pages/AuctionCreatePage'
import { AuctionEditPage } from './pages/AuctionEditPage'
import { OrdersPage } from './pages/OrdersPage'
import { MonitorPage } from './pages/MonitorPage'
import { DemoPage } from './pages/DemoPage'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          element={
            <AuthGate>
              <AppLayout />
            </AuthGate>
          }
        >
          <Route index element={<Navigate to="/auctions" replace />} />
          <Route path="/auctions" element={<AuctionListPage />} />
          <Route path="/auctions/new" element={<AuctionCreatePage />} />
          <Route path="/auctions/:id/edit" element={<AuctionEditPage />} />
          <Route path="/orders" element={<OrdersPage />} />
          <Route path="/monitor/:id" element={<MonitorPage />} />
          <Route path="/demo" element={<DemoPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/auctions" replace />} />
      </Routes>
    </BrowserRouter>
  )
}
