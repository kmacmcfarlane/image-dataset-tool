import { defineStore } from 'pinia'
import { ref } from 'vue'
import {
  listAccounts,
  createAccount,
  updateAccountCookies,
  deleteAccount,
} from '../api/accounts'
import type {
  AccountResult,
  CreateAccountPayload,
} from '../api/accounts'

/**
 * Accounts store.
 * Manages the account list and CRUD operations.
 */
export const useAccountsStore = defineStore('accounts', () => {
  const accounts = ref<AccountResult[]>([])
  const loading = ref(false)

  async function load(): Promise<void> {
    loading.value = true
    try {
      const result = await listAccounts()
      accounts.value = result.accounts
    } finally {
      loading.value = false
    }
  }

  async function create(payload: CreateAccountPayload): Promise<AccountResult> {
    const account = await createAccount(payload)
    await load()
    return account
  }

  async function setCookies(id: string, cookies: string): Promise<void> {
    await updateAccountCookies(id, cookies)
    await load()
  }

  async function remove(id: string): Promise<void> {
    await deleteAccount(id)
    await load()
  }

  return {
    accounts,
    loading,
    load,
    create,
    setCookies,
    remove,
  }
})
