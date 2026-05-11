import { apiFetch } from './client'

export interface SetupStatus {
  needs_setup: boolean
  data_dir: string
  data_dir_writable: boolean
  key_status: 'found' | 'missing' | 'wrong_permissions'
}

export interface GenerateKeyResult {
  ok: boolean
  message: string
}

/** Fetch current setup status from the backend. */
export function fetchSetupStatus(): Promise<SetupStatus> {
  return apiFetch<SetupStatus>('/api/setup/status')
}

/** Generate a new encryption key at $DATA_DIR/secret.key. */
export function generateEncryptionKey(): Promise<GenerateKeyResult> {
  return apiFetch<GenerateKeyResult>('/api/setup/generate-key', { method: 'POST' })
}
