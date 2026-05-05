<template>
  <div
    class="settings-view"
    data-testid="view-settings"
  >
    <h2 class="settings-title">Settings</h2>

    <!-- System Info Section -->
    <section class="settings-section">
      <h3 class="section-title">System</h3>

      <div
        v-if="store.loading"
        class="loading-text"
        data-testid="info-loading"
      >
        Loading…
      </div>
      <template v-else-if="store.info">
        <div
          class="info-grid"
          data-testid="system-info"
        >
          <div class="info-row">
            <span class="info-label">Data Directory</span>
            <code
              class="info-value"
              data-testid="data-dir"
            >{{ store.info.data_dir }}</code>
          </div>
          <div class="info-row">
            <span class="info-label">Encryption Key</span>
            <span
              class="key-status"
              :class="`key-status--${store.info.key_status}`"
              :data-testid="`key-status-${store.info.key_status}`"
            >
              {{ keyStatusLabel(store.info.key_status) }}
            </span>
          </div>
        </div>
      </template>
    </section>

    <!-- Toast Notification Threshold -->
    <section class="settings-section">
      <h3 class="section-title">Notifications</h3>
      <div class="info-row">
        <span class="info-label">Minimum toast level</span>
        <div
          class="threshold-buttons"
          data-testid="threshold-buttons"
        >
          <button
            v-for="level in toastLevels"
            :key="level"
            class="threshold-btn"
            :class="{ 'threshold-btn--active': toastStore.threshold === level }"
            :data-testid="`threshold-btn-${level}`"
            @click="toastStore.setThreshold(level)"
          >
            {{ level }}
          </button>
        </div>
      </div>
    </section>

    <!-- Secrets Management Section -->
    <section class="settings-section">
      <div class="section-header">
        <h3 class="section-title">Secrets</h3>
        <n-button
          size="small"
          type="primary"
          data-testid="add-secret-btn"
          @click="openAddSecret"
        >
          Add Secret
        </n-button>
      </div>

      <n-data-table
        :columns="secretColumns"
        :data="store.secrets"
        :loading="store.secretsLoading"
        :bordered="true"
        size="small"
        :row-key="(row: SecretEntry) => row.key"
        data-testid="secrets-table"
      />
    </section>

    <!-- Provider Config Section -->
    <section
      v-if="store.info"
      class="settings-section"
    >
      <h3 class="section-title">Provider Configuration</h3>
      <p class="section-hint">
        Provider settings are read from <code>config.yaml</code> (read-only; requires restart to change).
      </p>

      <div
        v-if="hasProviderConfig"
        class="provider-list"
        data-testid="provider-list"
      >
        <div
          v-for="(providerCfg, providerName) in providersConfig"
          :key="providerName"
          class="provider-card"
          :data-testid="`provider-card-${providerName}`"
        >
          <div class="provider-header">
            <strong class="provider-name">{{ providerName }}</strong>
            <n-button
              size="tiny"
              :loading="store.providerTestLoading[String(providerName)]"
              :data-testid="`test-provider-btn-${providerName}`"
              @click="handleTestProvider(String(providerName))"
            >
              Test Connection
            </n-button>
          </div>
          <div
            v-if="store.providerTestResult[String(providerName)]"
            class="provider-test-result"
            :class="store.providerTestResult[String(providerName)].success ? 'test-success' : 'test-failure'"
            :data-testid="`provider-test-result-${providerName}`"
          >
            {{ store.providerTestResult[String(providerName)].message }}
          </div>
          <div class="provider-fields">
            <div
              v-for="(val, cfgKey) in (providerCfg as Record<string, unknown>)"
              :key="String(cfgKey)"
              class="config-field"
            >
              <span class="config-key">{{ cfgKey }}</span>
              <code class="config-value">{{ val }}</code>
            </div>
          </div>
        </div>
      </div>
      <div
        v-else
        class="empty-text"
        data-testid="no-providers"
      >
        No provider configuration found in config.yaml.
      </div>
    </section>

    <!-- All Config Section -->
    <section
      v-if="store.info && Object.keys(store.info.config).length > 0"
      class="settings-section"
    >
      <h3 class="section-title">Full Configuration</h3>
      <p class="section-hint">
        All values from <code>config.yaml</code> (read-only).
      </p>
      <div
        class="config-sections"
        data-testid="config-sections"
      >
        <div
          v-for="(sectionVal, sectionKey) in store.info.config"
          :key="String(sectionKey)"
          class="config-section"
          :data-testid="`config-section-${sectionKey}`"
        >
          <h4 class="config-section-title">{{ sectionKey }}</h4>
          <pre
            class="config-raw"
          >{{ formatConfigValue(sectionVal) }}</pre>
        </div>
      </div>
    </section>

    <!-- Add/Edit Secret Modal -->
    <n-modal
      v-model:show="showSecretModal"
      preset="dialog"
      :title="editingKey ? 'Update Secret' : 'Add Secret'"
      positive-text="Save"
      negative-text="Cancel"
      data-testid="secret-modal"
      @positive-click="handleSaveSecret"
      @negative-click="closeSecretModal"
    >
      <div class="secret-form">
        <div class="form-field">
          <label class="form-label">Key</label>
          <input
            v-model="secretForm.key"
            class="form-input"
            :disabled="!!editingKey"
            placeholder="e.g. anthropic_api_key"
            data-testid="secret-key-input"
          />
        </div>
        <div class="form-field">
          <label class="form-label">Value</label>
          <input
            v-model="secretForm.value"
            class="form-input"
            type="password"
            placeholder="Plaintext secret value"
            data-testid="secret-value-input"
          />
        </div>
      </div>
    </n-modal>

    <!-- Delete Secret Confirmation -->
    <n-modal
      v-model:show="showDeleteConfirm"
      preset="dialog"
      type="warning"
      title="Delete Secret"
      :content="`Delete secret '${deletingKey}'? This cannot be undone.`"
      positive-text="Delete"
      negative-text="Cancel"
      data-testid="delete-secret-modal"
      @positive-click="handleConfirmDelete"
      @negative-click="showDeleteConfirm = false"
    />
  </div>
