import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSubjectsStore } from '../subjects'

// Mock the API module.
vi.mock('../../api/subjects', () => ({
  listSubjects: vi.fn(),
  createSubject: vi.fn(),
  updateSubject: vi.fn(),
  deleteSubject: vi.fn(),
}))

import {
  listSubjects,
  createSubject,
  updateSubject,
  deleteSubject,
} from '../../api/subjects'

const mockListSubjects = vi.mocked(listSubjects)
const mockCreateSubject = vi.mocked(createSubject)
const mockUpdateSubject = vi.mocked(updateSubject)
const mockDeleteSubject = vi.mocked(deleteSubject)

const sampleSubject = {
  id: 's-1',
  project_id: 'p-1',
  slug: 'my-subject',
  name: 'My Subject',
  sample_count: 5,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const sampleList = { subjects: [sampleSubject] }

describe('useSubjectsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('load', () => {
    it('fetches and stores subject list', async () => {
      mockListSubjects.mockResolvedValue(sampleList)

      const store = useSubjectsStore()
      await store.load('p-1')

      expect(mockListSubjects).toHaveBeenCalledWith('p-1')
      expect(store.subjects).toEqual(sampleList.subjects)
      expect(store.loading).toBe(false)
      expect(store.currentProjectId).toBe('p-1')
    })

    it('sets loading flag during fetch', async () => {
      let resolvePromise: (v: typeof sampleList) => void = () => {}
      mockListSubjects.mockReturnValue(
        new Promise((resolve) => { resolvePromise = resolve }),
      )

      const store = useSubjectsStore()
      const promise = store.load('p-1')

      expect(store.loading).toBe(true)
      resolvePromise(sampleList)
      await promise
      expect(store.loading).toBe(false)
    })
  })

  describe('create', () => {
    it('calls createSubject and refreshes list', async () => {
      mockCreateSubject.mockResolvedValue(sampleSubject)
      mockListSubjects.mockResolvedValue(sampleList)

      const store = useSubjectsStore()
      const result = await store.create('p-1', { name: 'My Subject' })

      expect(mockCreateSubject).toHaveBeenCalledWith('p-1', { name: 'My Subject' })
      expect(mockListSubjects).toHaveBeenCalledWith('p-1')
      expect(result).toEqual(sampleSubject)
    })
  })

  describe('update', () => {
    it('calls updateSubject and refreshes list', async () => {
      const updated = { ...sampleSubject, name: 'Updated' }
      mockUpdateSubject.mockResolvedValue(updated)
      mockListSubjects.mockResolvedValue({ subjects: [updated] })

      const store = useSubjectsStore()
      const result = await store.update('p-1', 's-1', { name: 'Updated' })

      expect(mockUpdateSubject).toHaveBeenCalledWith('p-1', 's-1', { name: 'Updated' })
      expect(mockListSubjects).toHaveBeenCalledWith('p-1')
      expect(result.name).toBe('Updated')
    })
  })

  describe('remove', () => {
    it('calls deleteSubject (soft) and refreshes list', async () => {
      mockDeleteSubject.mockResolvedValue(undefined)
      mockListSubjects.mockResolvedValue({ subjects: [] })

      const store = useSubjectsStore()
      await store.remove('p-1', 's-1')

      expect(mockDeleteSubject).toHaveBeenCalledWith('p-1', 's-1', false)
      expect(mockListSubjects).toHaveBeenCalledWith('p-1')
    })

    it('passes hardDelete flag', async () => {
      mockDeleteSubject.mockResolvedValue(undefined)
      mockListSubjects.mockResolvedValue({ subjects: [] })

      const store = useSubjectsStore()
      await store.remove('p-1', 's-1', true)

      expect(mockDeleteSubject).toHaveBeenCalledWith('p-1', 's-1', true)
    })
  })
})
