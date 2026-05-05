import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { ConsumerStats, QueueMessage } from '../api/queues'
import {
  fetchQueueStats,
  peekMessages,
  retryMessage,
  deleteMessage,
  purgeSubject,
} from '../api/queues'

/**
 * Queue admin store.
 * Manages consumer stats polling and message operations (peek, retry, delete, purge).
 */
export const useQueuesStore = defineStore('queues', () => {
  const consumers = ref<ConsumerStats[]>([])
  const messages = ref<QueueMessage[]>([])
  const totalMessages = ref(0)
  const loading = ref(false)
  const statsLoading = ref(false)
  const selectedSubject = ref<string | null>(null)
  const currentOffset = ref(0)
  const pageSize = ref(20)

  async function loadStats(): Promise<void> {
    statsLoading.value = true
    try {
      const result = await fetchQueueStats()
      consumers.value = result.consumers
    } finally {
      statsLoading.value = false
    }
  }

  async function loadMessages(subject: string, offset = 0): Promise<void> {
    loading.value = true
    selectedSubject.value = subject
    currentOffset.value = offset
    try {
      const result = await peekMessages(subject, offset, pageSize.value)
      messages.value = result.messages
      totalMessages.value = result.total
    } finally {
      loading.value = false
    }
  }

  async function retry(subject: string, sequence: number): Promise<void> {
    await retryMessage(subject, sequence)
    // Refresh messages and stats after retry.
    if (selectedSubject.value) {
      await loadMessages(selectedSubject.value, currentOffset.value)
    }
    await loadStats()
  }

  async function remove(subject: string, sequence: number): Promise<void> {
    await deleteMessage(subject, sequence)
    // Refresh messages after delete.
    if (selectedSubject.value) {
      await loadMessages(selectedSubject.value, currentOffset.value)
    }
    await loadStats()
  }

  async function purge(subject: string): Promise<void> {
    await purgeSubject(subject)
    // Clear messages and refresh stats.
    messages.value = []
    totalMessages.value = 0
    await loadStats()
  }

  return {
    consumers,
    messages,
    totalMessages,
    loading,
    statsLoading,
    selectedSubject,
    currentOffset,
    pageSize,
    loadStats,
    loadMessages,
    retry,
    remove,
    purge,
  }
})
