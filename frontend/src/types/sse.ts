/** SSE event types matching PRD section 5.5 */

export interface JobStateEvent {
  id: string
  trace_id: string
  type: string
  status: string
  error?: string
}

export interface JobProgressEvent {
  id: string
  trace_id: string
  current: number
  total: number
  pct: number
}

export interface ConsumerStatsEvent {
  subject: string
  pending: number
  ack_pending: number
  redelivered: number
}

export interface ProviderRateEvent {
  provider: string
  rpm_used: number
  rpm_limit: number
}

export interface SampleNewEvent {
  id: string
  subject_id: string
  status: string
  is_duplicate: boolean
  thumbnail_path: string
}

export interface SampleUpdatedEvent {
  id: string
  status: string
  edits_changed: boolean
  caption_changed: boolean
}

export type SSEEventType =
  | 'job.state'
  | 'job.progress'
  | 'consumer.stats'
  | 'provider.rate'
  | 'sample.new'
  | 'sample.updated'

export type SSEEventPayload =
  | JobStateEvent
  | JobProgressEvent
  | ConsumerStatsEvent
  | ProviderRateEvent
  | SampleNewEvent
  | SampleUpdatedEvent

export type SSEConnectionStatus = 'connecting' | 'connected' | 'disconnected'
