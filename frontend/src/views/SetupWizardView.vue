<template>
  <div
    class="setup-wizard"
    data-testid="setup-wizard"
  >
    <div class="setup-card">
      <h1 class="setup-heading">Welcome — First-Run Setup</h1>
      <p class="setup-subheading">Complete the steps below to get started.</p>

      <!-- Step indicators -->
      <div
        class="step-indicators"
        data-testid="step-indicators"
      >
        <div
          v-for="(label, idx) in stepLabels"
          :key="idx"
          class="step-indicator"
          :class="{
            'step-indicator--active': currentStep === idx + 1,
            'step-indicator--done': currentStep > idx + 1,
          }"
          :data-testid="`step-indicator-${idx + 1}`"
        >
          <span class="step-dot">{{ idx + 1 }}</span>
          <span class="step-label">{{ label }}</span>
        </div>
      </div>

      <!-- Step 1: Data Directory -->
      <div
        v-if="currentStep === 1"
        class="step-content"
        data-testid="step-1"
      >
        <h2 class="step-title">Data Directory</h2>
        <p class="step-desc">
          All project data, images, and configuration will be stored here.
        </p>

        <div
          v-if="statusLoading"
          class="loading-text"
          data-testid="status-loading"
        >
          Loading…
        </div>
        <template v-else>
          <div class="info-row">
            <span class="info-label">Directory</span>
            <code
              class="info-value"
              data-testid="data-dir-path"
            >{{ setupStatus?.data_dir }}</code>
          </div>
          <div class="info-row">
            <span class="info-label">Writable</span>
            <span
              class="status-badge"
              :class="setupStatus?.data_dir_writable ? 'status-badge--ok' : 'status-badge--error'"
              data-testid="data-dir-writable"
            >
              {{ setupStatus?.data_dir_writable ? 'Yes' : 'No — check permissions' }}
            </span>
          </div>
          <p
            v-if="!setupStatus?.data_dir_writable"
            class="error-hint"
            data-testid="data-dir-error"
          >
            The directory is not writable. Ensure the DATA_DIR environment variable points to a
            writable path and restart the backend.
          </p>
        </template>

        <div class="step-actions">
          <n-button
            type="primary"
            :disabled="!setupStatus?.data_dir_writable"
            data-testid="step-1-next"
            @click="goToStep(2)"
          >
            Next
          </n-button>
        </div>
      </div>

      <!-- Step 2: Encryption Key -->
      <div
        v-if="currentStep === 2"
        class="step-content"
        data-testid="step-2"
      >
        <h2 class="step-title">Encryption Key</h2>
        <p class="step-desc">
          Secrets (API keys, credentials) are encrypted with a 32-byte AES-256 key stored at
          <code>$DATA_DIR/secret.key</code> (mode 0600).
        </p>

        <div class="info-row">
          <span class="info-label">Key status</span>
          <span
            class="status-badge"
            :class="keyStatusClass"
            data-testid="key-status-badge"
          >
            {{ keyStatusLabel }}
          </span>
        </div>

        <div
          v-if="keyGenError"
          class="error-hint"
          data-testid="key-gen-error"
        >
          {{ keyGenError }}
        </div>
        <div
          v-if="keyGenSuccess"
          class="success-hint"
          data-testid="key-gen-success"
        >
          {{ keyGenSuccess }}
        </div>

        <div class="step-actions">
          <n-button
            v-if="currentKeyStatus !== 'found'"
            type="primary"
            :loading="generatingKey"
            data-testid="generate-key-btn"
            @click="handleGenerateKey"
          >
            Generate Key
          </n-button>
          <n-button
            :disabled="currentKeyStatus !== 'found'"
            type="primary"
            data-testid="step-2-next"
            @click="goToStep(3)"
          >
            Next
          </n-button>
          <n-button
            data-testid="step-2-back"
            @click="goToStep(1)"
          >
            Back
          </n-button>
        </div>
      </div>

      <!-- Step 3: Provider Setup (optional) -->
      <div
        v-if="currentStep === 3"
        class="step-content"
        data-testid="step-3"
      >
        <h2 class="step-title">Provider Setup <span class="optional-tag">(optional)</span></h2>
        <p class="step-desc">
          Optionally add an API key for a captioning provider. You can skip this and add it
          later in Settings.
        </p>

        <div class="form-field">
          <label class="form-label">Provider</label>
          <n-select
            v-model:value="providerName"
            :options="providerOptions"
            placeholder="Select provider"
            data-testid="provider-select"
          />
        </div>
        <div class="form-field">
          <label class="form-label">API Key</label>
          <input
            v-model="providerApiKey"
            type="password"
            class="form-input"
            placeholder="Paste API key here"
            data-testid="provider-api-key-input"
          />
        </div>

        <div
          v-if="providerSaveError"
          class="error-hint"
          data-testid="provider-save-error"
        >
          {{ providerSaveError }}
        </div>

        <div class="step-actions">
          <n-button
            type="primary"
            :loading="savingProvider"
            :disabled="!providerName || !providerApiKey.trim()"
            data-testid="save-provider-btn"
            @click="handleSaveProvider"
          >
            Save &amp; Next
          </n-button>
          <n-button
            data-testid="skip-provider-btn"
            @click="goToStep(4)"
          >
            Skip
          </n-button>
          <n-button
            data-testid="step-3-back"
            @click="goToStep(2)"
          >
            Back
          </n-button>
        </div>
      </div>

      <!-- Step 4: Complete -->
      <div
        v-if="currentStep === 4"
        class="step-content"
        data-testid="step-4"
      >
        <h2 class="step-title">Setup Complete</h2>
        <p class="step-desc">
          Your data directory is ready, encryption is configured, and the database has been
          initialised. You can now start creating projects.
        </p>

        <div class="checklist">
          <div class="checklist-item">
            <span class="checklist-icon checklist-icon--ok">✓</span>
            Data directory: <code>{{ setupStatus?.data_dir }}</code>
          </div>
          <div class="checklist-item">
            <span class="checklist-icon checklist-icon--ok">✓</span>
            Encryption key present
          </div>
          <div
            v-if="providerSaved"
            class="checklist-item"
            data-testid="provider-saved-indicator"
          >
            <span class="checklist-icon checklist-icon--ok">✓</span>
            Provider API key saved
          </div>
        </div>

        <div class="step-actions">
          <n-button
            type="primary"
            data-testid="complete-setup-btn"
            @click="completeSetup"
          >
            Go to Projects
          </n-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { NButton, NSelect } from 'naive-ui'
