import { apiFetch } from './client'

export interface JobResult {
  id: string
  type: string
  subject_id?: string
  study_id?: string
  status: string
  total_items?: number
  completed_items: number
  failed_items: number
  started_at: string
  finished_at?: string
  created_at: string
}

export interface JobDetail extends JobResult {}

export interface JobListResult {
  jobs: JobResult[]
  total: number
}

export interface JobMessage {
  id: string
  job_run_id: string
  level: string
  sample_ref?: string
  account_handle?: string
  message: string
  created_at: string
}

export interface JobMessageListResult {
  messages: JobMessage[]
  total: number
}

export type JobLevel = 'error' | 'warning' | 'info'

export interface ListJobsParams {
  type?: string
  status?: string
  offset?: number
  limit?: number
}

export interface ListJobMessagesParams {
  level?: JobLevel
  offset?: number
  limit?: number
}

/** List job runs with optional filters. */
export function listJobs(params: ListJobsParams = {}): Promise<JobListResult> {
  const qs = new URLSearchParams()
  if (params.type) qs.set('type', params.type)
  if (params.status) qs.set('status', params.status)
  if (params.offset !== undefined) qs.set('offset', String(params.offset))
  if (params.limit !== undefined) qs.set('limit', String(params.limit))
  const query = qs.toString() ? `?${qs.toString()}` : ''
  return apiFetch<JobListResult>(`/v1/jobs${query}`)
}

/** Get a single job run by ID. */
export function getJob(id: string): Promise<JobDetail> {
  return apiFetch<JobDetail>(`/v1/jobs/${encodeURIComponent(id)}`)
}

/** List messages for a job run. */
export function listJobMessages(
  id: string,
  params: ListJobMessagesParams = {},
): Promise<JobMessageListResult> {
  const qs = new URLSearchParams()
  if (params.level) qs.set('level', params.level)
  if (params.offset !== undefined) qs.set('offset', String(params.offset))
  if (params.limit !== undefined) qs.set('limit', String(params.limit))
  const query = qs.toString() ? `?${qs.toString()}` : ''
  return apiFetch<JobMessageListResult>(
    `/v1/jobs/${encodeURIComponent(id)}/messages${query}`,
  )
}
