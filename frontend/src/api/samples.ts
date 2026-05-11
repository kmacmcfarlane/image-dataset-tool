import { apiFetch } from './client'

/** Sample metadata as returned by the backend API. */
export interface Sample {
  id: string
  subject_id: string
  source_platform: string
  source_post_id: string
  slide_index: number
  width: number
  height: number
  status: 'pending' | 'kept' | 'rejected'
  is_duplicate: boolean
  duplicate_of?: string
  thumbnail_path: string
  created_at: string
  updated_at: string
}

export interface SampleListResult {
  samples: Sample[]
}

/** SSE event payload for sample.new events. */
export interface SampleNewEvent {
  id: string
  subject_id: string
  status: string
  is_duplicate: boolean
  thumbnail_path: string
}

/** SSE event payload for sample.updated events. */
export interface SampleUpdatedEvent {
  id: string
  status?: string
  edits_changed?: boolean
  caption_changed?: boolean
}

/** Fetch all samples for a subject (no pagination per PRD §8.3). */
export function listSamples(subjectId: string): Promise<SampleListResult> {
  return apiFetch<SampleListResult>(
    `/v1/subjects/${encodeURIComponent(subjectId)}/samples`,
  )
}

/** Update status of a single sample. Returns the updated sample. */
export function updateSampleStatus(
  sampleId: string,
  status: 'pending' | 'kept' | 'rejected',
): Promise<Sample> {
  return apiFetch<Sample>(
    `/v1/samples/${encodeURIComponent(sampleId)}/status`,
    { method: 'PUT', body: { status } },
  )
}

/** Bulk status update response. */
export interface BulkUpdateStatusResult {
  updated: number
}

/** Atomically update status for multiple samples. */
export function bulkUpdateSampleStatus(
  ids: string[],
  status: 'pending' | 'kept' | 'rejected',
): Promise<BulkUpdateStatusResult> {
  return apiFetch<BulkUpdateStatusResult>('/v1/samples/bulk-status', {
    method: 'PUT',
    body: { ids, status },
  })
}