</template>

<script setup lang="ts">
import { h, ref, computed, onMounted } from 'vue'
import { NButton, NDataTable, NModal } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useSettingsStore } from '../stores/settings'
import { useToastStore } from '../stores/toast'
import type { SecretEntry } from '../api/settings'
import type { ToastLevel } from '../stores/toast'

const store = useSettingsStore()
const toastStore = useToastStore()

const toastLevels: ToastLevel[] = ['info', 'warning', 'error']

// Modal state
const showSecretModal = ref(false)
const editingKey = ref<string | null>(null)
const secretForm = ref({ key: '', value: '' })

const showDeleteConfirm = ref(false)
const deletingKey = ref('')

onMounted(async () => {
  await Promise.all([store.loadInfo(), store.loadSecrets()])
})

function keyStatusLabel(status: string): string {
  switch (status) {
    case 'found': return 'Found (OK)'
    case 'missing': return 'Missing'
    case 'wrong_permissions': return 'Wrong permissions (need 0600)'
    default: return status
  }
}

const providersConfig = computed((): Record<string, unknown> => {
  const cfg = store.info?.config
  if (!cfg || typeof cfg.providers !== 'object' || cfg.providers === null) return {}
  return cfg.providers as Record<string, unknown>
})

const hasProviderConfig = computed(() => Object.keys(providersConfig.value).length > 0)

function formatConfigValue(val: unknown): string {
  if (typeof val === 'object' && val !== null) {
    return JSON.stringify(val, null, 2)
  }
  return String(val)
}

// Secrets table columns
const secretColumns: DataTableColumns<SecretEntry> = [
  { title: 'Key', key: 'key' },
  { title: 'Created', key: 'created_at', width: 200 },
  { title: 'Updated', key: 'updated_at', width: 200 },
  {
    title: 'Actions',
    key: 'actions',
    width: 160,
    render(row: SecretEntry) {
      return h('div', { class: 'row-actions' }, [
        h(
          NButton,
          {
            size: 'tiny',
            onClick: () => openEditSecret(row.key),
            'data-testid': `edit-secret-btn-${row.key}`,
          },
          { default: () => 'Update' },
        ),
        h(
          NButton,
          {
            size: 'tiny',
            type: 'error',
            onClick: () => openDeleteSecret(row.key),
            'data-testid': `delete-secret-btn-${row.key}`,
          },
          { default: () => 'Delete' },
        ),
      ])
    },
  },
]

function openAddSecret(): void {
  editingKey.value = null
  secretForm.value = { key: '', value: '' }
  showSecretModal.value = true
}

