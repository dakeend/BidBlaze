import type { ConnectionState } from './types'

type ConnectionManagerOptions = {
  buildUrl: () => string
  refreshStatus: () => Promise<void>
  onMessage: (payload: unknown) => void
  onStateChange: (state: ConnectionState) => void
  onError: (message: string | null) => void
  createSocket?: (url: string) => WebSocket
  reconnectBackoffMs?: number[]
  pollingAfterMs?: number
  pollingIntervalMs?: number
  pingIntervalMs?: number
}

const defaultReconnectBackoffMs = [1000, 2000, 5000, 10000]
const defaultPollingAfterMs = 15_000
const defaultPollingIntervalMs = 2000
const defaultPingIntervalMs = 25_000

export class ConnectionManager {
  private socket: WebSocket | null = null
  private reconnectTimer: number | null = null
  private pollingTimer: number | null = null
  private pingTimer: number | null = null
  private reconnectAttempt = 0
  private disconnectedAt = 0
  private stopped = true
  private reconnectBlocked = false
  private state: ConnectionState = 'disconnected'
  private readonly createSocket: (url: string) => WebSocket
  private readonly reconnectBackoffMs: number[]
  private readonly pollingAfterMs: number
  private readonly pollingIntervalMs: number
  private readonly pingIntervalMs: number
  private readonly options: ConnectionManagerOptions

  constructor(options: ConnectionManagerOptions) {
    this.options = options
    this.createSocket = options.createSocket ?? ((url) => new WebSocket(url))
    this.reconnectBackoffMs = options.reconnectBackoffMs ?? defaultReconnectBackoffMs
    this.pollingAfterMs = options.pollingAfterMs ?? defaultPollingAfterMs
    this.pollingIntervalMs = options.pollingIntervalMs ?? defaultPollingIntervalMs
    this.pingIntervalMs = options.pingIntervalMs ?? defaultPingIntervalMs
  }

  start() {
    if (!this.stopped) {
      return
    }

    this.stopped = false
    document.addEventListener('visibilitychange', this.handleVisibilityChange)
    void this.refreshStatus().finally(() => this.connect())
  }

  stop() {
    this.stopped = true
    document.removeEventListener('visibilitychange', this.handleVisibilityChange)
    this.clearReconnectTimer()
    this.stopPolling()
    this.stopPing()
    this.closeSocket(true)
    this.transition('disconnected')
  }

  reconnect() {
    this.reconnectBlocked = false
    this.reconnectAttempt = 0
    this.disconnectedAt = 0
    this.clearReconnectTimer()
    this.connect()
  }

  closeSocketForTest() {
    this.socket?.close()
  }

  rejectReconnectsForTest() {
    this.reconnectBlocked = true
    this.clearReconnectTimer()
    this.closeSocketForTest()
    this.startPolling()
  }

  allowReconnectsForTest() {
    this.reconnectBlocked = false
    this.stopPolling()
    this.reconnect()
  }

  forcePollingForTest() {
    this.startPolling()
  }

  getState() {
    return this.state
  }

  private connect = () => {
    if (this.stopped) {
      return
    }
    if (this.reconnectBlocked) {
      this.startPolling()
      return
    }

    this.clearReconnectTimer()
    this.stopPing()
    this.closeSocket(true)
    if (this.pollingTimer === null) {
      this.transition('reconnecting')
    }

    let nextSocket: WebSocket
    try {
      nextSocket = this.createSocket(this.options.buildUrl())
    } catch (error) {
      this.options.onError(error instanceof Error ? error.message : 'WebSocket 创建失败')
      this.handleDisconnected()
      return
    }

    this.socket = nextSocket
    nextSocket.onopen = this.handleOpen
    nextSocket.onmessage = this.handleMessage
    nextSocket.onerror = this.handleError
    nextSocket.onclose = this.handleClose
  }

  private handleOpen = () => {
    this.reconnectAttempt = 0
    this.disconnectedAt = 0
    this.stopPolling()
    this.options.onError(null)
    this.transition('connected')
    this.startPing()
  }

  private handleMessage = (message: MessageEvent) => {
    try {
      this.options.onMessage(JSON.parse(message.data))
    } catch (error) {
      this.options.onError(error instanceof Error ? error.message : 'WebSocket 消息解析失败')
    }
  }

  private handleError = () => {
    this.options.onError('WebSocket 连接异常')
  }

  private handleClose = () => {
    this.handleDisconnected()
  }

  private handleDisconnected() {
    this.stopPing()
    if (this.stopped) {
      return
    }

    void this.refreshStatus()
    if (this.disconnectedAt === 0) {
      this.disconnectedAt = Date.now()
    }

    if (Date.now() - this.disconnectedAt >= this.pollingAfterMs || this.reconnectBlocked) {
      this.startPolling()
    } else {
      this.transition('reconnecting')
    }

    this.scheduleReconnect()
  }

  private scheduleReconnect() {
    if (this.stopped || this.reconnectBlocked) {
      return
    }

    const delay =
      this.reconnectBackoffMs[Math.min(this.reconnectAttempt, this.reconnectBackoffMs.length - 1)]
    this.reconnectAttempt += 1
    this.reconnectTimer = window.setTimeout(this.connect, delay)
  }

  private startPing() {
    this.stopPing()
    this.pingTimer = window.setInterval(() => {
      if (this.socket?.readyState === WebSocket.OPEN) {
        this.socket.send(JSON.stringify({ type: 'ping', client_time: new Date().toISOString() }))
      }
    }, this.pingIntervalMs)
  }

  private stopPing() {
    if (this.pingTimer !== null) {
      window.clearInterval(this.pingTimer)
      this.pingTimer = null
    }
  }

  private startPolling() {
    if (this.stopped) {
      return
    }

    this.transition('polling')
    if (this.pollingTimer !== null) {
      return
    }

    void this.refreshStatus()
    this.pollingTimer = window.setInterval(() => {
      void this.refreshStatus()
    }, this.pollingIntervalMs)
  }

  private stopPolling() {
    if (this.pollingTimer !== null) {
      window.clearInterval(this.pollingTimer)
      this.pollingTimer = null
    }
  }

  private closeSocket(silent: boolean) {
    if (!this.socket) {
      return
    }
    if (silent) {
      this.socket.onclose = null
      this.socket.onerror = null
    }
    this.socket.close()
    this.socket = null
  }

  private clearReconnectTimer() {
    if (this.reconnectTimer !== null) {
      window.clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }

  private transition(state: ConnectionState) {
    if (this.state === state) {
      return
    }
    this.state = state
    this.options.onStateChange(state)
  }

  private refreshStatus = async () => {
    try {
      await this.options.refreshStatus()
    } catch (error) {
      this.options.onError(error instanceof Error ? error.message : '状态同步失败')
    }
  }

  private handleVisibilityChange = () => {
    if (document.visibilityState === 'visible') {
      void this.refreshStatus()
    }
  }
}
