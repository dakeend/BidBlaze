/* eslint-disable react-refresh/only-export-components -- 鉴权模块同时导出 Provider 组件、useAuth hook 与 roleFromToken 工具，属约定。 */
// 鉴权上下文：token 存取、当前用户、登录/登出、会话恢复。
// 角色仅用于前端体验分流（PC 后台只放卖家），不写入业务规则。见 dev-setup §5.2。
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import { api, clearToken, getToken, setToken, setUnauthorizedHandler } from './api-client'
import type { LoginData, UserBrief } from './types'

export type Role = 'seller' | 'buyer'

/**
 * 从 mock token 段推断角色（dev-setup §5.1：mock-token-<seller|user>-NNN）。
 * 仅供前端体验分流；鉴权以后端 DB 为唯一事实来源。
 */
export function roleFromToken(token: string | null): Role | null {
  if (!token) return null
  if (token.includes('-seller-')) return 'seller'
  if (token.includes('-user-')) return 'buyer'
  return null
}

interface AuthContextValue {
  user: UserBrief | null
  token: string | null
  role: Role | null
  ready: boolean // 会话恢复完成
  login: (nickname: string) => Promise<LoginData>
  logout: () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<UserBrief | null>(null)
  const [token, setTokenState] = useState<string | null>(() => getToken())
  const [ready, setReady] = useState(false)

  const logout = useCallback(() => {
    clearToken()
    setTokenState(null)
    setUser(null)
  }, [])

  // 把 401 全局回调接到登出。
  useEffect(() => {
    setUnauthorizedHandler(() => {
      logout()
      if (location.pathname !== '/login') location.assign('/login')
    })
  }, [logout])

  // 会话恢复：有 token 则拉 /api/users/me。
  useEffect(() => {
    let alive = true
    const t = getToken()
    if (!t) {
      // eslint-disable-next-line react-hooks/set-state-in-effect -- 无 token 时同步结束会话恢复
      setReady(true)
      return
    }
    api
      .get<{ user: UserBrief }>('/api/users/me')
      .then((data) => {
        if (alive) setUser(data.user)
      })
      .catch(() => {
        // 401 已由拦截器登出；其它错误忽略。
      })
      .finally(() => {
        if (alive) setReady(true)
      })
    return () => {
      alive = false
    }
  }, [])

  const login = useCallback(async (nickname: string) => {
    const data = await api.post<LoginData>('/api/login', { nickname })
    setToken(data.token)
    setTokenState(data.token)
    setUser(data.user)
    return data
  }, [])

  const value = useMemo<AuthContextValue>(
    () => ({ user, token, role: roleFromToken(token), ready, login, logout }),
    [user, token, ready, login, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used within <AuthProvider>')
  return ctx
}