import { fetchSetupStatus, generateEncryptionKey } from '../api/setup'
import { setSecret } from '../api/settings'
import type { SetupStatus } from '../api/setup'
import { SETUP_COMPLETE_KEY } from '../constants'

const router = useRouter()

const currentStep = ref(1)
const statusLoading = ref(true)
const setupStatus = ref<SetupStatus | null>(null)

// Step 2 key generation
const generatingKey = ref(false)
const keyGenError = ref('')
const keyGenSuccess = ref('')
const currentKeyStatus = ref<'found' | 'missing' | 'wrong_permissions'>('missing')

// Step 3 provider
const providerName = ref<string | null>(null)
const providerApiKey = ref('')
const savingProvider = ref(false)
const providerSaveError = ref('')
const providerSaved = ref(false)

const stepLabels = ['Data Directory', 'Encryption Key', 'Provider (optional)', 'Complete']

const providerOptions = [
  { label: 'Anthropic', value: 'anthropic' },
  { label: 'OpenAI', value: 'openai' },
  { label: 'xAI', value: 'xai' },
  { label: 'Gemini', value: 'gemini' },
]

const keyStatusLabel = computed(() => {
  switch (currentKeyStatus.value) {
    case 'found': return 'Key found'
    case 'missing': return 'Missing — generate a new key'
    case 'wrong_permissions': return 'Wrong permissions (need 0600)'
    default: return currentKeyStatus.value
  }
})

const keyStatusClass = computed(() => {
  switch (currentKeyStatus.value) {
    case 'found': return 'status-badge--ok'
    case 'missing': return 'status-badge--error'
    case 'wrong_permissions': return 'status-badge--warn'
    default: return ''
  }
})

