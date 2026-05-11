<template>
  <div
    class="project-detail-view"
    data-testid="view-project-detail"
  >
    <div class="view-header">
      <h2 class="view-title">
        {{ project?.name ?? 'Project' }}
      </h2>
      <n-button
        type="primary"
        size="small"
        data-testid="create-subject-btn"
        @click="openCreateDialog"
      >
        New Subject
      </n-button>
    </div>

    <div
      v-if="subjectStore.loading"
      class="loading-text"
      data-testid="subjects-loading"
    >
      Loading...
    </div>

    <div
      v-else-if="subjectStore.subjects.length === 0"
      class="empty-state"
      data-testid="subjects-empty"
    >
      <p>No subjects yet. Create one to get started.</p>
    </div>

    <div
      v-else
      class="subject-grid"
      data-testid="subject-list"
    >
      <div
        v-for="subject in subjectStore.subjects"
        :key="subject.id"
        class="subject-card"
        :data-testid="`subject-card-${subject.slug}`"
        @contextmenu.prevent="showContextMenu($event, subject)"
      >
        <div class="subject-card-header">
          <h3 class="subject-name">
            {{ subject.name }}
          </h3>
          <n-button
            quaternary
            size="tiny"
            class="menu-btn"
            :data-testid="`subject-menu-${subject.slug}`"
            @click.stop="showContextMenu($event, subject)"
          >
            ...
          </n-button>
        </div>
        <div class="subject-meta">
          <span class="subject-slug">{{ subject.slug }}</span>
          <span
            class="sample-count-badge"
            :data-testid="`subject-sample-count-${subject.slug}`"
          >
            {{ subject.sample_count }} samples
          </span>
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
      data-testid="subject-context-menu"
      @select="handleContextAction"
      @clickoutside="contextMenuVisible = false"
    />

    <!-- Create/Edit Subject Dialog -->
    <n-modal
      v-model:show="showSubjectDialog"
      preset="dialog"
      :title="editingSubject ? 'Edit Subject' : 'New Subject'"
      positive-text="Save"
      negative-text="Cancel"
      data-testid="subject-dialog"
      @positive-click="handleSaveSubject"
      @negative-click="closeSubjectDialog"
    >
      <div class="subject-form">
        <div class="form-field">
          <label class="form-label">Name</label>
          <input
            v-model="subjectForm.name"
            class="form-input"
            placeholder="Subject name"
            data-testid="subject-name-input"
          >
        </div>
      </div>
    </n-modal>

    <!-- Delete Confirmation Dialog -->
    <n-modal
      v-model:show="showDeleteDialog"
      preset="dialog"
      type="warning"
      title="Delete Subject"
      positive-text="Delete"
      negative-text="Cancel"
      data-testid="delete-subject-dialog"
      @positive-click="handleConfirmDelete"
      @negative-click="showDeleteDialog = false"
    >
      <div class="delete-form">
        <p>Are you sure you want to delete "{{ deletingSubject?.name }}"?</p>
        <label class="checkbox-label">
          <input
            v-model="hardDelete"
            type="checkbox"
            data-testid="hard-delete-checkbox"
          >
          Also delete files from disk
        </label>
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NModal, NDropdown } from 'naive-ui'
import { useSubjectsStore } from '../stores/subjects'
import { useToastStore } from '../stores/toast'
import { getProject } from '../api/projects'
import type { ProjectDetail } from '../api/projects'
import type { SubjectResult } from '../api/subjects'

const route = useRoute()
const subjectStore = useSubjectsStore()
const toastStore = useToastStore()

const projectId = route.params.id as string
const project = ref<ProjectDetail | null>(null)

// Dialog state
const showSubjectDialog = ref(false)
const editingSubject = ref<SubjectResult | null>(null)
const subjectForm = ref({ name: '' })

const showDeleteDialog = ref(false)
const deletingSubject = ref<SubjectResult | null>(null)
const hardDelete = ref(false)

// Context menu state
const contextMenuVisible = ref(false)
const contextMenuX = ref(0)
const contextMenuY = ref(0)
const contextMenuSubject = ref<SubjectResult | null>(null)

const contextMenuOptions = [
  { label: 'Edit', key: 'edit' },
  { label: 'Delete', key: 'delete' },
]

onMounted(async () => {
  try {
    project.value = await getProject(projectId)
  } catch {
    // Error handled by API error handler
  }
  await subjectStore.load(projectId)
})

function showContextMenu(event: MouseEvent, subject: SubjectResult): void {
  contextMenuSubject.value = subject
  contextMenuX.value = event.clientX
  contextMenuY.value = event.clientY
  contextMenuVisible.value = true
}

function handleContextAction(key: string): void {
  contextMenuVisible.value = false
  const subject = contextMenuSubject.value
  if (!subject) return

  if (key === 'edit') {
    openEditDialog(subject)
  } else if (key === 'delete') {
    openDeleteDialog(subject)
  }
}

function openCreateDialog(): void {
  editingSubject.value = null
  subjectForm.value = { name: '' }
  showSubjectDialog.value = true
}

function openEditDialog(subject: SubjectResult): void {
  editingSubject.value = subject
  subjectForm.value = { name: subject.name }
  showSubjectDialog.value = true
}

function closeSubjectDialog(): void {
  showSubjectDialog.value = false
  editingSubject.value = null
  subjectForm.value = { name: '' }
}

async function handleSaveSubject(): Promise<boolean | void> {
  const { name } = subjectForm.value
  if (!name.trim()) {
    toastStore.addToast('warning', 'Subject name cannot be empty')
    return false
  }

  try {
    if (editingSubject.value) {
      await subjectStore.update(projectId, editingSubject.value.id, { name: name.trim() })
      toastStore.addToast('info', `Subject "${name}" updated`)
    } else {
      await subjectStore.create(projectId, { name: name.trim() })
      toastStore.addToast('info', `Subject "${name}" created`)
    }
    closeSubjectDialog()
  } catch {
    // Error already handled by API error handler
  }
}

function openDeleteDialog(subject: SubjectResult): void {
  deletingSubject.value = subject
  hardDelete.value = false
  showDeleteDialog.value = true
}

async function handleConfirmDelete(): Promise<void> {
  if (!deletingSubject.value) return

  try {
    await subjectStore.remove(projectId, deletingSubject.value.id, hardDelete.value)
    toastStore.addToast('info', `Subject "${deletingSubject.value.name}" deleted`)
  } catch {
    // Error already handled by API error handler
  }
  showDeleteDialog.value = false
  deletingSubject.value = null
}
</script>

<style scoped>
.project-detail-view {
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

.subject-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
}

.subject-card {
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 14px;
  background: var(--bg-secondary);
  cursor: default;
  transition: border-color 0.15s;
}

.subject-card:hover {
  border-color: var(--accent-color);
}

.subject-card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.subject-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-color);
  margin: 0;
}

.menu-btn {
  flex-shrink: 0;
  font-weight: bold;
}

.subject-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 10px;
  font-size: 12px;
  color: var(--text-secondary);
}

.subject-slug {
  font-family: monospace;
  background: var(--bg-color);
  padding: 1px 5px;
  border-radius: 3px;
}

.sample-count-badge {
  background: var(--accent-color);
  color: var(--bg-color);
  padding: 2px 8px;
  border-radius: 10px;
  font-size: 11px;
  font-weight: 600;
}

/* Form styles */
.subject-form {
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
  font-family: inherit;
  outline: none;
}

.form-input:focus {
  border-color: var(--accent-color);
}

/* Delete form */
.delete-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 4px;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--text-secondary);
  cursor: pointer;
}
</style>
