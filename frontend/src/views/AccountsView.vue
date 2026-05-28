<template>
  <div
    class="accounts-view"
    data-testid="view-accounts"
  >
    <div class="view-header">
      <h2 class="view-title">
        Accounts
      </h2>
      <n-button
        type="primary"
        size="small"
        data-testid="create-account-btn"
        @click="openCreateDialog"
      >
        New Account
      </n-button>
    </div>

    <div
      v-if="store.loading"
      class="loading-text"
      data-testid="accounts-loading"
    >
      Loading...
    </div>

    <div
      v-else-if="store.accounts.length === 0"
      class="empty-state"
      data-testid="accounts-empty"
    >
      <p>No accounts yet. Add one to get started.</p>
    </div>

    <div
      v-else
      class="account-grid"
      data-testid="account-list"
    >
      <div
        v-for="account in store.accounts"
        :key="account.id"
        class="account-card"
        :data-testid="`account-card-${account.platform}-${account.handle}`"
      >
        <div class="account-card-header">
          <div class="account-info">
            <h3 class="account-handle">
              {{ account.handle }}
            </h3>
            <span class="account-platform">{{ account.platform }}</span>
          </div>
          <n-button
            quaternary
            size="tiny"
            class="menu-btn"
            :data-testid="`account-menu-${account.platform}-${account.handle}`"
            @click.stop="showContextMenu($event, account)"
          >
            ...
          </n-button>
        </div>
        <div class="account-meta">
          <span
            class="cookie-status"
            :data-testid="`account-cookies-${account.platform}-${account.handle}`"
          >
            {{ account.has_cookies ? 'Cookies set' : 'No cookies' }}
          </span>
          <span class="account-date">{{ formatDate(account.created_at) }}</span>
        </div>
      </div>
    </div>

    <!-- Context menu dropdown -->
    <n-dropdown
      :show="contextMenuVisible"
      :x="contextMenuX"
      :y="contextMenuY"
      :options="contextMenuOptions"
      placement="bottom-start"
      trigger="manual"
      data-testid="account-context-menu"
      @select="handleContextAction"
      @clickoutside="contextMenuVisible = false"
    />

    <!-- Create Account Dialog -->
    <n-modal
      v-model:show="showCreateDialog"
      preset="dialog"
      title="New Account"
      positive-text="Save"
      negative-text="Cancel"
      data-testid="account-dialog"
      @positive-click="handleSaveAccount"
      @negative-click="closeCreateDialog"
    >
      <div class="account-form">
        <div class="form-field">
          <label class="form-label">Platform</label>
          <n-select
            v-model:value="accountForm.platform"
            :options="platformOptions"
            placeholder="Select platform"
            data-testid="account-platform-input"
          />
        </div>
        <div class="form-field">
          <label class="form-label">Handle</label>
          <input
            v-model="accountForm.handle"
            class="form-input"
            placeholder="Account handle"
            data-testid="account-handle-input"
          >
        </div>
        <div class="form-field">
          <label class="form-label">Cookies (optional)</label>
          <p class="cookies-help">
            To extract cookies: (1) open DevTools (F12), (2) go to Network tab,
            (3) reload the page, (4) click any request to the site, (5) in the
            Headers pane find the Cookie request header, (6) click the value to
            select it, (7) copy and paste here.
          </p>
          <p
            v-if="accountForm.platform"
            class="cookies-example"
            data-testid="cookie-example"
          >
            Example: {{ cookieExampleForPlatform(accountForm.platform) }}
          </p>
          <textarea
            v-model="accountForm.cookies"
            class="form-textarea"
            placeholder="Paste cookies from browser devtools (encrypted at rest)"
            rows="4"
            data-testid="account-cookies-input"
          />
          <p
            v-if="createCookieError"
            class="form-error"
            data-testid="create-cookie-error"
          >
            {{ createCookieError }}
          </p>
        </div>
      </div>
    </n-modal>

    <!-- Update Cookies Dialog -->
    <n-modal
      v-model:show="showCookiesDialog"
      preset="dialog"
      title="Update Cookies"
      positive-text="Save"
      negative-text="Cancel"
      data-testid="cookies-dialog"
      @positive-click="handleSaveCookies"
      @negative-click="closeCookiesDialog"
    >
      <div class="account-form">
        <p class="cookies-info">
          Cookies for <strong>{{ cookiesAccount?.handle }}</strong> on {{ cookiesAccount?.platform }}.
          Cookies are encrypted with AES-256-GCM before storage.
        </p>
        <p class="cookies-help">
          To extract cookies: (1) open DevTools (F12), (2) go to Network tab,
          (3) reload the page, (4) click any request to the site, (5) in the
          Headers pane find the Cookie request header, (6) click the value to
          select it, (7) copy and paste here.
        </p>
        <p
          v-if="cookiesAccount?.platform"
          class="cookies-example"
          data-testid="update-cookie-example"
        >
          Example: {{ cookieExampleForPlatform(cookiesAccount.platform) }}
        </p>
        <div class="form-field">
          <label class="form-label">Cookies</label>
          <textarea
            v-model="cookiesForm.cookies"
            class="form-textarea"
            placeholder="Paste cookies from browser devtools"
            rows="6"
            data-testid="update-cookies-input"
          />
          <p
            v-if="updateCookieError"
            class="form-error"
            data-testid="update-cookie-error"
          >
            {{ updateCookieError }}
          </p>
        </div>
      </div>
    </n-modal>

    <!-- Delete Confirmation Dialog -->
    <n-modal
      v-model:show="showDeleteDialog"
      preset="dialog"
      type="warning"
      title="Delete Account"
      positive-text="Delete"
      negative-text="Cancel"
      data-testid="delete-account-dialog"
      @positive-click="handleConfirmDelete"
      @negative-click="showDeleteDialog = false"
    >
      <p>
        Are you sure you want to delete "{{ deletingAccount?.handle }}" ({{ deletingAccount?.platform }})?
        This will also remove all subject-account links.
      </p>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NButton, NModal, NDropdown, NSelect } from 'naive-ui'
