import { apiClient } from './api-client'
import type { UserBrief } from './types'

const tokenKey = 'auction_token'

export function getAuthToken(): string {
  return localStorage.getItem(tokenKey) || ''
}

export function setAuthToken(token: string): void {
  localStorage.setItem(tokenKey, token)
}

export function clearAuthToken(): void {
  localStorage.removeItem(tokenKey)
}

export function isLoggedIn(): boolean {
  return !!getAuthToken()
}

let cachedUser: UserBrief | null = null

export function getCurrentUser(): UserBrief | null {
  return cachedUser
}

export async function login(nickname: string): Promise<{ token: string; user: UserBrief }> {
  const response = await apiClient.post<{
    code: number
    msg: string
    data: { token: string; user: UserBrief }
  }>('/api/login', { nickname })
  const data = response.data.data
  setAuthToken(data.token)
  cachedUser = data.user
  return data
}

export async function fetchMe(): Promise<UserBrief | null> {
  const token = getAuthToken()
  if (!token) return null
  try {
    const response = await apiClient.get<{
      code: number
      msg: string
      data: { user: UserBrief }
    }>('/api/users/me')
    cachedUser = response.data.data.user
    return cachedUser
  } catch {
    clearAuthToken()
    cachedUser = null
    return null
  }
}

export function logout(): void {
  clearAuthToken()
  cachedUser = null
}
