// 展示格式化工具。合同金额单位为「分」，展示统一转「元」。
export function centsToYuanStr(cents: number | null | undefined): string {
  if (cents == null) return '—'
  return `¥${(cents / 100).toLocaleString('zh-CN', { minimumFractionDigits: 2 })}`
}