onMounted(async () => {
  try {
    setupStatus.value = await fetchSetupStatus()
    currentKeyStatus.value = setupStatus.value.key_status
  } catch {
    // Non-fatal; proceed with wizard even if status call fails
  } finally {
    statusLoading.value = false
  }
})

function goToStep(step: number): void {
  currentStep.value = step
}

async function handleGenerateKey(): Promise<void> {
  keyGenError.value = ''
  keyGenSuccess.value = ''
  generatingKey.value = true
  try {
    const result = await generateEncryptionKey()
    if (result.ok) {
      currentKeyStatus.value = 'found'
      keyGenSuccess.value = result.message
    } else {
      keyGenError.value = result.message
    }
  } catch (err: unknown) {
    keyGenError.value = err instanceof Error ? err.message : 'Failed to generate key'
  } finally {
    generatingKey.value = false
  }
}

async function handleSaveProvider(): Promise<void> {
  if (!providerName.value || !providerApiKey.value.trim()) return
  savingProvider.value = true
  providerSaveError.value = ''
  const secretKey = `${providerName.value}_api_key`
  try {
    await setSecret(secretKey, providerApiKey.value.trim())
    providerSaved.value = true
    goToStep(4)
  } catch (err: unknown) {
    providerSaveError.value = err instanceof Error ? err.message : 'Failed to save API key'
  } finally {
    savingProvider.value = false
  }
}

function completeSetup(): void {
  localStorage.setItem(SETUP_COMPLETE_KEY, 'true')
  router.push('/')
}
</script>

<style scoped>
.setup-wizard {
  display: flex;
  align-items: flex-start;
  justify-content: center;
  min-height: 100vh;
  padding: 48px 16px;
  background: var(--bg-color);
}

.setup-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 36px 40px;
  width: 100%;
  max-width: 600px;
}

.setup-heading {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-color);
  margin-bottom: 6px;
}

.setup-subheading {
  font-size: 14px;
  color: var(--text-secondary);
  margin-bottom: 28px;
}

/* Step indicators */
.step-indicators {
  display: flex;
  gap: 8px;
  margin-bottom: 28px;
  flex-wrap: wrap;
}

.step-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-secondary);
  opacity: 0.5;
}

.step-indicator--active {
  opacity: 1;
  color: var(--accent-color);
  font-weight: 600;
}

.step-indicator--done {
  opacity: 0.8;
  color: var(--status-connected);
}

.step-dot {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  border: 1.5px solid currentColor;
  font-size: 11px;
  font-weight: 700;
  flex-shrink: 0;
}

/* Step content */
.step-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-color);
  margin-bottom: 8px;
}

.step-desc {
  font-size: 13px;
  color: var(--text-secondary);
  margin-bottom: 20px;
  line-height: 1.5;
}

.optional-tag {
  font-size: 12px;
  font-weight: 400;
  color: var(--text-secondary);
}

.info-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 10px;
}

.info-label {
  min-width: 120px;
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
  word-break: break-all;
}

.status-badge {
  font-size: 12px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 4px;
}

.status-badge--ok {
  background: var(--success-bg);
  color: var(--success-text);
}

.status-badge--error {
  background: var(--error-bg);
  color: var(--error-text);
}

.status-badge--warn {
  background: var(--toast-warning-bg);
  color: var(--toast-warning-text);
}

.error-hint {
  font-size: 12px;
  color: var(--error-text);
  background: var(--error-bg);
  border: 1px solid var(--error-border);
  padding: 8px 10px;
  border-radius: 4px;
  margin: 10px 0;
}

.success-hint {
  font-size: 12px;
  color: var(--success-text);
  background: var(--success-bg);
  padding: 8px 10px;
  border-radius: 4px;
  margin: 10px 0;
}

.loading-text {
  font-size: 13px;
  color: var(--text-secondary);
  margin-bottom: 16px;
}

.step-actions {
  display: flex;
  gap: 8px;
  margin-top: 24px;
  flex-wrap: wrap;
}

/* Form fields */
.form-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 14px;
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

/* Checklist */
.checklist {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 20px;
}

.checklist-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--text-color);
}

.checklist-icon {
  font-weight: 700;
  font-size: 14px;
}

.checklist-icon--ok {
  color: var(--status-connected);
}
</style>