import { useAccountsStore } from '../stores/accounts'
import { useToastStore } from '../stores/toast'
import type { AccountResult } from '../api/accounts'

const store = useAccountsStore()
const toastStore = useToastStore()

/** Supported platforms for the dropdown. */
const platformOptions = [
  { label: 'Instagram', value: 'instagram' },
]

/** Required cookie keys per platform. */
const platformRequiredCookieKeys: Record<string, string[]> = {
  instagram: ['sessionid', 'csrftoken', 'ds_user_id'],
}

/** Example cookie strings per platform. */
const platformCookieExamples: Record<string, string> = {
  instagram: 'ig_did=...; mid=...; sessionid=...; csrftoken=...; ds_user_id=...',
}

/** Returns a truncated example cookie string for a platform. */
function cookieExampleForPlatform(platform: string): string {
  return platformCookieExamples[platform] ?? ''
}

/** Parses cookie keys from a cookie string. */
function parseCookieKeys(cookies: string): Set<string> {
  const keys = new Set<string>()
  for (const pair of cookies.split(';')) {
    const trimmed = pair.trim()
    if (!trimmed) continue
    const eqIdx = trimmed.indexOf('=')
    if (eqIdx > 0) {
      keys.add(trimmed.substring(0, eqIdx).trim())
    }
  }
  return keys
}

/** Validates cookies against required keys for a platform. Returns error string or empty. */
function validateCookiesForPlatform(platform: string, cookies: string): string {
  if (!cookies.trim()) return ''
  const required = platformRequiredCookieKeys[platform]
  if (!required) return ''
  const present = parseCookieKeys(cookies)
  const missing = required.filter(k => !present.has(k))
  if (missing.length > 0) {
    return `Missing required cookie keys for ${platform}: ${missing.join(', ')}`
  }
  return ''
}

// Create dialog state
const showCreateDialog = ref(false)
const accountForm = ref({ platform: null as string | null, handle: '', cookies: '' })
const createCookieError = ref('')

// Cookies dialog state
const showCookiesDialog = ref(false)
const cookiesAccount = ref<AccountResult | null>(null)
const cookiesForm = ref({ cookies: '' })
const updateCookieError = ref('')

// Delete dialog state
const showDeleteDialog = ref(false)
const deletingAccount = ref<AccountResult | null>(null)

// Context menu state
const contextMenuVisible = ref(false)
const contextMenuX = ref(0)
const contextMenuY = ref(0)
const contextMenuAccount = ref<AccountResult | null>(null)

const contextMenuOptions = [
  { label: 'Update Cookies', key: 'cookies' },
  { label: 'Delete', key: 'delete' },
]

onMounted(async () => {
  await store.load()
})

function formatDate(dateStr: string): string {
  try {
    return new Date(dateStr).toLocaleDateString()
  } catch {
    return dateStr
  }
}

function showContextMenu(event: MouseEvent, account: AccountResult): void {
  contextMenuAccount.value = account
  contextMenuX.value = event.clientX
  contextMenuY.value = event.clientY
  contextMenuVisible.value = true
}

function handleContextAction(key: string): void {
  contextMenuVisible.value = false
  const account = contextMenuAccount.value
  if (!account) return

  if (key === 'cookies') {
    openCookiesDialog(account)
  } else if (key === 'delete') {
    openDeleteDialog(account)
  }
}

