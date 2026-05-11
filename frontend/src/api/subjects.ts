import { apiFetch } from './client'

export interface SubjectResult {
  id: string
  project_id: string
  slug: string
  name: string
  sample_count: number
  created_at: string
  updated_at: string
}

export interface SubjectLinkedAccount {
  id: string
  platform: string
  handle: string
}

export interface SubjectSampleStats {
  pending: number
  kept: number
  rejected: number
  total: number
}

export interface SubjectDetail extends Omit<SubjectResult, 'sample_count'> {
  linked_accounts: SubjectLinkedAccount[]
  sample_stats: SubjectSampleStats
}

export interface SubjectListResult {
  subjects: SubjectResult[]
}

export interface CreateSubjectPayload {
  name: string
}

export interface UpdateSubjectPayload {
  name?: string
}

/** List all subjects in a project. */
export function listSubjects(projectId: string): Promise<SubjectListResult> {
  return apiFetch<SubjectListResult>(
    `/v1/projects/${encodeURIComponent(projectId)}/subjects`,
  )
}

/** Get a single subject by ID with linked accounts and sample stats. */
export function getSubject(projectId: string, id: string): Promise<SubjectDetail> {
  return apiFetch<SubjectDetail>(
    `/v1/projects/${encodeURIComponent(projectId)}/subjects/${encodeURIComponent(id)}`,
  )
}

/** Create a new subject in a project. */
export function createSubject(
  projectId: string,
  payload: CreateSubjectPayload,
): Promise<SubjectResult> {
  return apiFetch<SubjectResult>(
    `/v1/projects/${encodeURIComponent(projectId)}/subjects`,
    { method: 'POST', body: payload },
  )
}

/** Update a subject's name. */
export function updateSubject(
  projectId: string,
  id: string,
  payload: UpdateSubjectPayload,
): Promise<SubjectResult> {
  return apiFetch<SubjectResult>(
    `/v1/projects/${encodeURIComponent(projectId)}/subjects/${encodeURIComponent(id)}`,
    { method: 'PUT', body: payload },
  )
}

/** Delete a subject. If hardDelete is true, also removes the directory from disk. */
export function deleteSubject(
  projectId: string,
  id: string,
  hardDelete = false,
): Promise<void> {
  const params = hardDelete ? '?hard_delete=true' : ''
  return apiFetch<void>(
    `/v1/projects/${encodeURIComponent(projectId)}/subjects/${encodeURIComponent(id)}${params}`,
    { method: 'DELETE' },
  )
}

export interface SubjectLinkedAccountListResult {
  accounts: SubjectLinkedAccount[]
}

export interface LinkAccountPayload {
  account_id: string
}

/** Link an existing account to a subject. */
export function linkAccount(
  projectId: string,
  subjectId: string,
  accountId: string,
): Promise<void> {
  return apiFetch<void>(
    `/v1/projects/${encodeURIComponent(projectId)}/subjects/${encodeURIComponent(subjectId)}/accounts`,
    { method: 'POST', body: { account_id: accountId } },
  )
}

/** List accounts linked to a subject. */
export function listLinkedAccounts(
  projectId: string,
  subjectId: string,
): Promise<SubjectLinkedAccountListResult> {
  return apiFetch<SubjectLinkedAccountListResult>(
    `/v1/projects/${encodeURIComponent(projectId)}/subjects/${encodeURIComponent(subjectId)}/accounts`,
  )
}

/** Unlink an account from a subject (removes join row only). */
export function unlinkAccount(
  projectId: string,
  subjectId: string,
  accountId: string,
): Promise<void> {
  return apiFetch<void>(
    `/v1/projects/${encodeURIComponent(projectId)}/subjects/${encodeURIComponent(subjectId)}/accounts/${encodeURIComponent(accountId)}`,
    { method: 'DELETE' },
  )
}
