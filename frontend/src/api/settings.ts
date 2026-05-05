import { apiFetch } from './client'

export interface SecretEntry {
  key: string
  created_at: string
  updated_at: string
}

export interface SecretListResult {
  secrets: SecretEntry[]
}

export interface SettingsInfo {
  data_dir: string
  key_status: 'found' | 'missing' | 'wrong_permissions'
  config: Record<string, unknown>
}

export interface TestProviderResult {
  success: boolean
  message: string
}

/** Fetch system info: data dir, encryption key status, config. */
export function fetchSettingsInfo(): Promise<SettingsInfo> {
  return apiFetch<SettingsInfo>('/v1/settings/info')
}

/** List stored secret keys (not values). */
export function listSecrets(): Promise<SecretListResult> {
  return apiFetch<SecretListResult>('/v1/settings/secrets')
}

/** Create or update a secret (plaintext value encrypted on the server). */
export function setSecret(key: string, value: string): Promise<void> {
  const encoded = encodeURIComponent(key)
  return apiFetch<void>(`/v1/settings/secrets/${encoded}`, {
    method: 'PUT',
    body: { value },
  })
}

/** Delete a secret by key. */
export function deleteSecret(key: string): Promise<void> {
  const encoded = encodeURIComponent(key)
  return apiFetch<void>(`/v1/settings/secrets/${encoded}`, {
    method: 'DELETE',
  })
}

/** Test a provider connection using the stored API key. */
export function testProvider(provider: string): Promise<TestProviderResult> {
  const encoded = encodeURIComponent(provider)
  return apiFetch<TestProviderResult>(`/v1/settings/providers/${encoded}/test`, {
    method: 'POST',
  })
}
