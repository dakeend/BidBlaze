// MSW handlers —— mock-first 开发用。fixture 取自仓库根 fixtures/，与 mobile-h5 共用。
// 切真接口时机：按 integration-protocol §3 表，逐个移除对应 handler。
import { http, HttpResponse } from 'msw'
import usersFixture from '../../../fixtures/users.json'
import auctionsFixture from '../../../fixtures/auctions.json'
import type { Auction, Bid, Order, UserBrief } from '../lib/types'

const API = (path: string) => `${import.meta.env.VITE_API_BASE || 'http://localhost:8080'}${path}`

const ok = <T>(data: T) => HttpResponse.json({ code: 0, msg: 'ok', data })
const fail = (code: number, msg: string, httpStatus = 200, data: unknown = null) =>
  HttpResponse.json({ code, msg, data }, { status: httpStatus })

const nowIso = () => new Date().toISOString()
const shift = (sec: number) => new Date(Date.now() + sec * 1000).toISOString()

// --- 用户存储 ---
interface StoredUser extends UserBrief {
  token: string
  role: 'seller' | 'buyer'
}
const users: StoredUser[] = usersFixture.seed.map((u) => ({
  ...u,
  role: u.role as 'seller' | 'buyer',
}))
let nextUserId = users.length + 1

function userByToken(req: Request): StoredUser | null {
  const auth = req.headers.get('Authorization') || ''
  const token = auth.replace(/^Bearer\s+/i, '')
  return users.find((u) => u.token === token) || null
}
const brief = (u: StoredUser | UserBrief): UserBrief => ({ id: u.id, nickname: u.nickname, avatar: u.avatar })

// --- 拍卖存储（时间相对当前重算，便于演示）---
const auctions: Auction[] = (auctionsFixture.list as Auction[]).map((a) => {
  const copy = { ...a }
  if (a.status === 'active') {
    copy.start_time = shift(-120)
    copy.end_time = shift(300)
    copy.original_end_time = copy.end_time
  } else if (a.status === 'pending') {
    copy.start_time = shift(1800)
    copy.end_time = shift(1800 + 1800)
    copy.original_end_time = copy.end_time
  }
  return copy
})
let nextAuctionId = 200

// 每个拍卖的出价流（用于监控页 / bids 接口）。
const bidsByAuction = new Map<number, Bid[]>()
let nextBidId = 1000
function seedBids(a: Auction) {
  if (bidsByAuction.has(a.id)) return
  const list: Bid[] = []
  if (a.bid_count && a.current_leader) {
    list.push({
      id: nextBidId++,
      auction_id: a.id,
      user: a.current_leader,
      amount: a.current_price,
      status: 'accepted',
      reject_reason: null,
      idempotency_key: null,
      created_at: nowIso(),
    })
  }
  bidsByAuction.set(a.id, list)
}
auctions.forEach(seedBids)

// --- 订单存储（从 ended 拍卖合成）---
const orders: Order[] = auctions
  .filter((a) => a.status === 'ended' && a.current_leader)
  .map((a, i) => ({
    id: 5000 + i,
    auction_id: a.id,
    winner: a.current_leader as UserBrief,
    seller: a.seller,
    final_price: a.current_price,
    status: 'pending_pay',
    created_at: a.updated_at,
    paid_at: null,
  }))

// --- 出价模拟器：让 active 拍卖价格随时间上涨，监控页/倒计时有动效 ---
const buyers = users.filter((u) => u.role === 'buyer')
if (typeof window !== 'undefined' && import.meta.env.VITE_USE_MSW !== 'false') {
  setInterval(() => {
    const t = Date.now()
    for (const a of auctions) {
      if (a.status !== 'active') continue
      const end = new Date(a.end_time).getTime()
      if (t >= end) {
        a.status = 'ended'
        if (a.current_leader) {
          orders.push({
            id: 5000 + orders.length,
            auction_id: a.id,
            winner: a.current_leader,
            seller: a.seller,
            final_price: a.current_price,
            status: 'pending_pay',
            created_at: nowIso(),
            paid_at: null,
          })
        }
        continue
      }
      // 30% 概率出一手价
      if (Math.random() < 0.3 && buyers.length) {
        const buyer = buyers[Math.floor(Math.random() * buyers.length)]
        a.current_price += a.price_step
        a.version += 1
        a.bid_count = (a.bid_count || 0) + 1
        a.current_leader = brief(buyer)
        a.updated_at = nowIso()
        const list = bidsByAuction.get(a.id) || []
        list.unshift({
          id: nextBidId++,
          auction_id: a.id,
          user: brief(buyer),
          amount: a.current_price,
          status: 'accepted',
          reject_reason: null,
          idempotency_key: null,
          created_at: nowIso(),
        })
        bidsByAuction.set(a.id, list.slice(0, 50))
        // 临近结束触发延时
        if (end - t <= a.extend_threshold * 1000) {
          a.end_time = shift(a.extend_seconds)
        }
      }
      a.viewer_count = Math.max(0, (a.viewer_count || 0) + Math.floor(Math.random() * 7) - 3)
    }
  }, 3000)
}

