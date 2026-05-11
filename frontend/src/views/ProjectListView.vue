<template>
  <div
    class="project-list-view"
    data-testid="view-project-list"
  >
    <div class="view-header">
      <h2 class="view-title">
        Projects
      </h2>
      <n-button
        type="primary"
        size="small"
        data-testid="create-project-btn"
        @click="openCreateDialog"
      >
        New Project
      </n-button>
    </div>

    <div
      v-if="store.loading"
      class="loading-text"
      data-testid="projects-loading"
    >
      Loading...
    </div>

    <div
      v-else-if="store.projects.length === 0"
      class="empty-state"
      data-testid="projects-empty"
    >
      <p>No projects yet. Create one to get started.</p>
    </div>

    <div
      v-else
      class="project-grid"
      data-testid="project-list"
    >
      <div
        v-for="project in store.projects"
        :key="project.id"
        class="project-card"
        :data-testid="`project-card-${project.slug}`"
        @click="navigateToProject(project.id)"
        @contextmenu.prevent="showContextMenu($event, project)"
      >
        <div class="project-card-header">
          <h3 class="project-name">
            {{ project.name }}
          </h3>
          <n-button
            quaternary
            size="tiny"
            class="menu-btn"
            :data-testid="`project-menu-${project.slug}`"
            @click.stop="showContextMenu($event, project)"
          >
            ...
          </n-button>
        </div>
        <p
          v-if="project.description"
          class="project-description"
        >
          {{ project.description }}
        </p>
        <div class="project-meta">
          <span class="project-slug">{{ project.slug }}</span>
          <span class="project-date">{{ formatDate(project.created_at) }}</span>
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
      data-testid="project-context-menu"
      @select="handleContextAction"
      @clickoutside="contextMenuVisible = false"
    />

    <!-- Create/Edit Project Dialog -->
    <n-modal
      v-model:show="showProjectDialog"
      preset="dialog"
      :title="editingProject ? 'Edit Project' : 'New Project'"
      positive-text="Save"
      negative-text="Cancel"
      data-testid="project-dialog"
      @positive-click="handleSaveProject"
      @negative-click="closeProjectDialog"
    >
      <div class="project-form">
        <div class="form-field">
          <label class="form-label">Name</label>
          <input
            v-model="projectForm.name"
            class="form-input"
            placeholder="Project name"
            data-testid="project-name-input"
          >
        </div>
        <div class="form-field">
          <label class="form-label">Description</label>
          <textarea
            v-model="projectForm.description"
            class="form-textarea"
            placeholder="Optional description"
            rows="3"
            data-testid="project-description-input"
          />
        </div>
      </div>
    </n-modal>

    <!-- Delete Confirmation Dialog -->
    <n-modal
      v-model:show="showDeleteDialog"
      preset="dialog"
      type="warning"
      title="Delete Project"
      positive-text="Delete"
      negative-text="Cancel"
      data-testid="delete-project-dialog"
      @positive-click="handleConfirmDelete"
      @negative-click="showDeleteDialog = false"
    >
      <div class="delete-form">
        <p>Are you sure you want to delete "{{ deletingProject?.name }}"?</p>
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
import { useRouter } from 'vue-router'
import { NButton, NModal, NDropdown } from 'naive-ui'
import { useProjectsStore } from '../stores/projects'
import { useToastStore } from '../stores/toast'
import type { ProjectResult } from '../api/projects'

const store = useProjectsStore()
const toastStore = useToastStore()
const router = useRouter()

// Dialog state
const showProjectDialog = ref(false)
const editingProject = ref<ProjectResult | null>(null)
const projectForm = ref({ name: '', description: '' })

const showDeleteDialog = ref(false)
const deletingProject = ref<ProjectResult | null>(null)
const hardDelete = ref(false)

// Context menu state
const contextMenuVisible = ref(false)
const contextMenuX = ref(0)
const contextMenuY = ref(0)
const contextMenuProject = ref<ProjectResult | null>(null)

const contextMenuOptions = [
  { label: 'Edit', key: 'edit' },
  { label: 'Delete', key: 'delete' },
]

onMounted(async () => {
  await store.load()
})

function navigateToProject(id: string): void {
  router.push({ name: 'project-detail', params: { id } })
}

function formatDate(dateStr: string): string {
  try {
    return new Date(dateStr).toLocaleDateString()
  } catch {
    return dateStr
  }
}

function showContextMenu(event: MouseEvent, project: ProjectResult): void {
  contextMenuProject.value = project
  contextMenuX.value = event.clientX
  contextMenuY.value = event.clientY
  contextMenuVisible.value = true
}

function handleContextAction(key: string): void {
  contextMenuVisible.value = false
  const project = contextMenuProject.value
  if (!project) return

  if (key === 'edit') {
    openEditDialog(project)
  } else if (key === 'delete') {
    openDeleteDialog(project)
  }
}

function openCreateDialog(): void {
  editingProject.value = null
  projectForm.value = { name: '', description: '' }
  showProjectDialog.value = true
}

function openEditDialog(project: ProjectResult): void {
  editingProject.value = project
  projectForm.value = { name: project.name, description: project.description }
  showProjectDialog.value = true
}

function closeProjectDialog(): void {
  showProjectDialog.value = false
  editingProject.value = null
  projectForm.value = { name: '', description: '' }
}

async function handleSaveProject(): Promise<boolean | void> {
  const { name, description } = projectForm.value
  if (!name.trim()) {
    toastStore.addToast('warning', 'Project name cannot be empty')
    return false
  }

  try {
    if (editingProject.value) {
      await store.update(editingProject.value.id, { name: name.trim(), description })
      toastStore.addToast('info', `Project "${name}" updated`)
    } else {
      await store.create({ name: name.trim(), description })
      toastStore.addToast('info', `Project "${name}" created`)
    }
    closeProjectDialog()
  } catch {
    // Error already handled by API error handler
  }
}

function openDeleteDialog(project: ProjectResult): void {
  deletingProject.value = project
  hardDelete.value = false
  showDeleteDialog.value = true
}

async function handleConfirmDelete(): Promise<void> {
  if (!deletingProject.value) return

  try {
    await store.remove(deletingProject.value.id, hardDelete.value)
    toastStore.addToast('info', `Project "${deletingProject.value.name}" deleted`)
  } catch {
    // Error already handled by API error handler
  }
  showDeleteDialog.value = false
  deletingProject.value = null
}
</script>

<style scoped>
.project-list-view {
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

.project-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
}

.project-card {
  border: 1px solid var(--border-color);
  border-radius: 6px;
  padding: 14px;
  background: var(--bg-secondary);
  cursor: pointer;
  transition: border-color 0.15s;
}

.project-card:hover {
  border-color: var(--accent-color);
}

.project-card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.project-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-color);
  margin: 0;
}

.menu-btn {
  flex-shrink: 0;
  font-weight: bold;
}

.project-description {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 6px 0 0;
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.project-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 10px;
  font-size: 12px;
  color: var(--text-secondary);
}

.project-slug {
  font-family: monospace;
  background: var(--bg-color);
  padding: 1px 5px;
  border-radius: 3px;
}

/* Form styles */
.project-form {
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
