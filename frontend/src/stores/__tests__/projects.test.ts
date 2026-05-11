import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useProjectsStore } from '../projects'

// Mock the API module.
vi.mock('../../api/projects', () => ({
  listProjects: vi.fn(),
  createProject: vi.fn(),
  updateProject: vi.fn(),
  deleteProject: vi.fn(),
}))

import {
  listProjects,
  createProject,
  updateProject,
  deleteProject,
} from '../../api/projects'

const mockListProjects = vi.mocked(listProjects)
const mockCreateProject = vi.mocked(createProject)
const mockUpdateProject = vi.mocked(updateProject)
const mockDeleteProject = vi.mocked(deleteProject)

const sampleProject = {
  id: 'p-1',
  slug: 'my-project',
  name: 'My Project',
  description: 'A test project',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const sampleList = { projects: [sampleProject] }

describe('useProjectsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('load', () => {
    it('fetches and stores project list', async () => {
      mockListProjects.mockResolvedValue(sampleList)

      const store = useProjectsStore()
      await store.load()

      expect(mockListProjects).toHaveBeenCalledOnce()
      expect(store.projects).toEqual(sampleList.projects)
      expect(store.loading).toBe(false)
    })

    it('sets loading flag during fetch', async () => {
      let resolvePromise: (v: typeof sampleList) => void = () => {}
      mockListProjects.mockReturnValue(
        new Promise((resolve) => { resolvePromise = resolve }),
      )

      const store = useProjectsStore()
      const promise = store.load()

      expect(store.loading).toBe(true)
      resolvePromise(sampleList)
      await promise
      expect(store.loading).toBe(false)
    })
  })

  describe('create', () => {
    it('calls createProject and refreshes list', async () => {
      mockCreateProject.mockResolvedValue(sampleProject)
      mockListProjects.mockResolvedValue(sampleList)

      const store = useProjectsStore()
      const result = await store.create({ name: 'My Project', description: 'A test project' })

      expect(mockCreateProject).toHaveBeenCalledWith({ name: 'My Project', description: 'A test project' })
      expect(mockListProjects).toHaveBeenCalledOnce()
      expect(result).toEqual(sampleProject)
    })
  })

  describe('update', () => {
    it('calls updateProject and refreshes list', async () => {
      const updated = { ...sampleProject, name: 'Updated' }
      mockUpdateProject.mockResolvedValue(updated)
      mockListProjects.mockResolvedValue({ projects: [updated] })

      const store = useProjectsStore()
      const result = await store.update('p-1', { name: 'Updated' })

      expect(mockUpdateProject).toHaveBeenCalledWith('p-1', { name: 'Updated' })
      expect(mockListProjects).toHaveBeenCalledOnce()
      expect(result.name).toBe('Updated')
    })
  })

  describe('remove', () => {
    it('calls deleteProject (soft) and refreshes list', async () => {
      mockDeleteProject.mockResolvedValue(undefined)
      mockListProjects.mockResolvedValue({ projects: [] })

      const store = useProjectsStore()
      await store.remove('p-1')

      expect(mockDeleteProject).toHaveBeenCalledWith('p-1', false)
      expect(mockListProjects).toHaveBeenCalledOnce()
    })

    it('passes hardDelete flag', async () => {
      mockDeleteProject.mockResolvedValue(undefined)
      mockListProjects.mockResolvedValue({ projects: [] })

      const store = useProjectsStore()
      await store.remove('p-1', true)

      expect(mockDeleteProject).toHaveBeenCalledWith('p-1', true)
    })
  })
})
