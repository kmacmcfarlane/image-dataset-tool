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

// --- Edit operations (S-019) ---

/** Crop box region. */
export interface CropBox {
  x: number
  y: number
  w: number
  h: number
}

/** Request body for PUT /v1/samples/{id}/edits. */
export interface EditRequest {
  rotation_deg: number
  crop_box?: CropBox | null
  auto_actions: string[]
}

/** Response shape for edit operations. */
export interface EditResponse {
  sample_id: string
  rotation_deg: number
  crop_box: CropBox | null
  auto_actions: string[]
  created_at?: string
  updated_at?: string
}

/** Fetch current edits for a sample. */
export function getSampleEdits(sampleId: string): Promise<EditResponse> {
  return apiFetch<EditResponse>(
    `/v1/samples/${encodeURIComponent(sampleId)}/edits`,
  )
}

/** Create or update edits for a sample (idempotent). */
export function updateSampleEdits(
  sampleId: string,
  edits: EditRequest,
): Promise<EditResponse> {
  return apiFetch<EditResponse>(
    `/v1/samples/${encodeURIComponent(sampleId)}/edits`,
    { method: 'PUT', body: edits },
  )
}

/** Reset all edits for a sample (returns to original). */
export function resetSampleEdits(sampleId: string): Promise<void> {
  return apiFetch<void>(
    `/v1/samples/${encodeURIComponent(sampleId)}/edits`,
    { method: 'DELETE' },
  )
}
