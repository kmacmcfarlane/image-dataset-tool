import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  fetchSettingsInfo,
  listSecrets,
  setSecret,
  deleteSecret,
  testProvider,
} from '../api/settings'
import type { SettingsInfo, SecretEntry, TestProviderResult } from '../api/settings'

/**
 * Settings store.
 * Manages system info, secrets list, and provider test results.
 */
export const useSettingsStore = defineStore('settings', () => {
  const info = ref<SettingsInfo | null>(null)
  const secrets = ref<SecretEntry[]>([])
  const loading = ref(false)
  const secretsLoading = ref(false)
  const providerTestLoading = ref<Record<string, boolean>>({})
  const providerTestResult = ref<Record<string, TestProviderResult>>({})

  async function loadInfo(): Promise<void> {
    loading.value = true
    try {
      info.value = await fetchSettingsInfo()
    } finally {
      loading.value = false
    }
  }

  async function loadSecrets(): Promise<void> {
    secretsLoading.value = true
    try {
      const result = await listSecrets()
      secrets.value = result.secrets
    } finally {
      secretsLoading.value = false
    }
  }

  async function upsertSecret(key: string, value: string): Promise<void> {
    await setSecret(key, value)
    await loadSecrets()
  }

  async function removeSecret(key: string): Promise<void> {
    await deleteSecret(key)
    await loadSecrets()
  }

  async function runProviderTest(provider: string): Promise<void> {
    providerTestLoading.value = { ...providerTestLoading.value, [provider]: true }
    try {
      const result = await testProvider(provider)
      providerTestResult.value = { ...providerTestResult.value, [provider]: result }
    } finally {
      providerTestLoading.value = { ...providerTestLoading.value, [provider]: false }
    }
  }

  return {
    info,
    secrets,
    loading,
    secretsLoading,
    providerTestLoading,
    providerTestResult,
    loadInfo,
    loadSecrets,
    upsertSecret,
    removeSecret,
    runProviderTest,
  }
})