export const handlers = [
  // 登录：seller 前缀 → 卖家 token；同昵称返回旧 token。
  http.post(API('/api/login'), async ({ request }) => {
    const body = (await request.json()) as { nickname?: string; avatar?: string | null }
    const nickname = (body.nickname || '').trim()
    if (!nickname) return fail(1001, '昵称不能为空', 400)
    let u = users.find((x) => x.nickname === nickname)
    if (!u) {
      const isSeller = /^(主播|商家|卖家)/.test(nickname)
      const id = nextUserId++
      const kind = isSeller ? 'seller' : 'user'
      u = {
        id,
        nickname,
        avatar: body.avatar ?? null,
        token: `mock-token-${kind}-${String(id).padStart(3, '0')}`,
        role: isSeller ? 'seller' : 'buyer',
      }
      users.push(u)
    }
    return ok({ token: u.token, user: brief(u) })
  }),

  http.get(API('/api/users/me'), ({ request }) => {
    const u = userByToken(request)
    if (!u) return fail(1002, '未登录', 401)
    return ok({ user: brief(u) })
  }),

  // 列表
  http.get(API('/api/auctions'), ({ request }) => {
    const url = new URL(request.url)
    const status = url.searchParams.get('status')
    const sellerParam = url.searchParams.get('seller_id')
    const page = Number(url.searchParams.get('page') || 1)
    const size = Number(url.searchParams.get('size') || 20)

    let list = [...auctions]
    if (sellerParam) {
      let sellerId: number | null = null
      if (sellerParam === 'me') sellerId = userByToken(request)?.id ?? null
      else sellerId = Number(sellerParam)
      list = list.filter((a) => a.seller.id === sellerId)
    }
    if (status) list = list.filter((a) => a.status === status)
    list.sort((a, b) => b.id - a.id)
    const total = list.length
    const paged = list.slice((page - 1) * size, (page - 1) * size + size)
    return ok({ list: paged, total, page, size, server_time: nowIso() })
  }),

  // 详情
  http.get(API('/api/auctions/:id'), ({ params }) => {
    const a = auctions.find((x) => x.id === Number(params.id))
    if (!a) return fail(2001, '拍卖不存在', 404)
    return ok({ auction: a })
  }),

  // 创建
  http.post(API('/api/auctions'), async ({ request }) => {
    const u = userByToken(request)
    if (!u) return fail(1002, '未登录', 401)
    const body = (await request.json()) as Partial<Auction> & {
      start_time: string
      duration_seconds: number
    }
    if (!body.title || body.start_price == null || !body.price_step) {
      return fail(1001, '参数错误', 400)
    }
    const start = new Date(body.start_time).getTime()
    const end = new Date(start + (body.duration_seconds || 600) * 1000).toISOString()
    const a: Auction = {
      id: nextAuctionId++,
      title: body.title!,
      description: body.description ?? null,
      cover_url: body.cover_url ?? null,
      images: body.images ?? [],
      stream_url: body.stream_url ?? null,
      start_price: body.start_price!,
      price_step: body.price_step!,
      ceiling_price: body.ceiling_price ?? null,
      current_price: body.start_price!,
      current_leader: null,
      start_time: body.start_time,
      end_time: end,
      original_end_time: end,
      extend_seconds: body.extend_seconds ?? 30,
      extend_threshold: body.extend_threshold ?? 30,
      status: 'pending',
      version: 1,
      viewer_count: 0,
      bid_count: 0,
      seller: brief(u),
      created_at: nowIso(),
      updated_at: nowIso(),
    }
    auctions.push(a)
    seedBids(a)
    return ok({ auction: a })
  }),

  // 修改（仅 pending）
  http.put(API('/api/auctions/:id'), async ({ params, request }) => {
    const u = userByToken(request)
    if (!u) return fail(1002, '未登录', 401)
    const a = auctions.find((x) => x.id === Number(params.id))
    if (!a) return fail(2001, '拍卖不存在', 404)
    if (a.seller.id !== u.id) return fail(1003, '无权限', 403)
    if (a.status !== 'pending') return fail(1003, '仅未开始的拍卖可修改', 403)
    const body = (await request.json()) as Partial<Auction> & { duration_seconds?: number }
    Object.assign(a, {
      title: body.title ?? a.title,
      description: body.description ?? a.description,
      cover_url: body.cover_url ?? a.cover_url,
      images: body.images ?? a.images,
      stream_url: body.stream_url ?? a.stream_url,
      start_price: body.start_price ?? a.start_price,
      price_step: body.price_step ?? a.price_step,
      ceiling_price: body.ceiling_price ?? a.ceiling_price,
      extend_seconds: body.extend_seconds ?? a.extend_seconds,
      extend_threshold: body.extend_threshold ?? a.extend_threshold,
      version: a.version + 1,
      updated_at: nowIso(),
    })
    if (body.start_time) {
      a.start_time = body.start_time
      const end = new Date(new Date(body.start_time).getTime() + (body.duration_seconds || 600) * 1000).toISOString()
      a.end_time = end
      a.original_end_time = end
    }
    if (a.current_leader == null) a.current_price = a.start_price
    return ok({ auction: a })
  }),

  // 取消
  http.post(API('/api/auctions/:id/cancel'), ({ params, request }) => {
    const u = userByToken(request)
    if (!u) return fail(1002, '未登录', 401)
    const a = auctions.find((x) => x.id === Number(params.id))
    if (!a) return fail(2001, '拍卖不存在', 404)
    if (a.seller.id !== u.id) return fail(1003, '无权限', 403)
    if (a.status !== 'pending' && a.status !== 'active') return fail(1003, '当前状态不可取消', 403)
    a.status = 'cancelled'
    a.version += 1
    a.updated_at = nowIso()
    return ok({ auction: a })
  }),

  // 快照（监控页轮询）
  http.get(API('/api/auctions/:id/status'), ({ params }) => {
    const a = auctions.find((x) => x.id === Number(params.id))
    if (!a) return fail(2001, '拍卖不存在', 404)
    const top = (bidsByAuction.get(a.id) || []).slice(0, 10)
    return ok({ auction: a, top_bids: top, last_event_seq: a.version, server_time: nowIso() })
  }),

  // 出价列表
  http.get(API('/api/auctions/:id/bids'), ({ params, request }) => {
    const a = auctions.find((x) => x.id === Number(params.id))
    if (!a) return fail(2001, '拍卖不存在', 404)
    const limit = Number(new URL(request.url).searchParams.get('limit') || 20)
    return ok({ list: (bidsByAuction.get(a.id) || []).slice(0, limit) })
  }),

  // 上传：返回固定 fixture url
  http.post(API('/api/uploads'), ({ request }) => {
    const u = userByToken(request)
    if (!u) return fail(1002, '未登录', 401)
    const seed = Math.floor(Math.random() * 1000)
    return ok({ url: `https://picsum.photos/seed/up${seed}/800/800`, width: 800, height: 800, size: 120000 })
  }),

  // 卖家订单
  http.get(API('/api/orders/seller'), ({ request }) => {
    const u = userByToken(request)
    if (!u) return fail(1002, '未登录', 401)
    const url = new URL(request.url)
    const status = url.searchParams.get('status')
    const page = Number(url.searchParams.get('page') || 1)
    const size = Number(url.searchParams.get('size') || 20)
    let list = orders.filter((o) => o.seller.id === u.id)
    if (status) list = list.filter((o) => o.status === status)
    const total = list.length
    return ok({ list: list.slice((page - 1) * size, (page - 1) * size + size), total, page, size })
  }),

  http.get(API('/api/orders/mine'), ({ request }) => {
    const u = userByToken(request)
    if (!u) return fail(1002, '未登录', 401)
    const list = orders.filter((o) => o.winner.id === u.id)
    return ok({ list, total: list.length, page: 1, size: 20 })
  }),

  http.get(API('/api/orders/:id'), ({ params, request }) => {
    const u = userByToken(request)
    if (!u) return fail(1002, '未登录', 401)
    const o = orders.find((x) => x.id === Number(params.id))
    if (!o) return fail(3001, '订单不存在', 404)
    if (o.seller.id !== u.id && o.winner.id !== u.id) return fail(1003, '无权限', 403)
    return ok({ order: o })
  }),

  http.post(API('/api/orders/:id/pay'), ({ params, request }) => {
    const u = userByToken(request)
    if (!u) return fail(1002, '未登录', 401)
    const o = orders.find((x) => x.id === Number(params.id))
    if (!o) return fail(3001, '订单不存在', 404)
    if (o.winner.id !== u.id) return fail(1003, '无权限', 403)
    if (o.status === 'pending_pay') {
      o.status = 'paid'
      o.paid_at = nowIso()
    }
    return ok({ order: o })
  }),
]
