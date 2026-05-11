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
