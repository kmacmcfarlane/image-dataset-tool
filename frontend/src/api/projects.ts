import { apiFetch } from './client'

export interface ProjectResult {
  id: string
  slug: string
  name: string
  description: string
  created_at: string
  updated_at: string
}

export interface ProjectDetail extends ProjectResult {
  subject_count: number
}

export interface ProjectListResult {
  projects: ProjectResult[]
}

export interface CreateProjectPayload {
  name: string
  description?: string
}

export interface UpdateProjectPayload {
  name?: string
  description?: string
}

/** List all projects. */
export function listProjects(): Promise<ProjectListResult> {
  return apiFetch<ProjectListResult>('/v1/projects')
}

/** Get a single project by ID with subject count. */
export function getProject(id: string): Promise<ProjectDetail> {
  return apiFetch<ProjectDetail>(`/v1/projects/${encodeURIComponent(id)}`)
}

/** Create a new project. */
export function createProject(payload: CreateProjectPayload): Promise<ProjectResult> {
  return apiFetch<ProjectResult>('/v1/projects', {
    method: 'POST',
    body: payload,
  })
}

/** Update a project's name and/or description. */
export function updateProject(id: string, payload: UpdateProjectPayload): Promise<ProjectResult> {
  return apiFetch<ProjectResult>(`/v1/projects/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: payload,
  })
}

/** Delete a project. If hardDelete is true, also removes the directory from disk. */
export function deleteProject(id: string, hardDelete = false): Promise<void> {
  const params = hardDelete ? '?hard_delete=true' : ''
  return apiFetch<void>(`/v1/projects/${encodeURIComponent(id)}${params}`, {
    method: 'DELETE',
  })
}
