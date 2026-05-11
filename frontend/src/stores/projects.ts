import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  listProjects,
  createProject,
  updateProject,
  deleteProject,
} from '../api/projects'
import type {
  ProjectResult,
  CreateProjectPayload,
  UpdateProjectPayload,
} from '../api/projects'

/**
 * Projects store.
 * Manages the project list and CRUD operations.
 */
export const useProjectsStore = defineStore('projects', () => {
  const projects = ref<ProjectResult[]>([])
  const loading = ref(false)

  async function load(): Promise<void> {
    loading.value = true
    try {
      const result = await listProjects()
      projects.value = result.projects
    } finally {
      loading.value = false
    }
  }

  async function create(payload: CreateProjectPayload): Promise<ProjectResult> {
    const project = await createProject(payload)
    await load()
    return project
  }

  async function update(id: string, payload: UpdateProjectPayload): Promise<ProjectResult> {
    const project = await updateProject(id, payload)
    await load()
    return project
  }

  async function remove(id: string, hardDelete = false): Promise<void> {
    await deleteProject(id, hardDelete)
    await load()
  }

  return {
    projects,
    loading,
    load,
    create,
    update,
    remove,
  }
})
