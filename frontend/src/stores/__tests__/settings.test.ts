import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useSettingsStore } from '../settings'

// Mock the API module.
vi.mock('../../api/settings', () => ({
  fetchSettingsInfo: vi.fn(),
  listSecrets: vi.fn(),
  setSecret: vi.fn(),
  deleteSecret: vi.fn(),
  testProvider: vi.fn(),
}))

import {
  fetchSettingsInfo,
  listSecrets,
  setSecret,
  deleteSecret,
  testProvider,
} from '../../api/settings'

const mockFetchSettingsInfo = vi.mocked(fetchSettingsInfo)
const mockListSecrets = vi.mocked(listSecrets)
const mockSetSecret = vi.mocked(setSecret)
const mockDeleteSecret = vi.mocked(deleteSecret)
const mockTestProvider = vi.mocked(testProvider)

const sampleInfo = {
  data_dir: '/data/image-dataset-tool',
  key_status: 'found' as const,
  config: { providers: { anthropic: { rpm: 50 } } },
}

const sampleSecrets = {
  secrets: [
    { key: 'anthropic_api_key', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z' },
    { key: 'openai_api_key', created_at: '2026-01-02T00:00:00Z', updated_at: '2026-01-02T00:00:00Z' },
  ],
}

describe('useSettingsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('loadInfo', () => {
    it('fetches and stores settings info', async () => {
      mockFetchSettingsInfo.mockResolvedValue(sampleInfo)

      const store = useSettingsStore()
      await store.loadInfo()

      expect(mockFetchSettingsInfo).toHaveBeenCalledOnce()
      expect(store.info).toEqual(sampleInfo)
      expect(store.loading).toBe(false)
    })

    it('sets loading flag during fetch', async () => {
      let resolvePromise: (v: typeof sampleInfo) => void = () => {}
      mockFetchSettingsInfo.mockReturnValue(
        new Promise((resolve) => { resolvePromise = resolve }),
      )

      const store = useSettingsStore()
      const promise = store.loadInfo()

      expect(store.loading).toBe(true)
      resolvePromise(sampleInfo)
      await promise
      expect(store.loading).toBe(false)
    })
  })

  describe('loadSecrets', () => {
    it('fetches and stores secret list', async () => {
      mockListSecrets.mockResolvedValue(sampleSecrets)

      const store = useSettingsStore()
      await store.loadSecrets()

      expect(mockListSecrets).toHaveBeenCalledOnce()
      expect(store.secrets).toEqual(sampleSecrets.secrets)
      expect(store.secretsLoading).toBe(false)
    })

    it('sets secretsLoading during fetch', async () => {
      let resolvePromise: (v: typeof sampleSecrets) => void = () => {}
      mockListSecrets.mockReturnValue(
        new Promise((resolve) => { resolvePromise = resolve }),
      )

      const store = useSettingsStore()
      const promise = store.loadSecrets()

      expect(store.secretsLoading).toBe(true)
      resolvePromise(sampleSecrets)
      await promise
      expect(store.secretsLoading).toBe(false)
    })
  })

  describe('upsertSecret', () => {
    it('calls setSecret and refreshes the list', async () => {
      mockSetSecret.mockResolvedValue(undefined)
      mockListSecrets.mockResolvedValue(sampleSecrets)

      const store = useSettingsStore()
      await store.upsertSecret('anthropic_api_key', 'sk-new-value')

      expect(mockSetSecret).toHaveBeenCalledWith('anthropic_api_key', 'sk-new-value')
      expect(mockListSecrets).toHaveBeenCalledOnce()
      expect(store.secrets).toEqual(sampleSecrets.secrets)
    })
  })

  describe('removeSecret', () => {
    it('calls deleteSecret and refreshes the list', async () => {
      mockDeleteSecret.mockResolvedValue(undefined)
      mockListSecrets.mockResolvedValue({ secrets: [] })

      const store = useSettingsStore()
      await store.removeSecret('anthropic_api_key')

      expect(mockDeleteSecret).toHaveBeenCalledWith('anthropic_api_key')
      expect(mockListSecrets).toHaveBeenCalledOnce()
      expect(store.secrets).toEqual([])
    })
  })

  describe('runProviderTest', () => {
    it('calls testProvider and stores result', async () => {
      const result = { success: true, message: 'Connection to anthropic successful (HTTP 200)' }
      mockTestProvider.mockResolvedValue(result)

      const store = useSettingsStore()
      await store.runProviderTest('anthropic')

      expect(mockTestProvider).toHaveBeenCalledWith('anthropic')
      expect(store.providerTestResult['anthropic']).toEqual(result)
      expect(store.providerTestLoading['anthropic']).toBe(false)
    })

    it('stores failure results', async () => {
      const result = { success: false, message: 'Authentication failed for openai (HTTP 401)' }
      mockTestProvider.mockResolvedValue(result)

      const store = useSettingsStore()
      await store.runProviderTest('openai')

      expect(store.providerTestResult['openai']).toEqual(result)
      expect(store.providerTestResult['openai'].success).toBe(false)
    })

    it('clears loading flag even on API error', async () => {
      mockTestProvider.mockRejectedValue(new Error('network error'))

      const store = useSettingsStore()
      await expect(store.runProviderTest('xai')).rejects.toThrow('network error')
      expect(store.providerTestLoading['xai']).toBe(false)
    })
  })
})
