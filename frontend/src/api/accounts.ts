import { apiFetch } from './client'

export interface AccountResult {
  id: string
  platform: string
  handle: string
  has_cookies: boolean
  last_login_at: string
  created_at: string
  updated_at: string
}

export interface AccountLinkedSubject {
  id: string
  project_id: string
  name: string
}

export interface AccountDetailResult extends Omit<AccountResult, 'last_login_at'> {
  last_login_at: string
  linked_subjects: AccountLinkedSubject[]
}

export interface AccountListResult {
  accounts: AccountResult[]
}

export interface CreateAccountPayload {
  platform: string
  handle: string
}

/** List all accounts. Never returns cookies. */
export function listAccounts(): Promise<AccountListResult> {
  return apiFetch<AccountListResult>('/v1/accounts')
}

/** Get an account by ID with linked subjects. Never returns cookies. */
export function getAccount(id: string): Promise<AccountDetailResult> {
  return apiFetch<AccountDetailResult>(
    `/v1/accounts/${encodeURIComponent(id)}`,
  )
}

/** Create a new account. */
export function createAccount(payload: CreateAccountPayload): Promise<AccountResult> {
  return apiFetch<AccountResult>('/v1/accounts', {
    method: 'POST',
    body: payload,
  })
}

/** Update cookies for an account (encrypts with AES-256-GCM on server). */
export function updateAccountCookies(id: string, cookies: string): Promise<void> {
  return apiFetch<void>(
    `/v1/accounts/${encodeURIComponent(id)}/cookies`,
    { method: 'PUT', body: { cookies } },
  )
}

/** Delete an account and all subject_accounts links. */
export function deleteAccount(id: string): Promise<void> {
  return apiFetch<void>(
    `/v1/accounts/${encodeURIComponent(id)}`,
    { method: 'DELETE' },
  )
}