function openCreateDialog(): void {
  accountForm.value = { platform: null, handle: '', cookies: '' }
  createCookieError.value = ''
  showCreateDialog.value = true
}

function closeCreateDialog(): void {
  showCreateDialog.value = false
  accountForm.value = { platform: null, handle: '', cookies: '' }
  createCookieError.value = ''
}

async function handleSaveAccount(): Promise<boolean | void> {
  const { platform, handle, cookies } = accountForm.value
  createCookieError.value = ''

  if (!platform) {
    toastStore.addToast('warning', 'Platform is required')
    return false
  }
  if (!handle.trim()) {
    toastStore.addToast('warning', 'Handle is required')
    return false
  }

  // Validate cookie keys if cookies provided.
  if (cookies.trim()) {
    const err = validateCookiesForPlatform(platform, cookies)
    if (err) {
      createCookieError.value = err
      return false
    }
  }

  try {
    const account = await store.create({
      platform,
      handle: handle.trim(),
    })

    // If cookies were provided, set them.
    if (cookies.trim()) {
      await store.setCookies(account.id, cookies.trim())
    }

    toastStore.addToast('info', `Account "${handle}" created`)
    closeCreateDialog()
  } catch {
    // Error already handled by API error handler
  }
}

function openCookiesDialog(account: AccountResult): void {
  cookiesAccount.value = account
  cookiesForm.value = { cookies: '' }
  showCookiesDialog.value = true
}

function closeCookiesDialog(): void {
  showCookiesDialog.value = false
  cookiesAccount.value = null
  cookiesForm.value = { cookies: '' }
  updateCookieError.value = ''
}

async function handleSaveCookies(): Promise<boolean | void> {
  if (!cookiesAccount.value) return
  updateCookieError.value = ''
  const { cookies } = cookiesForm.value
  if (!cookies.trim()) {
    toastStore.addToast('warning', 'Cookies cannot be empty')
    return false
  }

  // Validate cookie keys.
  const err = validateCookiesForPlatform(cookiesAccount.value.platform, cookies)
  if (err) {
    updateCookieError.value = err
    return false
  }

  try {
    await store.setCookies(cookiesAccount.value.id, cookies.trim())
    toastStore.addToast('info', `Cookies updated for "${cookiesAccount.value.handle}"`)
    closeCookiesDialog()
  } catch {
    // Error already handled by API error handler
  }
}

function openDeleteDialog(account: AccountResult): void {
  deletingAccount.value = account
  showDeleteDialog.value = true
}

async function handleConfirmDelete(): Promise<void> {
  if (!deletingAccount.value) return

  try {
    await store.remove(deletingAccount.value.id)
    toastStore.addToast('info', `Account "${deletingAccount.value.handle}" deleted`)
  } catch {
    // Error already handled by API error handler
  }
  showDeleteDialog.value = false
  deletingAccount.value = null
}
</script>

<style scoped>
.accounts-view {
  max-width: 1000px;
}

.view-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
}

.view-title {
  color: var(--text-color);
}

.loading-text,
.empty-state {
  color: var(--text-secondary);
  font-size: 14px;
  padding: 20px 0;
}

.account-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
}

.account-card {
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 14px;
  background: var(--bg-secondary);
}

.account-card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.account-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.account-handle {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-color);
  margin: 0;
}

.account-platform {
  font-size: 12px;
  color: var(--text-secondary);
  text-transform: capitalize;
}

.menu-btn {
  flex-shrink: 0;
  font-weight: bold;
}

.account-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 10px;
  font-size: 12px;
  color: var(--text-secondary);
}

.cookie-status {
  font-family: monospace;
  background: var(--bg-color);
  padding: 1px 5px;
  border-radius: 3px;
}

/* Form styles */
.account-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 8px;
}

.form-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.form-label {
  font-size: 12px;
  color: var(--text-secondary);
}

.form-input,
.form-textarea {
  padding: 6px 10px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background: var(--bg-color);
  color: var(--text-color);
  font-size: 13px;
  font-family: inherit;
  outline: none;
}

.form-input:focus,
.form-textarea:focus {
  border-color: var(--accent-color);
}

.form-textarea {
  resize: vertical;
  min-height: 60px;
}

.cookies-info {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.4;
}

.cookies-help {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.5;
  margin: 0 0 4px 0;
}

.cookies-example {
  font-size: 12px;
  color: var(--text-secondary);
  font-family: monospace;
  background: var(--bg-color);
  padding: 4px 8px;
  border-radius: 3px;
  margin: 0 0 4px 0;
  word-break: break-all;
}

.form-error {
  font-size: 12px;
  color: var(--error-color, #e74c3c);
  margin: 4px 0 0 0;
}
</style>
