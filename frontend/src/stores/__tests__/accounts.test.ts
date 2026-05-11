import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAccountsStore } from '../accounts'

// Mock the API module.
vi.mock('../../api/accounts', () => ({
  listAccounts: vi.fn(),
  createAccount: vi.fn(),
  updateAccountCookies: vi.fn(),
  deleteAccount: vi.fn(),
}))

import {
  listAccounts,
  createAccount,
  updateAccountCookies,
  deleteAccount,
} from '../../api/accounts'

const mockListAccounts = vi.mocked(listAccounts)
const mockCreateAccount = vi.mocked(createAccount)
const mockUpdateAccountCookies = vi.mocked(updateAccountCookies)
const mockDeleteAccount = vi.mocked(deleteAccount)

const sampleAccount = {
  id: 'a-1',
  platform: 'instagram',
  handle: 'testuser',
  has_cookies: false,
  last_login_at: '',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const sampleList = { accounts: [sampleAccount] }

describe('useAccountsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('load', () => {
    it('fetches and stores account list', async () => {
      mockListAccounts.mockResolvedValue(sampleList)

      const store = useAccountsStore()
      await store.load()

      expect(mockListAccounts).toHaveBeenCalled()
      expect(store.accounts).toEqual(sampleList.accounts)
      expect(store.loading).toBe(false)
    })

    it('sets loading flag during fetch', async () => {
      let resolvePromise: (v: typeof sampleList) => void = () => {}
      mockListAccounts.mockReturnValue(
        new Promise((resolve) => { resolvePromise = resolve }),
      )

      const store = useAccountsStore()
      const promise = store.load()

      expect(store.loading).toBe(true)
      resolvePromise(sampleList)
      await promise
      expect(store.loading).toBe(false)
    })
  })

  describe('create', () => {
    it('calls createAccount and refreshes list', async () => {
      mockCreateAccount.mockResolvedValue(sampleAccount)
      mockListAccounts.mockResolvedValue(sampleList)

      const store = useAccountsStore()
      const result = await store.create({
        platform: 'instagram',
        handle: 'testuser',
      })

      expect(mockCreateAccount).toHaveBeenCalledWith({
        platform: 'instagram',
        handle: 'testuser',
      })
      expect(mockListAccounts).toHaveBeenCalled()
      expect(result).toEqual(sampleAccount)
    })
  })

  describe('setCookies', () => {
    it('calls updateAccountCookies and refreshes list', async () => {
      mockUpdateAccountCookies.mockResolvedValue(undefined)
      mockListAccounts.mockResolvedValue({
        accounts: [{ ...sampleAccount, has_cookies: true }],
      })

      const store = useAccountsStore()
      await store.setCookies('a-1', 'session=abc')

      expect(mockUpdateAccountCookies).toHaveBeenCalledWith('a-1', 'session=abc')
      expect(mockListAccounts).toHaveBeenCalled()
    })
  })

  describe('remove', () => {
    it('calls deleteAccount and refreshes list', async () => {
      mockDeleteAccount.mockResolvedValue(undefined)
      mockListAccounts.mockResolvedValue({ accounts: [] })

      const store = useAccountsStore()
      await store.remove('a-1')

      expect(mockDeleteAccount).toHaveBeenCalledWith('a-1')
      expect(mockListAccounts).toHaveBeenCalled()
    })
  })
})
