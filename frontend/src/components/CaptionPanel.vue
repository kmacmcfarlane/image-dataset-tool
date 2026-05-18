<template>
  <!--
    CaptionPanel: side panel showing captions grouped by study for a sample.

    Supports viewing, editing (manual pseudo-study), and navigation.

    Events:
    - navigate: emitted when next/prev is clicked, payload is 'prev' | 'next'
  -->
  <div class="caption-panel" data-testid="caption-panel">
    <!-- Header with navigation -->
    <div class="caption-panel-header">
      <n-text class="panel-title">Captions</n-text>
      <div class="nav-buttons">
        <n-button
          size="tiny"
          quaternary
          data-testid="caption-nav-prev"
          @click="$emit('navigate', 'prev')"
        >
          Prev
        </n-button>
        <n-button
          size="tiny"
          quaternary
          data-testid="caption-nav-next"
          @click="$emit('navigate', 'next')"
        >
          Next
        </n-button>
      </div>
    </div>

    <!-- Loading state -->
    <n-spin v-if="captionsStore.loading" size="small" data-testid="caption-loading" />

    <!-- No captions state -->
    <div
      v-else-if="groupedCaptions.length === 0 && !isEditing"
      class="no-captions"
      data-testid="caption-empty"
    >
      <n-text depth="3">No captions yet.</n-text>
      <router-link to="/studies" data-testid="caption-studies-link">
        <n-button size="small" quaternary>Go to Studies</n-button>
      </router-link>
    </div>

    <!-- Caption groups by study -->
    <div v-else class="caption-groups">
      <div
        v-for="group in groupedCaptions"
        :key="group.study.id"
        class="caption-group"
        :data-testid="`caption-group-${group.study.slug}`"
      >
        <!-- Study header -->
        <div class="study-header">
          <n-text class="study-name" strong>{{ group.study.name }}</n-text>
          <n-text depth="3" class="study-meta">
            {{ group.study.provider }} / {{ group.study.model }}
          </n-text>
        </div>

        <!-- Caption text (read-only for non-manual studies) -->
        <div class="caption-text-wrap">
          <n-text
            v-if="group.study.slug !== 'manual'"
            class="caption-text"
            :data-testid="`caption-text-${group.study.slug}`"
          >
            {{ group.caption.text }}
          </n-text>
          <!-- Manual study gets editable textarea -->
          <textarea
            v-else
            class="caption-textarea"
            :value="group.caption.text"
            :data-testid="`caption-text-${group.study.slug}`"
            rows="4"
            @input="onManualInput($event)"
            @blur="onManualBlur"
          />
        </div>
      </div>

      <!-- Inline edit section (always visible when there are captions) -->
      <div
        v-if="!hasManualCaption"
        class="caption-group caption-edit-group"
        data-testid="caption-edit-section"
      >
        <div class="study-header">
          <n-text class="study-name" strong>Manual Edit</n-text>
          <n-text depth="3" class="study-meta">manual / manual</n-text>
        </div>
        <textarea
          class="caption-textarea"
          :value="editText"
          data-testid="caption-edit-textarea"
          placeholder="Type to add a manual caption..."
          rows="4"
          @input="onManualInput($event)"
          @blur="onManualBlur"
          @focus="isEditing = true"
        />
      </div>
    </div>

    <!-- Saving indicator -->
    <div v-if="captionsStore.saving" class="saving-indicator" data-testid="caption-saving">
      <n-text depth="3">Saving...</n-text>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * CaptionPanel: side panel for viewing/editing captions, grouped by study.
 *
 * Events:
 * - navigate: 'prev' | 'next' — emitted when navigation buttons are clicked
 */
import { ref, computed, watch, onMounted } from 'vue'
import { NButton, NSpin, NText } from 'naive-ui'
import { useCaptionsStore } from '../stores/captions'
import type { Caption, CaptionStudy } from '../api/captions'

