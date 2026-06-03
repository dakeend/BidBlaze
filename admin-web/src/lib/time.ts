// 统一时间工具：dayjs + Asia/Shanghai + server_time 偏移。
// 倒计时严禁直接用 Date.now()（漂移）；用 serverNow() 校准后的时间。
import dayjs from 'dayjs'
import utc from 'dayjs/plugin/utc'
import timezone from 'dayjs/plugin/timezone'
import duration from 'dayjs/plugin/duration'
import 'dayjs/locale/zh-cn'

dayjs.extend(utc)
dayjs.extend(timezone)
dayjs.extend(duration)
dayjs.locale('zh-cn')
dayjs.tz.setDefault('Asia/Shanghai')

export { dayjs }

// 本地时钟 与 服务端时钟 的偏移：serverNow = Date.now() + offsetMs。
let offsetMs = 0

/** 用任意接口返回的 server_time 校准本地偏移。 */
export function syncServerTime(serverTimeIso: string): void {
  const server = dayjs(serverTimeIso).valueOf()
  if (!Number.isFinite(server)) return
  offsetMs = server - Date.now()
}

/** 当前服务端时间（毫秒）。 */
export function serverNow(): number {
  return Date.now() + offsetMs
}

export function getOffsetMs(): number {
  return offsetMs
}

/** 格式化为本地展示字符串。 */
export function fmt(iso: string | null | undefined, pattern = 'YYYY-MM-DD HH:mm:ss'): string {
  if (!iso) return '-'
  return dayjs(iso).tz('Asia/Shanghai').format(pattern)
}

/** 剩余毫秒（基于服务端校准时钟）；过期或无效返回 0。 */
export function remainingMs(endTimeIso: string | null | undefined): number {
  if (!endTimeIso) return 0
  const end = dayjs(endTimeIso).valueOf()
  if (!Number.isFinite(end)) return 0
  return Math.max(0, end - serverNow())
}

/** 把剩余毫秒格式化为 HH:mm:ss / mm:ss。 */
export function fmtRemaining(ms: number): string {
  if (ms <= 0) return '00:00'
  const total = Math.floor(ms / 1000)
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  const pad = (n: number) => String(n).padStart(2, '0')
  return h > 0 ? `${pad(h)}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`
}
