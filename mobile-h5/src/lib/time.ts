export function toServerMs(serverTime: string | undefined): number {
  if (!serverTime) {
    return Date.now()
  }
  const parsed = new Date(serverTime).getTime()
  return Number.isNaN(parsed) ? Date.now() : parsed
}

export function createServerOffset(serverTime: string): number {
  return toServerMs(serverTime) - Date.now()
}

export function formatRemaining(ms: number): string {
  const safeMs = Math.max(0, ms)
  const totalSeconds = Math.floor(safeMs / 1000)
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return `${String(minutes).padStart(2, '0')}:${String(seconds).padStart(2, '0')}`
}

export function formatMoney(amount: number | null | undefined): string {
  const value = Math.max(0, amount ?? 0) / 100
  return new Intl.NumberFormat('zh-CN', {
    style: 'currency',
    currency: 'CNY',
    maximumFractionDigits: 0,
  }).format(value)
}

export function nowIso(): string {
  return new Date().toISOString()
}
