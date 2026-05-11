import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSamplesStore } from '../samples'

// Mock the API module.
vi.mock('../../api/samples', () => ({
  listSamples: vi.fn(),
  updateSampleStatus: vi.fn(),
  bulkUpdateSampleStatus: vi.fn(),
}))

import { listSamples, updateSampleStatus, bulkUpdateSampleStatus } from '../../api/samples'

const mockListSamples = vi.mocked(listSamples)
const mockUpdateSampleStatus = vi.mocked(updateSampleStatus)
const mockBulkUpdateSampleStatus = vi.mocked(bulkUpdateSampleStatus)

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

/** Helper to create a sample fixture. */
function makeSample(overrides: Partial<{
  id: string; subject_id: string; status: string; is_duplicate: boolean
}> = {}) {
  return {
    id: overrides.id ?? 'sam-1',
    subject_id: overrides.subject_id ?? 'sub-1',
    source_platform: 'instagram',
    source_post_id: 'post1',
    slide_index: 0,
    width: 800,
    height: 600,
    status: (overrides.status ?? 'pending') as 'pending' | 'kept' | 'rejected',
    is_duplicate: overrides.is_duplicate ?? false,
    thumbnail_path: `/v1/media/${overrides.id ?? 'sam-1'}/thumb`,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
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
        samples: [makeSample()],
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

    it('clears selection on new connectAndFetch', async () => {
      mockListSamples.mockResolvedValue({ samples: [makeSample()] })

      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')
      store.selectSample('sam-1')
      expect(store.selectedCount).toBe(1)

      await store.connectAndFetch('sub-1')
      expect(store.selectedCount).toBe(0)
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
        samples: [makeSample()],
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
        samples: [makeSample()],
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
          makeSample({ id: 'pending-1', status: 'pending' }),
          makeSample({ id: 'kept-1', status: 'kept' }),
          makeSample({ id: 'dup-1', status: 'pending', is_duplicate: true }),
        ],
      })
    })

    it('defaults: shows pending only, hides duplicates', async () => {
      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

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

  describe('selection', () => {
    beforeEach(async () => {
      mockListSamples.mockResolvedValue({
        samples: [
          makeSample({ id: 'sam-1' }),
          makeSample({ id: 'sam-2' }),
          makeSample({ id: 'sam-3' }),
          makeSample({ id: 'sam-4' }),
        ],
      })
    })

    it('selectSample selects one and deselects others', async () => {
      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      store.selectSample('sam-1')
      expect(store.selectedCount).toBe(1)
      expect(store.isSelected('sam-1')).toBe(true)

      store.selectSample('sam-2')
      expect(store.selectedCount).toBe(1)
      expect(store.isSelected('sam-1')).toBe(false)
      expect(store.isSelected('sam-2')).toBe(true)
    })

    it('toggleSelect adds/removes from selection', async () => {
      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      store.toggleSelect('sam-1')
      store.toggleSelect('sam-2')
      expect(store.selectedCount).toBe(2)

      store.toggleSelect('sam-1')
      expect(store.selectedCount).toBe(1)
      expect(store.isSelected('sam-2')).toBe(true)
    })

    // AC: Shift+click range select
    it('rangeSelect selects range between last click and target', async () => {
      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      store.selectSample('sam-1')
      store.rangeSelect('sam-3')

      expect(store.selectedCount).toBe(3)
      expect(store.isSelected('sam-1')).toBe(true)
      expect(store.isSelected('sam-2')).toBe(true)
      expect(store.isSelected('sam-3')).toBe(true)
    })

    // AC: Ctrl+A select all
    it('selectAll selects all filtered samples', async () => {
      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      store.selectAll()
      expect(store.selectedCount).toBe(4)
    })

    // AC: Ctrl+Shift+A deselect
    it('clearSelection deselects all', async () => {
      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      store.selectAll()
      store.clearSelection()
      expect(store.selectedCount).toBe(0)
    })

    it('moveFocus navigates through filtered samples', async () => {
      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      store.moveFocus('next')
      expect(store.focusedId).toBe('sam-1')

      store.moveFocus('next')
      expect(store.focusedId).toBe('sam-2')

      store.moveFocus('prev')
      expect(store.focusedId).toBe('sam-1')
    })

    it('moveFocus does not go past boundaries', async () => {
      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      store.moveFocus('prev') // at start
      expect(store.focusedId).toBe('sam-1')

      // Go to last
      store.selectSample('sam-4')
      store.moveFocus('next') // at end
      expect(store.focusedId).toBe('sam-4')
    })
  })

  describe('applyStatus', () => {
    beforeEach(async () => {
      mockUpdateSampleStatus.mockResolvedValue(makeSample({ id: 'sam-1', status: 'kept' }))
      mockBulkUpdateSampleStatus.mockResolvedValue({ updated: 2 })
      mockListSamples.mockResolvedValue({
        samples: [
          makeSample({ id: 'sam-1' }),
          makeSample({ id: 'sam-2' }),
          makeSample({ id: 'sam-3' }),
          makeSample({ id: 'sam-4' }),
        ],
      })
    })

    // AC: K=keep on selected samples
    it('applies status to selected samples optimistically', async () => {
      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      store.selectSample('sam-1')
      await store.applyStatus('kept')

      expect(mockUpdateSampleStatus).toHaveBeenCalledWith('sam-1', 'kept')
      // Optimistic: local state updated
      const sample = store.samplesMap.get('sam-1')
      expect(sample?.status).toBe('kept')
    })

    // AC: Single selection: auto-advance to next sample after keep/reject
    it('auto-advances to next sample after single keep', async () => {
      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      store.selectSample('sam-2')
      await store.applyStatus('kept')

      // Should advance to sam-3
      expect(store.focusedId).toBe('sam-3')
      expect(store.selectedCount).toBe(1)
      expect(store.isSelected('sam-3')).toBe(true)
    })

    // AC: Multi selection: auto-advance to sample after last in selection
    it('auto-advances to sample after last in multi-selection', async () => {
      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      store.selectSample('sam-1')
      store.rangeSelect('sam-3')
      await store.applyStatus('rejected')

      expect(mockBulkUpdateSampleStatus).toHaveBeenCalledWith(
        expect.arrayContaining(['sam-1', 'sam-2', 'sam-3']),
        'rejected',
      )
      // Should advance to sam-4
      expect(store.focusedId).toBe('sam-4')
      expect(store.selectedCount).toBe(1)
    })

    // AC: Status is reversible between any states
    it('allows reversing status from kept to pending', async () => {
      mockUpdateSampleStatus.mockResolvedValue(makeSample({ id: 'sam-1', status: 'pending' }))

      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      // First keep
      store.selectSample('sam-1')
      await store.applyStatus('kept')

      // Then revert to pending — need to re-select since auto-advance moved us
      store.selectSample('sam-1')
      await store.applyStatus('pending')

      const sample = store.samplesMap.get('sam-1')
      expect(sample?.status).toBe('pending')
    })

    it('uses focused sample when nothing selected', async () => {
      const store = useSamplesStore()
      await store.connectAndFetch('sub-1')

      store.moveFocus('next') // focuses sam-1
      await store.applyStatus('kept')

      expect(mockUpdateSampleStatus).toHaveBeenCalledWith('sam-1', 'kept')
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
