import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Sample, SampleNewEvent, SampleUpdatedEvent } from '../api/samples'
import { listSamples } from '../api/samples'

/**
 * Samples store implementing the connect-then-fetch pattern (PRD §8.3, §11.5).
 *
 * Flow:
 * 1. Connect SSE first
 * 2. Fetch full metadata array
 * 3. Buffer SSE events received during fetch
 * 4. Merge buffered events by ID (idempotent)
 * 5. Continue applying real-time SSE events
 */
export const useSamplesStore = defineStore('samples', () => {
  // --- State ---
  const samplesMap = ref<Map<string, Sample>>(new Map())
  const loading = ref(false)
  const error = ref<string | null>(null)
  const subjectId = ref<string | null>(null)

  // Filter state (applied client-side).
  const showPending = ref(true)
  const showKept = ref(false)
  const showRejected = ref(false)
  const showDuplicates = ref(false)

  // SSE connection state.
  let eventSource: EventSource | null = null
  let bufferedEvents: Array<{ type: string; data: unknown }> = []
  let fetchComplete = false

  // --- Computed ---
  const allSamples = computed(() => Array.from(samplesMap.value.values()))

  const filteredSamples = computed(() => {
    return allSamples.value.filter((s) => {
      // Filter by duplicate status.
      if (!showDuplicates.value && s.is_duplicate) return false
      // Filter by status.
      if (s.status === 'pending' && !showPending.value) return false
      if (s.status === 'kept' && !showKept.value) return false
      if (s.status === 'rejected' && !showRejected.value) return false
      return true
    })
  })

  const totalCount = computed(() => samplesMap.value.size)

  // --- Actions ---

  /** Connect SSE, fetch metadata, merge. Call when entering sample grid view. */
  async function connectAndFetch(sid: string): Promise<void> {
    subjectId.value = sid
    loading.value = true
    error.value = null
    fetchComplete = false
    bufferedEvents = []
    samplesMap.value = new Map()

    // Step 1: Connect SSE first.
    connectSSE()

    // Step 2: Fetch full metadata.
    try {
      const result = await listSamples(sid)
      const newMap = new Map<string, Sample>()
      for (const sample of result.samples) {
        newMap.set(sample.id, sample)
      }
      samplesMap.value = newMap

      // Step 3: Apply buffered events.
      fetchComplete = true
      for (const evt of bufferedEvents) {
        applySSEEvent(evt.type, evt.data)
      }
      bufferedEvents = []
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load samples'
    } finally {
      loading.value = false
    }
  }

  /** Disconnect SSE and clear state. Call when leaving sample grid view. */
  function disconnect(): void {
    if (eventSource) {
      eventSource.close()
      eventSource = null
    }
    fetchComplete = false
    bufferedEvents = []
  }

  /** Connect to the SSE endpoint. */
  function connectSSE(): void {
    if (eventSource) {
      eventSource.close()
    }

    eventSource = new EventSource('/api/sse')

    eventSource.addEventListener('sample.new', (event: MessageEvent) => {
      try {
        const data = JSON.parse(event.data) as SampleNewEvent
        handleEvent('sample.new', data)
      } catch {
        // Ignore malformed events.
      }
    })

    eventSource.addEventListener('sample.updated', (event: MessageEvent) => {
      try {
        const data = JSON.parse(event.data) as SampleUpdatedEvent
        handleEvent('sample.updated', data)
      } catch {
        // Ignore malformed events.
      }
    })
  }

  /** Route event to buffer or apply directly. */
  function handleEvent(type: string, data: unknown): void {
    if (!fetchComplete) {
      bufferedEvents.push({ type, data })
      return
    }
    applySSEEvent(type, data)
  }

  /** Apply a single SSE event to the samples map (idempotent). */
  function applySSEEvent(type: string, data: unknown): void {
    if (type === 'sample.new') {
      const evt = data as SampleNewEvent
      // Only add if it belongs to our current subject.
      if (evt.subject_id !== subjectId.value) return
      // Dedup by ID — ignore if already present.
      if (samplesMap.value.has(evt.id)) return
      // Create a partial Sample from the SSE event.
      const sample: Sample = {
        id: evt.id,
        subject_id: evt.subject_id,
        source_platform: '',
        source_post_id: '',
        slide_index: 0,
        width: 0,
        height: 0,
        status: evt.status as Sample['status'],
        is_duplicate: evt.is_duplicate,
        thumbnail_path: evt.thumbnail_path || `/v1/media/${evt.id}/thumb`,
        created_at: new Date().toISOString(),
        updated_at: new Date().toISOString(),
      }
      samplesMap.value.set(evt.id, sample)
    } else if (type === 'sample.updated') {
      const evt = data as SampleUpdatedEvent
      const existing = samplesMap.value.get(evt.id)
      if (!existing) return
      // Merge updated fields.
      if (evt.status) {
        existing.status = evt.status as Sample['status']
      }
      existing.updated_at = new Date().toISOString()
      samplesMap.value.set(evt.id, { ...existing })
    }
  }

  /** Toggle status filter. */
  function setStatusFilter(status: 'pending' | 'kept' | 'rejected', enabled: boolean): void {
    if (status === 'pending') showPending.value = enabled
    else if (status === 'kept') showKept.value = enabled
    else if (status === 'rejected') showRejected.value = enabled
  }

  /** Toggle duplicate visibility. */
  function setShowDuplicates(show: boolean): void {
    showDuplicates.value = show
  }

  return {
    // State
    samplesMap,
    loading,
    error,
    subjectId,
    showPending,
    showKept,
    showRejected,
    showDuplicates,
    // Computed
    allSamples,
    filteredSamples,
    totalCount,
    // Actions
    connectAndFetch,
    disconnect,
    setStatusFilter,
    setShowDuplicates,
  }
})
