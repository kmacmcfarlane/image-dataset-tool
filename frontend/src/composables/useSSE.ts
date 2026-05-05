import { ref, readonly, onUnmounted, type Ref, type DeepReadonly } from 'vue'
import type { SSEConnectionStatus, SSEEventType } from '../types/sse'

export interface SSEOptions {
  /** SSE endpoint URL. Defaults to '/v1/events'. */
  url?: string
  /** Reconnection delay in ms. Defaults to 3000. */
  reconnectDelay?: number
  /** Max reconnection delay in ms. Defaults to 30000. */
  maxReconnectDelay?: number
}

export interface SSEComposable {
  /** Current connection status */
  status: DeepReadonly<Ref<SSEConnectionStatus>>
  /** Last event ID received (for reconnection) */
  lastEventId: DeepReadonly<Ref<string>>
  /** Register a handler for a specific event type */
  on: (eventType: SSEEventType, handler: (data: unknown) => void) => void
  /** Remove a handler for a specific event type */
  off: (eventType: SSEEventType, handler: (data: unknown) => void) => void
  /** Manually close the connection */
  close: () => void
  /** Manually reconnect */
  reconnect: () => void
}

/**
 * SSE composable: connects to the backend SSE endpoint, reconnects with
 * Last-Event-ID, and exposes reactive connection status.
 *
 * Reconnection uses exponential backoff with jitter.
 */
export function useSSE(options: SSEOptions = {}): SSEComposable {
  const {
    url = '/v1/events',
    reconnectDelay = 3000,
    maxReconnectDelay = 30000,
  } = options

  const status = ref<SSEConnectionStatus>('disconnected')
  const lastEventId = ref('')
  const handlers = new Map<SSEEventType, Set<(data: unknown) => void>>()
  let eventSource: EventSource | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let currentDelay = reconnectDelay
  let closed = false

  const SSE_EVENT_TYPES: SSEEventType[] = [
    'job.state',
    'job.progress',
    'consumer.stats',
    'provider.rate',
    'sample.new',
    'sample.updated',
  ]

  function connect(): void {
    if (closed) return

    cleanup()
    status.value = 'connecting'

    const connectUrl = lastEventId.value
      ? `${url}?lastEventId=${encodeURIComponent(lastEventId.value)}`
      : url

    eventSource = new EventSource(connectUrl)

    eventSource.onopen = () => {
      status.value = 'connected'
      currentDelay = reconnectDelay // Reset backoff on successful connect
    }

    eventSource.onerror = () => {
      status.value = 'disconnected'
      cleanup()
      scheduleReconnect()
    }

    // Register listeners for each known event type
    for (const eventType of SSE_EVENT_TYPES) {
      eventSource.addEventListener(eventType, (event: MessageEvent) => {
        if (event.lastEventId) {
          lastEventId.value = event.lastEventId
        }
        const typeHandlers = handlers.get(eventType)
        if (typeHandlers) {
          let data: unknown
          try {
            data = JSON.parse(event.data as string)
          } catch {
            data = event.data
          }
          for (const handler of typeHandlers) {
            handler(data)
          }
        }
      })
    }
  }

  function cleanup(): void {
    if (eventSource) {
      eventSource.close()
      eventSource = null
    }
  }

  function scheduleReconnect(): void {
    if (closed) return
    if (reconnectTimer !== null) return

    // Exponential backoff with jitter
    const jitter = Math.random() * 1000
    const delay = Math.min(currentDelay + jitter, maxReconnectDelay)
    currentDelay = Math.min(currentDelay * 2, maxReconnectDelay)

    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      connect()
    }, delay)
  }

  function on(eventType: SSEEventType, handler: (data: unknown) => void): void {
    if (!handlers.has(eventType)) {
      handlers.set(eventType, new Set())
    }
    handlers.get(eventType)!.add(handler)
  }

  function off(eventType: SSEEventType, handler: (data: unknown) => void): void {
    handlers.get(eventType)?.delete(handler)
  }

  function close(): void {
    closed = true
    if (reconnectTimer !== null) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    cleanup()
    status.value = 'disconnected'
  }

  function reconnect(): void {
    closed = false
    currentDelay = reconnectDelay
    connect()
  }

  // Auto-connect
  connect()

  // Auto-cleanup on component unmount
  onUnmounted(() => {
    close()
  })

  return {
    status: readonly(status),
    lastEventId: readonly(lastEventId),
    on,
    off,
    close,
    reconnect,
  }
}
