import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSamplesStore } from '../samples'

// Mock the API module.
vi.mock('../../api/samples', () => ({
  listSamples: vi.fn(),
}))

import { listSamples } from '../../api/samples'

const mockListSamples = vi.mocked(listSamples)

// Mock EventSource.
class MockEventSource {
  static instances: MockEventSource[] = []
  listeners: Record<string, ((event: MessageEvent) => void)[]> = {}
  readyState = 1
  url: string

  constructor(url: string) {
    this.url = url
    MockEventSource.instances.push(this)
  }

  addEventListener(type: string, handler: (event: MessageEvent) => void): void {
    if (!this.listeners[type]) this.listeners[type] = []
    this.listeners[type].push(handler)
  }

  close(): void {
    this.readyState = 2
  }

  // Test helper: simulate an event.
  _emit(type: string, data: unknown): void {
    const handlers = this.listeners[type] || []
    for (const handler of handlers) {
      handler(new MessageEvent(type, { data: JSON.stringify(data) }))
    }
  }
}

describe('useSamplesStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    MockEventSource.instances = []
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    ;(globalThis as any).EventSource = MockEventSource
  })

  afterEach(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    delete (globalThis as any).EventSource
  })

  describe('connectAndFetch', () => {
    it('fetches samples and populates the map', async () => {
      mockListSamples.mockResolvedValue({
        samples: [
          {
            id: 'sam-1',
            subject_id: 'sub-1',
            source_platform: 'instagram',
            source_post_id: 'post1',
            slide_index: 0,
            width: 800,
            height: 600,
            status: 'pending',
            is_duplicate: false,
            thumbnail_path: '/v1/media/sam-1/thumb',
            created_at: '2026-01-01T00:00:00Z',
            updated_at: '2026-01-01T00:00:00Z',
          },
        ],
      })

      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      expect(mockListSamples).toHaveBeenCalledWith('sub-1')
      expect(store.totalCount).toBe(1)
      expect(store.allSamples[0].id).toBe('sam-1')
      expect(store.loading).toBe(false)
      expect(store.error).toBeNull()
    })

    it('sets error when fetch fails', async () => {
      mockListSamples.mockRejectedValue(new Error('Network error'))

      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      expect(store.error).toBe('Network error')
      expect(store.loading).toBe(false)
    })

    it('connects EventSource before fetching', async () => {
      mockListSamples.mockResolvedValue({ samples: [] })

      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      expect(MockEventSource.instances).toHaveLength(1)
      expect(MockEventSource.instances[0].url).toBe('/api/sse')
    })
  })

  describe('SSE event handling', () => {
    it('adds new sample from sample.new event after fetch complete', async () => {
      mockListSamples.mockResolvedValue({ samples: [] })

      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      const es = MockEventSource.instances[0]
      es._emit('sample.new', {
        id: 'new-sam',
        subject_id: 'sub-1',
        status: 'pending',
        is_duplicate: false,
        thumbnail_path: '/v1/media/new-sam/thumb',
      })

      expect(store.totalCount).toBe(1)
      expect(store.allSamples[0].id).toBe('new-sam')
    })

    it('ignores sample.new for different subject', async () => {
      mockListSamples.mockResolvedValue({ samples: [] })

      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      const es = MockEventSource.instances[0]
      es._emit('sample.new', {
        id: 'other-sam',
        subject_id: 'sub-other',
        status: 'pending',
        is_duplicate: false,
        thumbnail_path: '/v1/media/other-sam/thumb',
      })

      expect(store.totalCount).toBe(0)
    })

    it('deduplicates sample.new events by ID', async () => {
      mockListSamples.mockResolvedValue({
        samples: [
          {
            id: 'sam-1',
            subject_id: 'sub-1',
            source_platform: 'instagram',
            source_post_id: 'post1',
            slide_index: 0,
            width: 800,
            height: 600,
            status: 'pending',
            is_duplicate: false,
            thumbnail_path: '/v1/media/sam-1/thumb',
            created_at: '2026-01-01T00:00:00Z',
            updated_at: '2026-01-01T00:00:00Z',
          },
        ],
      })

      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      const es = MockEventSource.instances[0]
      es._emit('sample.new', {
        id: 'sam-1', // same ID as already fetched
        subject_id: 'sub-1',
        status: 'pending',
        is_duplicate: false,
        thumbnail_path: '/v1/media/sam-1/thumb',
      })

      expect(store.totalCount).toBe(1) // Still 1, not 2
    })

    it('updates existing sample from sample.updated event', async () => {
      mockListSamples.mockResolvedValue({
        samples: [
          {
            id: 'sam-1',
            subject_id: 'sub-1',
            source_platform: 'instagram',
            source_post_id: 'post1',
            slide_index: 0,
            width: 800,
            height: 600,
            status: 'pending',
            is_duplicate: false,
            thumbnail_path: '/v1/media/sam-1/thumb',
            created_at: '2026-01-01T00:00:00Z',
            updated_at: '2026-01-01T00:00:00Z',
          },
        ],
      })

      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      const es = MockEventSource.instances[0]
      es._emit('sample.updated', {
        id: 'sam-1',
        status: 'kept',
      })

      expect(store.allSamples[0].status).toBe('kept')
    })
  })

  describe('filters', () => {
    beforeEach(async () => {
      mockListSamples.mockResolvedValue({
        samples: [
          {
            id: 'pending-1', subject_id: 'sub-1', source_platform: 'ig',
            source_post_id: 'p1', slide_index: 0, width: 100, height: 100,
            status: 'pending', is_duplicate: false,
            thumbnail_path: '/t/1', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
          },
          {
            id: 'kept-1', subject_id: 'sub-1', source_platform: 'ig',
            source_post_id: 'p2', slide_index: 0, width: 100, height: 100,
            status: 'kept', is_duplicate: false,
            thumbnail_path: '/t/2', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
          },
          {
            id: 'dup-1', subject_id: 'sub-1', source_platform: 'ig',
            source_post_id: 'p3', slide_index: 0, width: 100, height: 100,
            status: 'pending', is_duplicate: true, duplicate_of: '["pending-1"]',
            thumbnail_path: '/t/3', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
          },
        ],
      })
    })

    it('defaults: shows pending only, hides duplicates', async () => {
      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      // Default: showPending=true, showKept=false, showRejected=false, showDuplicates=false
      expect(store.filteredSamples).toHaveLength(1)
      expect(store.filteredSamples[0].id).toBe('pending-1')
    })

    it('shows kept when filter enabled', async () => {
      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      store.setStatusFilter('kept', true)
      expect(store.filteredSamples.map(s => s.id)).toContain('kept-1')
    })

    it('shows duplicates when toggle enabled', async () => {
      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      store.setShowDuplicates(true)
      expect(store.filteredSamples.map(s => s.id)).toContain('dup-1')
    })
  })

  describe('disconnect', () => {
    it('closes EventSource', async () => {
      mockListSamples.mockResolvedValue({ samples: [] })

      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      const es = MockEventSource.instances[0]
      store.disconnect()

      expect(es.readyState).toBe(2)
    })
  })
})
