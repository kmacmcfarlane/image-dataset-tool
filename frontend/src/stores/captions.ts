import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Caption } from '../api/captions'
import { listCaptions, saveCaption } from '../api/captions'

/**
 * Captions store: manages caption data for the currently viewed sample.
 *
 * Loads captions grouped by study from the backend. Supports saving
 * manual edits which create/update the "manual" pseudo-study caption
 * while preserving original study captions.
 */
export const useCaptionsStore = defineStore('captions', () => {
  // --- State ---

  /** Captions for the currently loaded sample, keyed by sample ID. */
  const captionsBySampleId = ref<Map<string, Caption[]>>(new Map())

  const loading = ref(false)
  const saving = ref(false)
  const error = ref<string | null>(null)

  // --- Actions ---

  /** Load captions for a sample from the backend. */
  async function loadCaptions(sampleId: string): Promise<Caption[]> {
    loading.value = true
    error.value = null
    try {
      const result = await listCaptions(sampleId)
      const captions = result.captions ?? []
      captionsBySampleId.value.set(sampleId, captions)
      return captions
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to load captions'
      return []
    } finally {
      loading.value = false
    }
  }

  /**
   * Save a manual caption edit.
   * Creates the "manual" pseudo-study if it does not exist.
   * The original study caption is preserved; only the manual caption is updated.
   */
  async function saveManualCaption(sampleId: string, text: string): Promise<Caption | null> {
    saving.value = true
    error.value = null
    try {
      const caption = await saveCaption(sampleId, { text, is_manual: true })
      // Refresh the full list so the UI shows the updated state.
      await loadCaptions(sampleId)
      return caption
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to save caption'
      return null
    } finally {
      saving.value = false
    }
  }

  /** Get cached captions for a sample (returns empty array if not loaded). */
  function getCaptions(sampleId: string): Caption[] {
    return captionsBySampleId.value.get(sampleId) ?? []
  }

  /** Clear all cached caption data. */
  function clear(): void {
    captionsBySampleId.value = new Map()
    error.value = null
  }

  return {
    // State
    captionsBySampleId,
    loading,
    saving,
    error,
    // Actions
    loadCaptions,
    saveManualCaption,
    getCaptions,
    clear,
  }
})
