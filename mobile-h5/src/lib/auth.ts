import type { UserBrief } from './types'

const tokenKey = 'auction_token'

const seedUsersByToken: Record<string, UserBrief> = {
  'mock-token-seller-001': {
    id: 1,
    nickname: '主播阿明',
    avatar: null,
  },
  'mock-token-user-001': {
    id: 2,
    nickname: '买家张三',
    avatar: null,
  },
  'mock-token-user-002': {
    id: 3,
    nickname: '买家李四',
    avatar: null,
  },
}

export function getAuthToken(): string {
  // TODO(prod): replace with JWT or safer WS auth.
  return localStorage.getItem(tokenKey) || 'mock-token-user-001'
}

export function getCurrentUserId(): number {
  return getCurrentUser().id
}

export function getCurrentUser(): UserBrief {
  const token = getAuthToken()
  const seedUser = seedUsersByToken[token]
  if (seedUser) {
    return seedUser
  }

  const id = Number(token.match(/-(\d+)$/)?.[1] ?? 0)
  return {
    id: id > 0 ? id : 2,
    nickname: token.includes('seller') ? '主播阿明' : '买家张三',
    avatar: null,
  }
}
