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

    <!-- Link Account Dialog -->
    <n-modal
      v-model:show="showLinkAccountDialog"
      preset="dialog"
      title="Link Account"
      positive-text="Link"
      negative-text="Cancel"
      data-testid="link-account-dialog"
      @positive-click="handleLinkAccount"
      @negative-click="closeLinkAccountDialog"
    >
      <div class="link-account-form">
        <p class="link-account-subject-name">
          Subject: <strong>{{ linkAccountSubject?.name }}</strong>
        </p>

        <div
          v-if="linkedAccountIds.size > 0"
          class="linked-accounts-list"
          data-testid="linked-accounts-list"
        >
          <p class="form-label">
            Currently linked:
          </p>
          <div
            v-for="a in currentLinkedAccounts"
            :key="a.id"
            class="linked-account-row"
            :data-testid="`linked-account-row-${a.id}`"
          >
            <span class="linked-account-handle">{{ a.platform }}/{{ a.handle }}</span>
            <n-button
              size="tiny"
              type="error"
              quaternary
              :data-testid="`unlink-account-btn-${a.id}`"
              @click="handleUnlinkAccount(a.id)"
            >
              Unlink
            </n-button>
          </div>
        </div>

        <div class="form-field">
          <label class="form-label">Add account</label>
          <select
            v-model="selectedAccountId"
            class="form-input"
            data-testid="account-picker"
          >
            <option value="">
              -- select an account --
            </option>
            <option
              v-for="a in availableAccounts"
              :key="a.id"
              :value="a.id"
              :data-testid="`account-option-${a.id}`"
            >
              {{ a.platform }}/{{ a.handle }}
            </option>
          </select>
        </div>

        <div class="accounts-nav-hint">
          <span class="nav-hint-text">Don't see the account you need?</span>
          <router-link
            to="/accounts"
            class="nav-link"
            data-testid="go-to-accounts-link"
            @click="closeLinkAccountDialog"
          >
            Manage accounts
          </router-link>
        </div>
      </div>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NModal, NDropdown } from 'naive-ui'
import { useSubjectsStore } from '../stores/subjects'
import { useToastStore } from '../stores/toast'
import { getProject } from '../api/projects'
import type { ProjectDetail } from '../api/projects'
import { listAccounts } from '../api/accounts'
import type { AccountResult } from '../api/accounts'
import { linkAccount, unlinkAccount, listLinkedAccounts } from '../api/subjects'
import type { SubjectResult, SubjectLinkedAccount } from '../api/subjects'

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
  { label: 'Link account', key: 'link-account' },
  { label: 'Delete', key: 'delete' },
]

// Link account dialog state
const showLinkAccountDialog = ref(false)
const linkAccountSubject = ref<SubjectResult | null>(null)
const selectedAccountId = ref('')
const allAccounts = ref<AccountResult[]>([])
const currentLinkedAccounts = ref<SubjectLinkedAccount[]>([])

const linkedAccountIds = computed(() => new Set(currentLinkedAccounts.value.map((a) => a.id)))

const availableAccounts = computed(() =>
  allAccounts.value.filter((a) => !linkedAccountIds.value.has(a.id)),
)

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
  } else if (key === 'link-account') {
    openLinkAccountDialog(subject)
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

async function openLinkAccountDialog(subject: SubjectResult): Promise<void> {
  linkAccountSubject.value = subject
  selectedAccountId.value = ''
  showLinkAccountDialog.value = true

  try {
    const [accountsResult, linkedResult] = await Promise.all([
      listAccounts(),
      listLinkedAccounts(projectId, subject.id),
    ])
    allAccounts.value = accountsResult.accounts
    currentLinkedAccounts.value = linkedResult.accounts
  } catch {
    // Error already handled by API error handler
  }
}

function closeLinkAccountDialog(): void {
  showLinkAccountDialog.value = false
  linkAccountSubject.value = null
  selectedAccountId.value = ''
  allAccounts.value = []
  currentLinkedAccounts.value = []
}

async function handleLinkAccount(): Promise<boolean | void> {
  if (!linkAccountSubject.value || !selectedAccountId.value) {
    toastStore.addToast('warning', 'Please select an account to link')
    return false
  }

  try {
    await linkAccount(projectId, linkAccountSubject.value.id, selectedAccountId.value)
    // Refresh linked accounts list
    const linkedResult = await listLinkedAccounts(projectId, linkAccountSubject.value.id)
    currentLinkedAccounts.value = linkedResult.accounts
    selectedAccountId.value = ''
    toastStore.addToast('info', 'Account linked successfully')
  } catch {
    // Error already handled by API error handler
    return false
  }
  return false // keep dialog open so user can link more
}

async function handleUnlinkAccount(accountId: string): Promise<void> {
  if (!linkAccountSubject.value) return

  try {
    await unlinkAccount(projectId, linkAccountSubject.value.id, accountId)
    const linkedResult = await listLinkedAccounts(projectId, linkAccountSubject.value.id)
    currentLinkedAccounts.value = linkedResult.accounts
    toastStore.addToast('info', 'Account unlinked')
  } catch {
    // Error already handled by API error handler
  }
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

/* Link account form */
.link-account-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding-top: 4px;
}

.link-account-subject-name {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 0;
}

.linked-accounts-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.linked-account-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 8px;
  border: 1px solid var(--border-color);
  border-radius: 4px;
  background: var(--bg-color);
}

.linked-account-handle {
  font-size: 13px;
  color: var(--text-color);
  font-family: monospace;
}

.accounts-nav-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.nav-hint-text {
  color: var(--text-secondary);
}

.nav-link {
  color: var(--accent-color);
  text-decoration: none;
  font-weight: 500;
}

.nav-link:hover {
  text-decoration: underline;
}
</style>