interface CaptionGroup {
  study: CaptionStudy
  caption: Caption
}

const props = defineProps<{
  sampleId: string
}>()

defineEmits<{
  /** navigate: emitted when prev/next button is clicked */
  (e: 'navigate', direction: 'prev' | 'next'): void
}>()

const captionsStore = useCaptionsStore()

const editText = ref('')
const isEditing = ref(false)

// Debounce timer for manual saves.
let saveTimer: ReturnType<typeof setTimeout> | null = null
const SAVE_DEBOUNCE_MS = 800

/** Group captions by study, one caption per study (latest). */
const groupedCaptions = computed((): CaptionGroup[] => {
  const captions = captionsStore.getCaptions(props.sampleId)
  // Backend already returns one caption per study (UNIQUE constraint).
  // Group them ordered: non-manual studies first, then manual.
  const groups: CaptionGroup[] = []
  const manualGroup: CaptionGroup[] = []

  for (const c of captions) {
    const group: CaptionGroup = {
      study: c.study,
      caption: c,
    }
    if (c.study.slug === 'manual') {
      manualGroup.push(group)
    } else {
      groups.push(group)
    }
  }

  return [...groups, ...manualGroup]
})

/** Whether there is already a manual caption for this sample. */
const hasManualCaption = computed(() =>
  groupedCaptions.value.some((g) => g.study.slug === 'manual'),
)

/** Handle input on the manual edit textarea. */
function onManualInput(event: Event): void {
  const target = event.target as HTMLTextAreaElement
  editText.value = target.value
  isEditing.value = true

  // Debounce auto-save.
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(() => {
    if (editText.value.trim()) {
      captionsStore.saveManualCaption(props.sampleId, editText.value)
    }
  }, SAVE_DEBOUNCE_MS)
}

/** Save on blur if there's pending text. */
function onManualBlur(): void {
  if (saveTimer) {
    clearTimeout(saveTimer)
    saveTimer = null
  }
  if (editText.value.trim()) {
    captionsStore.saveManualCaption(props.sampleId, editText.value)
  }
}

/** Load captions when sample changes. */
async function loadForSample(sampleId: string): Promise<void> {
  editText.value = ''
  isEditing.value = false
  await captionsStore.loadCaptions(sampleId)
  // Pre-fill edit text from existing manual caption.
  const manualCaption = captionsStore.getCaptions(sampleId).find(
    (c) => c.study.slug === 'manual',
  )
  if (manualCaption) {
    editText.value = manualCaption.text
  }
}

onMounted(() => {
  if (props.sampleId) {
    loadForSample(props.sampleId)
  }
})

watch(() => props.sampleId, (newId) => {
  if (newId) {
    loadForSample(newId)
  }
})
</script>

<style scoped>
.caption-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  width: 100%;
  height: 100%;
  overflow-y: auto;
}

.caption-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.panel-title {
  font-weight: 600;
  font-size: 15px;
}

.nav-buttons {
  display: flex;
  gap: 4px;
}

.no-captions {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 24px 0;
}

.caption-groups {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.caption-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.study-header {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.study-name {
  font-size: 13px;
}

.study-meta {
  font-size: 11px;
}

.caption-text-wrap {
  min-height: 40px;
}

.caption-text {
  font-size: 13px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

.caption-textarea {
  width: 100%;
  min-height: 80px;
  padding: 8px;
  border: 1px solid var(--n-border-color, #444);
  border-radius: 4px;
  background: var(--n-color-embedded, #1a1a1a);
  color: var(--n-text-color, #fff);
  font-family: inherit;
  font-size: 13px;
  line-height: 1.5;
  resize: vertical;
  outline: none;
}

.caption-textarea:focus {
  border-color: var(--n-primary-color, #63e2b7);
}

.caption-textarea::placeholder {
  color: var(--n-text-color-3, #888);
}

.saving-indicator {
  font-size: 12px;
}
</style>