function openEditSecret(key: string): void {
  editingKey.value = key
  secretForm.value = { key, value: '' }
  showSecretModal.value = true
}

function closeSecretModal(): void {
  showSecretModal.value = false
  editingKey.value = null
  secretForm.value = { key: '', value: '' }
}

async function handleSaveSecret(): Promise<void> {
  const { key, value } = secretForm.value
  if (!key.trim()) {
    toastStore.addToast('warning', 'Secret key cannot be empty')
    return
  }
  if (!value.trim()) {
    toastStore.addToast('warning', 'Secret value cannot be empty')
    return
  }
  await store.upsertSecret(key.trim(), value)
  toastStore.addToast('info', `Secret '${key}' saved`)
  closeSecretModal()
}

function openDeleteSecret(key: string): void {
  deletingKey.value = key
  showDeleteConfirm.value = true
}

async function handleConfirmDelete(): Promise<void> {
  await store.removeSecret(deletingKey.value)
  toastStore.addToast('info', `Secret '${deletingKey.value}' deleted`)
  showDeleteConfirm.value = false
  deletingKey.value = ''
}

async function handleTestProvider(provider: string): Promise<void> {
  await store.runProviderTest(provider)
}
</script>

<style scoped>
.settings-view {
  max-width: 900px;
}

.settings-title {
  margin-bottom: 20px;
  color: var(--text-color);
}

.settings-section {
  margin-bottom: 28px;
  padding: 16px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--bg-secondary);
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-color);
  margin-bottom: 12px;
}

.section-header .section-title {
  margin-bottom: 0;
}

.section-hint {
  font-size: 12px;
  color: var(--text-secondary);
  margin-bottom: 12px;
}

.info-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.info-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.info-label {
  min-width: 160px;
  font-size: 13px;
  color: var(--text-secondary);
  flex-shrink: 0;
}

.info-value {
  font-size: 13px;
  background: var(--bg-color);
  padding: 2px 6px;
  border-radius: 3px;
  color: var(--text-color);
}

.key-status {
  font-size: 13px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 4px;
}

.key-status--found {
  background: #dcfce7;
  color: #15803d;
}

.key-status--missing {
  background: var(--error-bg);
  color: var(--error-text);
}

.key-status--wrong_permissions {
  background: var(--toast-warning-bg);
  color: var(--toast-warning-text);
}

.loading-text {
  font-size: 13px;
  color: var(--text-secondary);
}

.empty-text {
  font-size: 13px;
  color: var(--text-secondary);
}

/* Toast threshold buttons */
.threshold-buttons {
  display: flex;
  gap: 6px;
}

.threshold-btn {
  padding: 4px 12px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background: var(--bg-color);
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  text-transform: capitalize;
}

.threshold-btn--active {
  background: var(--accent-color);
  color: #fff;
  border-color: var(--accent-color);
}

/* Provider cards */
.provider-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.provider-card {
  border: 1px solid var(--border-color);
  border-radius: 4px;
  padding: 12px;
  background: var(--bg-color);
}

.provider-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.provider-name {
  font-size: 13px;
  color: var(--text-color);
  text-transform: capitalize;
}

.provider-test-result {
  font-size: 12px;
  padding: 4px 8px;
  border-radius: 3px;
  margin-bottom: 8px;
}

.test-success {
  background: #dcfce7;
  color: #15803d;
}

.test-failure {
  background: var(--error-bg);
  color: var(--error-text);
}

.provider-fields {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.config-field {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}

.config-key {
  min-width: 120px;
  color: var(--text-secondary);
  flex-shrink: 0;
}

.config-value {
  color: var(--text-color);
  background: var(--bg-secondary);
  padding: 1px 4px;
  border-radius: 2px;
}

/* Full config sections */
.config-sections {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.config-section {
  border: 1px solid var(--border-color);
  border-radius: 4px;
  overflow: hidden;
}

.config-section-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
  padding: 6px 10px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-color);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.config-raw {
  font-size: 12px;
  padding: 10px;
  background: var(--bg-color);
  color: var(--text-color);
  overflow-x: auto;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
}

/* Secret form */
.secret-form {
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

.form-input {
  padding: 6px 10px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background: var(--bg-color);
  color: var(--text-color);
  font-size: 13px;
  outline: none;
}

.form-input:focus {
  border-color: var(--accent-color);
}

.form-input:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

/* Table action buttons */
.row-actions {
  display: flex;
  gap: 4px;
}
</style>
