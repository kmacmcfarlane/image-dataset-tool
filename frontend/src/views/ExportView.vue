<template>
  <div
    class="export-view"
    data-testid="view-export"
  >
    <div class="view-header">
      <h2 class="view-title">
        Export
      </h2>
    </div>

    <div
      v-if="loadingProjects"
      class="loading-text"
      data-testid="export-loading"
    >
      Loading projects...
    </div>

    <div
      v-else
      class="export-form"
      data-testid="export-form"
    >
      <!-- Project selector -->
      <div class="form-group">
        <label class="form-label">Project</label>
        <n-select
          v-model:value="selectedProjectId"
          :options="projectOptions"
          placeholder="Select a project"
          data-testid="export-project-select"
          @update:value="onProjectChange"
        />
      </div>

      <!-- Subject selector (multi) -->
      <div class="form-group">
        <label class="form-label">Subjects</label>
        <n-select
          v-model:value="selectedSubjectIds"
          :options="subjectOptions"
          multiple
          placeholder="Select subjects to export"
          :disabled="!selectedProjectId"
          data-testid="export-subject-select"
        />
      </div>

      <!-- Format selector -->
      <div class="form-group">
        <label class="form-label">Format</label>
        <n-select
          v-model:value="selectedFormat"
          :options="formatOptions"
          data-testid="export-format-select"
        />
      </div>

      <!-- Action buttons -->
      <div class="export-actions">
        <n-button
          type="primary"
          :disabled="selectedSubjectIds.length === 0 || serverExporting"
          :loading="serverExporting"
          data-testid="export-server-btn"
          @click="triggerServerExport"
        >
          Export to Server
        </n-button>

        <n-button
          :disabled="selectedSubjectIds.length === 0 || downloading"
          :loading="downloading"
          data-testid="export-zip-download-btn"
          @click="triggerZipDownload"
        >
          Download Zip
        </n-button>
      </div>

      <!-- Status messages -->
      <div
        v-if="statusMessage"
        class="export-status"
        :class="statusType"
        data-testid="export-status"
      >
        {{ statusMessage }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { NSelect, NButton } from 'naive-ui'
import { listProjects, type ProjectResult } from '../api/projects'
import { listSubjects, type SubjectResult } from '../api/subjects'
import { createExport, downloadZip } from '../api/exports'

const loadingProjects = ref(true)
const projects = ref<ProjectResult[]>([])
const subjects = ref<SubjectResult[]>([])
const selectedProjectId = ref<string | null>(null)
const selectedSubjectIds = ref<string[]>([])
const selectedFormat = ref('kohya-ss')
const serverExporting = ref(false)
const downloading = ref(false)
const statusMessage = ref('')
const statusType = ref<'success' | 'error'>('success')

const formatOptions = [
  { label: 'kohya-ss', value: 'kohya-ss' },
]

const projectOptions = ref<Array<{ label: string; value: string }>>([])
const subjectOptions = ref<Array<{ label: string; value: string }>>([])

onMounted(async () => {
  try {
    const result = await listProjects()
    projects.value = result.projects
    projectOptions.value = result.projects.map((p) => ({
      label: p.name,
      value: p.id,
    }))
  } finally {
    loadingProjects.value = false
  }
})

async function onProjectChange(projectId: string) {
  selectedSubjectIds.value = []
  subjects.value = []
  subjectOptions.value = []
  statusMessage.value = ''

  if (!projectId) return

  const result = await listSubjects(projectId)
  subjects.value = result.subjects
  subjectOptions.value = result.subjects.map((s) => ({
    label: `${s.name} (${s.sample_count} samples)`,
    value: s.id,
  }))
}

async function triggerServerExport() {
  serverExporting.value = true
  statusMessage.value = ''
  try {
    const result = await createExport({
      subject_ids: selectedSubjectIds.value,
      format: selectedFormat.value,
    })
    statusType.value = 'success'
    statusMessage.value = `Export started: ${result.total_items} samples (Job: ${result.job_id.slice(0, 8)}...)`
  } catch (err: unknown) {
    statusType.value = 'error'
    statusMessage.value = err instanceof Error ? err.message : String(err)
  } finally {
    serverExporting.value = false
  }
}

async function triggerZipDownload() {
  downloading.value = true
  statusMessage.value = ''
  try {
    await downloadZip({
      subject_ids: selectedSubjectIds.value,
      format: selectedFormat.value,
    })
    statusType.value = 'success'
    statusMessage.value = 'Zip download started.'
  } catch (err: unknown) {
    statusType.value = 'error'
    statusMessage.value = err instanceof Error ? err.message : String(err)
  } finally {
    downloading.value = false
  }
}
</script>

<style scoped>
.export-view {
  padding: 16px 24px;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.view-title {
  font-size: 1.5rem;
  font-weight: 600;
  color: var(--text-color);
  margin: 0;
}

.loading-text {
  color: var(--text-color-secondary);
  padding: 20px 0;
}

.export-form {
  max-width: 480px;
}

.form-group {
  margin-bottom: 16px;
}

.form-label {
  display: block;
  margin-bottom: 4px;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--text-color);
}

.export-actions {
  display: flex;
  gap: 12px;
  margin-top: 24px;
}

.export-status {
  margin-top: 16px;
  padding: 8px 12px;
  border-radius: 4px;
  font-size: 0.875rem;
}

.export-status.success {
  background-color: var(--success-bg, rgba(0, 128, 0, 0.1));
  color: var(--success-color, #18a058);
}

.export-status.error {
  background-color: var(--error-bg, rgba(255, 0, 0, 0.1));
  color: var(--error-color, #d03050);
}
</style>
