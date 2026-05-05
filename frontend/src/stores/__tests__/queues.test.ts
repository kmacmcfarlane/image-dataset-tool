import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useQueuesStore } from '../queues'

// Mock the API module.
vi.mock('../../api/queues', () => ({
  fetchQueueStats: vi.fn(),
  peekMessages: vi.fn(),
  retryMessage: vi.fn(),
  deleteMessage: vi.fn(),
  purgeSubject: vi.fn(),
}))

import {
  fetchQueueStats,
  peekMessages,
  retryMessage,
  deleteMessage,
  purgeSubject,
} from '../../api/queues'

const mockFetchQueueStats = vi.mocked(fetchQueueStats)
const mockPeekMessages = vi.mocked(peekMessages)
const mockRetryMessage = vi.mocked(retryMessage)
const mockDeleteMessage = vi.mocked(deleteMessage)
const mockPurgeSubject = vi.mocked(purgeSubject)

describe('useQueuesStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('loadStats', () => {
    it('fetches and stores consumer statistics', async () => {
      const mockData = {
        consumers: [
          { name: 'media-dlq', subject: 'media.dlq', pending: 5, ack_pending: 0, redelivered: 2, waiting: 0 },
        ],
      }
      mockFetchQueueStats.mockResolvedValue(mockData)

      const store = useQueuesStore()
      await store.loadStats()

      expect(mockFetchQueueStats).toHaveBeenCalledOnce()
      expect(store.consumers).toEqual(mockData.consumers)
      expect(store.statsLoading).toBe(false)
    })

    it('sets statsLoading during fetch', async () => {
      let resolvePromise: (v: unknown) => void = () => {}
      mockFetchQueueStats.mockReturnValue(
        new Promise((resolve) => { resolvePromise = resolve as (v: unknown) => void }),
      )

      const store = useQueuesStore()
      const promise = store.loadStats()

      expect(store.statsLoading).toBe(true)
      resolvePromise({ consumers: [] })
      await promise
      expect(store.statsLoading).toBe(false)
    })
  })

  describe('loadMessages', () => {
    it('fetches messages for a subject', async () => {
      const mockData = {
        messages: [
          { sequence: 1, subject: 'media.dlq', data: '{"error":"test"}', timestamp: '2026-01-01T00:00:00Z' },
        ],
        total: 1,
      }
      mockPeekMessages.mockResolvedValue(mockData)

      const store = useQueuesStore()
      await store.loadMessages('media.dlq', 0)

      expect(mockPeekMessages).toHaveBeenCalledWith('media.dlq', 0, 20)
      expect(store.messages).toEqual(mockData.messages)
      expect(store.totalMessages).toBe(1)
      expect(store.selectedSubject).toBe('media.dlq')
    })
  })

  describe('retry', () => {
    it('retries message and refreshes data', async () => {
      mockRetryMessage.mockResolvedValue(undefined)
      mockPeekMessages.mockResolvedValue({ messages: [], total: 0 })
      mockFetchQueueStats.mockResolvedValue({ consumers: [] })

      const store = useQueuesStore()
      store.selectedSubject = 'media.dlq'
      await store.retry('media.dlq', 42)

      expect(mockRetryMessage).toHaveBeenCalledWith('media.dlq', 42)
      expect(mockPeekMessages).toHaveBeenCalled()
      expect(mockFetchQueueStats).toHaveBeenCalled()
    })
  })

  describe('remove', () => {
    it('deletes message and refreshes data', async () => {
      mockDeleteMessage.mockResolvedValue(undefined)
      mockPeekMessages.mockResolvedValue({ messages: [], total: 0 })
      mockFetchQueueStats.mockResolvedValue({ consumers: [] })

      const store = useQueuesStore()
      store.selectedSubject = 'media.dlq'
      await store.remove('media.dlq', 7)

      expect(mockDeleteMessage).toHaveBeenCalledWith('media.dlq', 7)
      expect(mockPeekMessages).toHaveBeenCalled()
      expect(mockFetchQueueStats).toHaveBeenCalled()
    })
  })

  describe('purge', () => {
    it('purges subject and clears messages', async () => {
      mockPurgeSubject.mockResolvedValue(undefined)
      mockFetchQueueStats.mockResolvedValue({ consumers: [] })

      const store = useQueuesStore()
      store.messages = [{ sequence: 1, subject: 'media.dlq', data: 'x', timestamp: '' }]
      store.totalMessages = 1

      await store.purge('media.dlq')

      expect(mockPurgeSubject).toHaveBeenCalledWith('media.dlq')
      expect(store.messages).toEqual([])
      expect(store.totalMessages).toBe(0)
      expect(mockFetchQueueStats).toHaveBeenCalled()
    })
  })
})
