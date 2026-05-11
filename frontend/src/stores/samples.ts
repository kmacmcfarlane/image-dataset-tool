import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Sample, SampleNewEvent, SampleUpdatedEvent, EditRequest, EditResponse } from '../api/samples'
import { listSamples, updateSampleStatus, bulkUpdateSampleStatus, updateSampleEdits, resetSampleEdits, getSampleEdits } from '../api/samples'

/**
 * Samples store implementing the connect-then-fetch pattern (PRD §8.3, §11.5).
 *
 * Flow:
 * 1. Connect SSE first
 * 2. Fetch full metadata array
 * 3. Buffer SSE events received during fetch
 * 4. Merge buffered events by ID (idempotent)
 * 5. Continue applying real-time SSE events
 *
 * Also manages selection state, keep/reject actions, and auto-advance (S-018).
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

  // Selection state.
  const selectedIds = ref<Set<string>>(new Set())
  const focusedId = ref<string | null>(null)
  const lastClickedId = ref<string | null>(null)

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

  const selectedCount = computed(() => selectedIds.value.size)

  const isSelected = computed(() => (id: string) => selectedIds.value.has(id))

  // --- Actions ---

  /** Connect SSE, fetch metadata, merge. Call when entering sample grid view. */
  async function connectAndFetch(sid: string): Promise<void> {
    subjectId.value = sid
    loading.value = true
    error.value = null
    fetchComplete = false
    bufferedEvents = []
    samplesMap.value = new Map()
    clearSelection()

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

  // --- Selection actions ---

  /** Select a single sample (click without modifiers). */
  function selectSample(id: string): void {
    selectedIds.value = new Set([id])
    focusedId.value = id
    lastClickedId.value = id
  }

  /** Toggle selection of a single sample (Ctrl+click or Space). */
  function toggleSelect(id: string): void {
    const newSet = new Set(selectedIds.value)
    if (newSet.has(id)) {
      newSet.delete(id)
    } else {
      newSet.add(id)
    }
    selectedIds.value = newSet
    focusedId.value = id
    lastClickedId.value = id
  }

  /** Range select from last clicked to target (Shift+click). */
  function rangeSelect(id: string): void {
    const filtered = filteredSamples.value
    const lastIdx = lastClickedId.value
      ? filtered.findIndex((s) => s.id === lastClickedId.value)
      : -1
    const targetIdx = filtered.findIndex((s) => s.id === id)
    if (lastIdx === -1 || targetIdx === -1) {
      selectSample(id)
      return
    }

    const start = Math.min(lastIdx, targetIdx)
    const end = Math.max(lastIdx, targetIdx)
    const newSet = new Set(selectedIds.value)
    for (let i = start; i <= end; i++) {
      newSet.add(filtered[i].id)
    }
    selectedIds.value = newSet
    focusedId.value = id
  }

  /** Select all visible (filtered) samples. */
  function selectAll(): void {
    selectedIds.value = new Set(filteredSamples.value.map((s) => s.id))
  }

  /** Deselect all. */
  function clearSelection(): void {
    selectedIds.value = new Set()
  }

  /** Move focus (and optionally selection) to adjacent sample. */
  function moveFocus(direction: 'prev' | 'next'): void {
    const filtered = filteredSamples.value
    if (filtered.length === 0) return

    const currentIdx = focusedId.value
      ? filtered.findIndex((s) => s.id === focusedId.value)
      : -1

    let newIdx: number
    if (currentIdx === -1) {
      newIdx = 0
    } else if (direction === 'next') {
      newIdx = Math.min(currentIdx + 1, filtered.length - 1)
    } else {
      newIdx = Math.max(currentIdx - 1, 0)
    }

    focusedId.value = filtered[newIdx].id
  }

  // --- Status actions ---

  /**
   * Apply status to selected samples. Auto-advances to the sample after the
   * last in selection (single selection: next sample; multi: after last in selection).
   *
   * Optimistically updates local state, then calls API.
   */
  async function applyStatus(status: 'kept' | 'rejected' | 'pending'): Promise<void> {
    const ids = Array.from(selectedIds.value)
    if (ids.length === 0 && focusedId.value) {
      ids.push(focusedId.value)
    }
    if (ids.length === 0) return

    // Determine advance target before changing state.
    const advanceTarget = computeAdvanceTarget(ids)

    // Optimistic update.
    for (const id of ids) {
      const sample = samplesMap.value.get(id)
      if (sample) {
        samplesMap.value.set(id, { ...sample, status, updated_at: new Date().toISOString() })
      }
    }

    // Auto-advance.
    clearSelection()
    if (advanceTarget) {
      selectSample(advanceTarget)
    }

    // API call.
    try {
      if (ids.length === 1) {
        await updateSampleStatus(ids[0], status)
      } else {
        await bulkUpdateSampleStatus(ids, status)
      }
    } catch (e) {
      // Revert optimistic update on failure — re-fetch.
      error.value = e instanceof Error ? e.message : 'Failed to update status'
      if (subjectId.value) {
        const result = await listSamples(subjectId.value)
        const newMap = new Map<string, Sample>()
        for (const sample of result.samples) {
          newMap.set(sample.id, sample)
        }
        samplesMap.value = newMap
      }
    }
  }

  /**
   * Compute the sample ID to auto-advance to after applying a status action.
   * Returns the next sample after the last selected sample in the filtered list,
   * or null if there's no valid target.
   */
  function computeAdvanceTarget(actionIds: string[]): string | null {
    const filtered = filteredSamples.value
    if (filtered.length === 0) return null

    // Find the highest index among selected IDs in the filtered list.
    let maxIdx = -1
    for (const id of actionIds) {
      const idx = filtered.findIndex((s) => s.id === id)
      if (idx > maxIdx) maxIdx = idx
    }

    if (maxIdx === -1) return null

    // Look for the first sample after maxIdx that is NOT in the action set.
    const actionSet = new Set(actionIds)
    for (let i = maxIdx + 1; i < filtered.length; i++) {
      if (!actionSet.has(filtered[i].id)) {
        return filtered[i].id
      }
    }

    // If no sample after, try before.
    for (let i = maxIdx - 1; i >= 0; i--) {
      if (!actionSet.has(filtered[i].id)) {
        return filtered[i].id
      }
    }

    return null
  }

  // --- Edit state (S-019) ---
  const editsBySampleId = ref<Map<string, EditResponse>>(new Map())
  const editSaving = ref(false)

  // Debounce timers keyed by sample ID.
  const debounceTimers = new Map<string, ReturnType<typeof setTimeout>>()
  const DEBOUNCE_MS = 500

  /** Load current edits for a sample from the backend. */
  async function loadEdits(sampleId: string): Promise<EditResponse | null> {
    try {
      const edits = await getSampleEdits(sampleId)
      editsBySampleId.value.set(sampleId, edits)
      return edits
    } catch {
      return null
    }
  }

  /**
   * Save edits for a sample with debounce (auto-save on edit).
   * Cancels any pending save for the same sample.
   */
  function saveEditsDebounced(sampleId: string, edits: EditRequest): void {
    // Cancel previous debounce for this sample.
    const existing = debounceTimers.get(sampleId)
    if (existing) {
      clearTimeout(existing)
    }

    debounceTimers.set(sampleId, setTimeout(async () => {
      debounceTimers.delete(sampleId)
      editSaving.value = true
      try {
        const result = await updateSampleEdits(sampleId, edits)
        editsBySampleId.value.set(sampleId, result)
      } catch (e) {
        error.value = e instanceof Error ? e.message : 'Failed to save edits'
      } finally {
        editSaving.value = false
      }
    }, DEBOUNCE_MS))
  }

  /** Save edits immediately (no debounce). */
  async function saveEditsImmediate(sampleId: string, edits: EditRequest): Promise<void> {
    // Cancel any pending debounce.
    const existing = debounceTimers.get(sampleId)
    if (existing) {
      clearTimeout(existing)
      debounceTimers.delete(sampleId)
    }

    editSaving.value = true
    try {
      const result = await updateSampleEdits(sampleId, edits)
      editsBySampleId.value.set(sampleId, result)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to save edits'
    } finally {
      editSaving.value = false
    }
  }

  /** Reset all edits for a sample to original (saves immediately). */
  async function resetEdits(sampleId: string): Promise<void> {
    // Cancel any pending debounce.
    const existing = debounceTimers.get(sampleId)
    if (existing) {
      clearTimeout(existing)
      debounceTimers.delete(sampleId)
    }

    editSaving.value = true
    try {
      await resetSampleEdits(sampleId)
      editsBySampleId.value.delete(sampleId)
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to reset edits'
    } finally {
      editSaving.value = false
    }
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
    selectedIds,
    focusedId,
    lastClickedId,
    editsBySampleId,
    editSaving,
    // Computed
    allSamples,
    filteredSamples,
    totalCount,
    selectedCount,
    isSelected,
    // Actions
    connectAndFetch,
    disconnect,
    setStatusFilter,
    setShowDuplicates,
    selectSample,
    toggleSelect,
    rangeSelect,
    selectAll,
    clearSelection,
    moveFocus,
    applyStatus,
    loadEdits,
    saveEditsDebounced,
    saveEditsImmediate,
    resetEdits,
  }
})
