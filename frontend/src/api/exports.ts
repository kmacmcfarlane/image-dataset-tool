import { apiFetch } from './client'

export interface CreateExportRequest {
  subject_ids: string[]
  study_slug?: string
  format?: string
  destination?: string
}

export interface CreateExportResult {
  job_id: string
  trace_id: string
  status: string
  total_items: number
  output_path: string
}

export interface DownloadZipRequest {
  subject_ids: string[]
  study_slug?: string
  format?: string
}

/** Trigger a server-side export via NATS pipeline. */
export function createExport(req: CreateExportRequest): Promise<CreateExportResult> {
  return apiFetch<CreateExportResult>('/v1/exports', {
    method: 'POST',
    body: req,
  })
}

/**
 * Download a zip archive of exported samples.
 * Triggers a browser download via a standard save dialog.
 * Uses raw fetch (not apiFetch) since the response is a binary blob.
 */
export async function downloadZip(req: DownloadZipRequest): Promise<void> {
  const response = await fetch('/v1/exports/zip', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })

  if (!response.ok) {
    let message = `HTTP ${response.status}`
    try {
      const body = (await response.json()) as Record<string, unknown>
      if (typeof body.message === 'string') message = body.message
    } catch {
      // Body was not JSON
    }
    throw new Error(message)
  }

  // Extract filename from Content-Disposition header if available.
  const disposition = response.headers.get('Content-Disposition')
  let filename = 'export.zip'
  if (disposition) {
    const match = disposition.match(/filename="?([^"]+)"?/)
    if (match) filename = match[1]
  }

  // Create a blob URL and trigger download via an anchor element.
  const blob = await response.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
