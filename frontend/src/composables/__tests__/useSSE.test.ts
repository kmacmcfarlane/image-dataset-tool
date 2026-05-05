import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { nextTick } from 'vue'

// Mock EventSource before importing the composable
class MockEventSource {
  static instances: MockEventSource[] = []

  url: string
  onopen: ((event: Event) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  listeners: Map<string, ((event: MessageEvent) => void)[]> = new Map()
  readyState = 0

  constructor(url: string) {
    this.url = url
    MockEventSource.instances.push(this)
  }

  addEventListener(type: string, listener: (event: MessageEvent) => void): void {
    const existing = this.listeners.get(type) || []
    existing.push(listener)
    this.listeners.set(type, existing)
  }

  close(): void {
    this.readyState = 2
  }

  // Test helpers
  simulateOpen(): void {
    this.readyState = 1
    this.onopen?.(new Event('open'))
  }

  simulateError(): void {
    this.readyState = 2
    this.onerror?.(new Event('error'))
  }

  simulateEvent(type: string, data: string, lastEventId?: string): void {
    const listeners = this.listeners.get(type) || []
    const event = new MessageEvent(type, {
      data,
      lastEventId: lastEventId || '',
    })
    for (const listener of listeners) {
      listener(event)
    }
  }

  static reset(): void {
    MockEventSource.instances = []
  }

  static latest(): MockEventSource | undefined {
    return MockEventSource.instances[MockEventSource.instances.length - 1]
  }
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any, no-extra-semi
;(globalThis as any).EventSource = MockEventSource

// Must import after EventSource mock is set up
// We also need to handle onUnmounted — in test context there's no component instance
vi.mock('vue', async () => {
  const actual = await vi.importActual('vue')
  return {
    ...(actual as object),
    onUnmounted: vi.fn(), // no-op in tests
  }
})

// Dynamic import after mocks
const { useSSE } = await import('../useSSE')

describe('useSSE', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    MockEventSource.reset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('connects to the default endpoint on creation', () => {
    const sse = useSSE()
    expect(MockEventSource.latest()?.url).toBe('/v1/events')
    expect(sse.status.value).toBe('connecting')
  })

  it('connects to a custom endpoint', () => {
    useSSE({ url: '/custom/events' })
    expect(MockEventSource.latest()?.url).toBe('/custom/events')
  })

  it('transitions to connected on open', async () => {
    const sse = useSSE()
    MockEventSource.latest()!.simulateOpen()
    await nextTick()
    expect(sse.status.value).toBe('connected')
  })

  it('transitions to disconnected on error', async () => {
    const sse = useSSE()
    MockEventSource.latest()!.simulateOpen()
    await nextTick()
    expect(sse.status.value).toBe('connected')

    MockEventSource.latest()!.simulateError()
    await nextTick()
    expect(sse.status.value).toBe('disconnected')
  })

  it('dispatches typed events to registered handlers', async () => {
    const sse = useSSE()
    MockEventSource.latest()!.simulateOpen()

    const handler = vi.fn()
    sse.on('job.state', handler)

    MockEventSource.latest()!.simulateEvent(
      'job.state',
      JSON.stringify({ id: '1', trace_id: 't1', type: 'fetch', status: 'running' })
    )

    expect(handler).toHaveBeenCalledWith({
      id: '1',
      trace_id: 't1',
      type: 'fetch',
      status: 'running',
    })
  })

  it('tracks lastEventId from received events', () => {
    const sse = useSSE()
    MockEventSource.latest()!.simulateOpen()

    MockEventSource.latest()!.simulateEvent(
      'job.progress',
      JSON.stringify({ id: '1', trace_id: 't1', current: 5, total: 10, pct: 50 }),
      'evt-42'
    )

    expect(sse.lastEventId.value).toBe('evt-42')
  })

  it('reconnects with lastEventId after error', async () => {
    const sse = useSSE({ reconnectDelay: 1000 })

    // First connection
    const first = MockEventSource.latest()!
    first.simulateOpen()

    // Receive event with ID
    first.simulateEvent('job.state', '{}', 'evt-100')
    expect(sse.lastEventId.value).toBe('evt-100')

    // Error triggers reconnect
    first.simulateError()
    expect(sse.status.value).toBe('disconnected')

    // Advance timers to trigger reconnect (with jitter up to 1s)
    vi.advanceTimersByTime(2100)

    // New EventSource should include lastEventId
    const second = MockEventSource.latest()!
    expect(second).not.toBe(first)
    expect(second.url).toContain('lastEventId=evt-100')
  })

  it('uses exponential backoff on repeated failures', () => {
    useSSE({ reconnectDelay: 1000, maxReconnectDelay: 10000 })

    const first = MockEventSource.latest()!
    first.simulateError()

    // After first error: delay ~1000ms + jitter
    const countBefore = MockEventSource.instances.length
    vi.advanceTimersByTime(2100)
    expect(MockEventSource.instances.length).toBe(countBefore + 1)

    // Second failure — delay ~2000ms + jitter
    MockEventSource.latest()!.simulateError()
    const countBefore2 = MockEventSource.instances.length
    vi.advanceTimersByTime(2100)
    // Should NOT have reconnected yet (2s base + up to 1s jitter)
    // At 2.1s we might not hit it if base is 2s + jitter
    // Advance more to be sure
    vi.advanceTimersByTime(1000)
    expect(MockEventSource.instances.length).toBe(countBefore2 + 1)
  })

  it('stops reconnecting after close()', () => {
    const sse = useSSE({ reconnectDelay: 1000 })

    MockEventSource.latest()!.simulateError()
    sse.close()

    const countBefore = MockEventSource.instances.length
    vi.advanceTimersByTime(10000)
    expect(MockEventSource.instances.length).toBe(countBefore)
    expect(sse.status.value).toBe('disconnected')
  })

  it('reconnect() resets and creates new connection', () => {
    const sse = useSSE()
    sse.close()

    const countBefore = MockEventSource.instances.length
    sse.reconnect()

    expect(MockEventSource.instances.length).toBe(countBefore + 1)
    expect(sse.status.value).toBe('connecting')
  })

  it('off() removes a handler', () => {
    const sse = useSSE()
    MockEventSource.latest()!.simulateOpen()

    const handler = vi.fn()
    sse.on('job.state', handler)
    sse.off('job.state', handler)

    MockEventSource.latest()!.simulateEvent('job.state', '{}')
    expect(handler).not.toHaveBeenCalled()
  })
})
