import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useJobsStore } from '../jobs'

vi.mock('../../api/jobs', () => ({
  listJobs: vi.fn(),
  getJob: vi.fn(),
  listJobMessages: vi.fn(),
}))

import { listJobs, getJob, listJobMessages } from '../../api/jobs'

const mockListJobs = vi.mocked(listJobs)
const mockGetJob = vi.mocked(getJob)
const mockListJobMessages = vi.mocked(listJobMessages)

const sampleJob = {
  id: 'job-1',
  type: 'sample_collection',
  status: 'succeeded',
  completed_items: 10,
  failed_items: 1,
  started_at: '2026-01-01T00:00:00Z',
  created_at: '2026-01-01T00:00:00Z',
}

const sampleMessage = {
  id: 'msg-1',
  job_run_id: 'job-1',
  level: 'warning',
  message: 'Rate limit hit',
  created_at: '2026-01-01T00:01:00Z',
}

describe('useJobsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('load', () => {
    it('fetches and stores job list', async () => {
      mockListJobs.mockResolvedValue({ jobs: [sampleJob], total: 1 })

      const store = useJobsStore()
      await store.load()

      expect(mockListJobs).toHaveBeenCalledOnce()
      expect(store.jobs).toEqual([sampleJob])
      expect(store.total).toBe(1)
      expect(store.loading).toBe(false)
    })

    it('sets loading flag during fetch', async () => {
      let resolve: (v: { jobs: typeof sampleJob[]; total: number }) => void = () => {}
      mockListJobs.mockReturnValue(new Promise((r) => { resolve = r }))

      const store = useJobsStore()
      const pending = store.load()
      expect(store.loading).toBe(true)

      resolve({ jobs: [], total: 0 })
      await pending
      expect(store.loading).toBe(false)
    })
  })

  describe('selectJob / deselectJob', () => {
    it('loads messages when job is selected', async () => {
      mockListJobMessages.mockResolvedValue({ messages: [sampleMessage], total: 1 })

      const store = useJobsStore()
      await store.selectJob('job-1')

      expect(store.selectedJobId).toBe('job-1')
      expect(store.messages).toEqual([sampleMessage])
      expect(store.messagesTotal).toBe(1)
    })

    it('clears state on deselect', async () => {
      mockListJobMessages.mockResolvedValue({ messages: [sampleMessage], total: 1 })

      const store = useJobsStore()
      await store.selectJob('job-1')
      store.deselectJob()

      expect(store.selectedJobId).toBeNull()
      expect(store.messages).toEqual([])
    })
  })

  describe('setMessageLevel', () => {
    it('reloads messages with new level when job is selected', async () => {
      mockListJobMessages.mockResolvedValue({ messages: [], total: 0 })

      const store = useJobsStore()
      store.selectedJobId = 'job-1'
      await store.setMessageLevel('error')

      expect(store.messageLevel).toBe('error')
      expect(mockListJobMessages).toHaveBeenCalledWith('job-1', expect.objectContaining({ level: 'error' }))
    })
  })

  describe('applyProgressEvent', () => {
    it('updates liveProgress map', () => {
      const store = useJobsStore()
      store.applyProgressEvent({ id: 'job-1', current: 5, total: 10, pct: 50 })

      expect(store.liveProgress['job-1']).toEqual({ current: 5, total: 10, pct: 50 })
    })
  })

  describe('applyStateEvent', () => {
    it('updates liveStatus and fetches job detail', async () => {
      mockGetJob.mockResolvedValue({ ...sampleJob, status: 'failed' })
      mockListJobs.mockResolvedValue({ jobs: [sampleJob], total: 1 })

      const store = useJobsStore()
      await store.load()
      await store.applyStateEvent({ id: 'job-1', type: 'sample_collection', status: 'failed' })

      expect(store.liveStatus['job-1']).toBe('failed')
      expect(store.jobs[0].status).toBe('failed')
    })

    it('prepends new job if not in list', async () => {
      mockGetJob.mockResolvedValue({ ...sampleJob, id: 'job-new', status: 'running' })
      mockListJobs.mockResolvedValue({ jobs: [sampleJob], total: 1 })

      const store = useJobsStore()
      await store.load()
      await store.applyStateEvent({ id: 'job-new', type: 'sample_collection', status: 'running' })

      expect(store.jobs).toHaveLength(2)
      expect(store.jobs[0].id).toBe('job-new')
    })
  })
})
