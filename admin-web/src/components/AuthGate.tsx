// 路由守卫：未登录跳 /login；已登录但非卖家则拦截（PC 后台仅限卖家）。
import { Navigate, useLocation } from 'react-router-dom'
import { Result, Spin, Button } from 'antd'
import type { ReactNode } from 'react'
import { useAuth } from '../lib/auth'

export function AuthGate({ children }: { children: ReactNode }) {
  const { user, token, role, ready, logout } = useAuth()
  const location = useLocation()

  if (!ready) {
    return (
      <div style={{ display: 'grid', placeItems: 'center', height: '100vh' }}>
        <Spin size="large" tip="加载中..." />
      </div>
    )
  }

  if (!token || !user) {
    return <Navigate to="/login" replace state={{ from: location.pathname }} />
  }

  if (role === 'buyer') {
    return (
      <Result
        status="403"
        title="仅限卖家访问"
        subTitle="PC 商家后台只对卖家开放。买家请使用移动端 H5。"
        extra={
          <Button type="primary" onClick={logout}>
            切换账号
          </Button>
        }
      />
    )
  }

  return <>{children}</>
}
