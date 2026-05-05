import { apiFetch } from './client'

export interface ConsumerStats {
  name: string
  subject: string
  pending: number
  ack_pending: number
  redelivered: number
  waiting: number
}

export interface QueueStatsResult {
  consumers: ConsumerStats[]
}

export interface QueueMessage {
  sequence: number
  subject: string
  data: string
  headers?: Record<string, string>
  timestamp: string
}

export interface PeekResult {
  messages: QueueMessage[]
  total: number
}

/** Fetch per-consumer queue statistics. */
export function fetchQueueStats(): Promise<QueueStatsResult> {
  return apiFetch<QueueStatsResult>('/v1/queues/stats')
}

/** Peek at messages in a queue subject without consuming them. */
export function peekMessages(
  subject: string,
  offset = 0,
  limit = 20,
): Promise<PeekResult> {
  const encoded = encodeURIComponent(subject)
  return apiFetch<PeekResult>(
    `/v1/queues/${encoded}/messages?offset=${offset}&limit=${limit}`,
  )
}

/** Retry (redeliver) a specific message to its original subject. */
export function retryMessage(subject: string, sequence: number): Promise<void> {
  const encoded = encodeURIComponent(subject)
  return apiFetch<void>(`/v1/queues/${encoded}/retry`, {
    method: 'POST',
    body: { sequence },
  })
}

/** Delete a specific message from a queue. */
export function deleteMessage(subject: string, sequence: number): Promise<void> {
  const encoded = encodeURIComponent(subject)
  return apiFetch<void>(`/v1/queues/${encoded}/messages/${sequence}`, {
    method: 'DELETE',
  })
}

/** Purge all messages from a queue subject. */
export function purgeSubject(subject: string): Promise<void> {
  const encoded = encodeURIComponent(subject)
  return apiFetch<void>(`/v1/queues/${encoded}/purge`, {
    method: 'POST',
  })
}
