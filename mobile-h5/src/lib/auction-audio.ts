type ToneType = 'tick' | 'notice' | 'outbid' | 'ended'

let audioContext: AudioContext | null = null
let audioUnlocked = false

function getAudioContext() {
  if (typeof window === 'undefined') {
    return null
  }

  const AudioContextCtor = window.AudioContext || window.webkitAudioContext
  if (!AudioContextCtor) {
    return null
  }

  if (!audioContext) {
    audioContext = new AudioContextCtor()
  }
  return audioContext
}

function playOscillator(context: AudioContext, frequency: number, startAt: number, duration: number, gain = 0.05) {
  const oscillator = context.createOscillator()
  const volume = context.createGain()
  oscillator.type = 'sine'
  oscillator.frequency.setValueAtTime(frequency, startAt)
  volume.gain.setValueAtTime(0.0001, startAt)
  volume.gain.exponentialRampToValueAtTime(gain, startAt + 0.015)
  volume.gain.exponentialRampToValueAtTime(0.0001, startAt + duration)
  oscillator.connect(volume)
  volume.connect(context.destination)
  oscillator.start(startAt)
  oscillator.stop(startAt + duration + 0.02)
}

export async function unlockAlertAudio() {
  const context = getAudioContext()
  if (!context) {
    return false
  }

  try {
    if (context.state === 'suspended') {
      await context.resume()
    }
    const now = context.currentTime
    playOscillator(context, 1, now, 0.01, 0.0001)
    audioUnlocked = true
    return true
  } catch {
    audioUnlocked = false
    return false
  }
}

export function playAlertTone(type: ToneType) {
  const context = getAudioContext()
  if (!context || !audioUnlocked || context.state !== 'running') {
    return
  }

  const now = context.currentTime
  if (type === 'tick') {
    playOscillator(context, 760, now, 0.055, 0.035)
    return
  }

  if (type === 'outbid') {
    playOscillator(context, 520, now, 0.09, 0.05)
    playOscillator(context, 390, now + 0.11, 0.12, 0.045)
    return
  }

  if (type === 'ended') {
    playOscillator(context, 440, now, 0.11, 0.045)
    playOscillator(context, 660, now + 0.13, 0.16, 0.045)
    return
  }

  playOscillator(context, 660, now, 0.08, 0.045)
  playOscillator(context, 880, now + 0.09, 0.1, 0.04)
}

declare global {
  interface Window {
    webkitAudioContext?: typeof AudioContext
  }
}
