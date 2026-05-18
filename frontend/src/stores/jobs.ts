import { defineStore } from 'pinia'
import { ref } from 'vue'
import { listJobs, listJobMessages, getJob } from '../api/jobs'
import type { JobResult, JobMessage, JobLevel } from '../api/jobs'

/**
 * Jobs store.
 * Manages job list, per-job SSE progress updates, and message panel state.
 */
export const useJobsStore = defineStore('jobs', () => {
  const jobs = ref<JobResult[]>([])
  const total = ref(0)
  const loading = ref(false)

  // Per-job messages panel state.
  const selectedJobId = ref<string | null>(null)
  const messages = ref<JobMessage[]>([])
  const messagesTotal = ref(0)
  const messagesLoading = ref(false)
  const messageLevel = ref<JobLevel>('warning')

  // SSE-driven live progress: keyed by job ID.
  const liveProgress = ref<Record<string, { current: number; total: number; pct: number }>>({})
  const liveStatus = ref<Record<string, string>>({})

  /** Load the job list. */
  async function load(params: { type?: string; status?: string; offset?: number; limit?: number } = {}): Promise<void> {
    loading.value = true
    try {
      const result = await listJobs({ limit: 50, ...params })
      jobs.value = result.jobs
      total.value = result.total
    } finally {
      loading.value = false
    }
  }

  /** Open the message panel for a job. */
  async function selectJob(id: string): Promise<void> {
    selectedJobId.value = id
    await loadMessages(id, messageLevel.value)
  }

  /** Close the message panel. */
  function deselectJob(): void {
    selectedJobId.value = null
    messages.value = []
    messagesTotal.value = 0
  }

  /** Load (or reload) messages for the selected job with the current level filter. */
  async function loadMessages(id: string, level: JobLevel = messageLevel.value): Promise<void> {
    messagesLoading.value = true
    try {
      const result = await listJobMessages(id, { level, limit: 100 })
      messages.value = result.messages
      messagesTotal.value = result.total
    } finally {
      messagesLoading.value = false
    }
  }

  /** Change the level filter and reload messages. */
  async function setMessageLevel(level: JobLevel): Promise<void> {
    messageLevel.value = level
    if (selectedJobId.value) {
      await loadMessages(selectedJobId.value, level)
    }
  }

  /** Apply a job.progress SSE event to live progress map. */
  function applyProgressEvent(payload: { id: string; current: number; total: number; pct: number }): void {
    liveProgress.value = {
      ...liveProgress.value,
      [payload.id]: { current: payload.current, total: payload.total, pct: payload.pct },
    }
  }

  /** Apply a job.state SSE event — refresh the job in-place or reload list. */
  async function applyStateEvent(payload: { id: string; type: string; status: string }): Promise<void> {
    liveStatus.value = { ...liveStatus.value, [payload.id]: payload.status }
    // Reload detail for this job to get updated counts.
    try {
      const updated = await getJob(payload.id)
      const idx = jobs.value.findIndex((j) => j.id === updated.id)
      if (idx >= 0) {
        jobs.value = [
          ...jobs.value.slice(0, idx),
          updated,
          ...jobs.value.slice(idx + 1),
        ]
      } else {
        // New job — prepend to list.
        jobs.value = [updated, ...jobs.value]
        total.value += 1
      }
    } catch {
      // Non-fatal: list will catch up on next manual refresh.
    }
  }

  return {
    jobs,
    total,
    loading,
    selectedJobId,
    messages,
    messagesTotal,
    messagesLoading,
    messageLevel,
    liveProgress,
    liveStatus,
    load,
    selectJob,
    deselectJob,
    loadMessages,
    setMessageLevel,
    applyProgressEvent,
    applyStateEvent,
  }
})
