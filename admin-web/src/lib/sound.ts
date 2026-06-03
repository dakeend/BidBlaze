// 音效：Web Audio API，零外部依赖。iOS Safari 首次用户手势需 unlock。
// 不引入任何音频文件，全部用振荡器合成（滴答 / 成交 / 冲击波）。

let ctx: AudioContext | null = null

function getCtx(): AudioContext | null {
  if (typeof window === 'undefined') return null
  if (!ctx) {
    const Ctor = window.AudioContext || (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext
    if (!Ctor) return null
    ctx = new Ctor()
  }
  return ctx
}

/** 在首次用户手势（如点击出价）里调用，解锁 iOS 音频。 */
export function unlockAudio(): void {
  const c = getCtx()
  if (c && c.state === 'suspended') void c.resume()
}

function blip(freq: number, durationMs: number, type: OscillatorType = 'sine', gain = 0.06): void {
  const c = getCtx()
  if (!c || c.state === 'suspended') return
  const osc = c.createOscillator()
  const g = c.createGain()
  osc.type = type
  osc.frequency.value = freq
  g.gain.setValueAtTime(gain, c.currentTime)
  g.gain.exponentialRampToValueAtTime(0.0001, c.currentTime + durationMs / 1000)
  osc.connect(g).connect(c.destination)
  osc.start()
  osc.stop(c.currentTime + durationMs / 1000)
}

/** 倒计时滴答。 */
export function playTick(): void {
  blip(880, 60, 'square', 0.04)
}

/** 成交 / 出价成功提示音（上扬两音）。 */
export function playDing(): void {
  blip(660, 120, 'sine', 0.06)
  setTimeout(() => blip(990, 180, 'sine', 0.06), 110)
}

/** 延时冲击波（下滑噪声感）。 */
export function playWhoosh(): void {
  const c = getCtx()
  if (!c || c.state === 'suspended') return
  const osc = c.createOscillator()
  const g = c.createGain()
  osc.type = 'sawtooth'
  osc.frequency.setValueAtTime(440, c.currentTime)
  osc.frequency.exponentialRampToValueAtTime(120, c.currentTime + 0.4)
  g.gain.setValueAtTime(0.05, c.currentTime)
  g.gain.exponentialRampToValueAtTime(0.0001, c.currentTime + 0.4)
  osc.connect(g).connect(c.destination)
  osc.start()
  osc.stop(c.currentTime + 0.4)
}
