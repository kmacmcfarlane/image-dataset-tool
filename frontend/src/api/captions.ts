import { apiFetch } from './client'

/** Caption study metadata as returned by the API. */
export interface CaptionStudy {
  id: string
  name: string
  slug: string
  provider: string
  model: string
}

/** Single caption with its parent study metadata. */
export interface Caption {
  id: string
  sample_id: string
  study_id: string
  text: string
  study: CaptionStudy
  created_at: string
}

/** Response shape for GET /v1/samples/{id}/captions. */
export interface CaptionListResult {
  captions: Caption[]
}

/** Request body for PUT /v1/samples/{id}/captions. */
export interface SaveCaptionRequest {
  text: string
  study_id?: string
  is_manual?: boolean
}

/** Fetch all captions for a sample (grouped by study via backend). */
export function listCaptions(sampleId: string): Promise<CaptionListResult> {
  return apiFetch<CaptionListResult>(
    `/v1/samples/${encodeURIComponent(sampleId)}/captions`,
  )
}

/** Save a caption (manual edit or study-specific). Returns the saved caption. */
export function saveCaption(
  sampleId: string,
  req: SaveCaptionRequest,
): Promise<Caption> {
  return apiFetch<Caption>(
    `/v1/samples/${encodeURIComponent(sampleId)}/captions`,
    { method: 'PUT', body: req },
  )
}
