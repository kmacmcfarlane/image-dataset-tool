<template>
  <div
    class="sample-grid-view"
    data-testid="view-sample-grid"
    tabindex="0"
    @keydown="onKeydown"
  >
    <div class="sample-grid-header">
      <h2>Sample Review Grid</h2>
      <div class="sample-grid-controls" data-testid="sample-grid-controls">
        <n-space align="center" :size="12">
          <n-text depth="3">{{ filteredCount }} / {{ totalCount }} samples</n-text>
          <n-text
            v-if="store.selectedCount > 0"
            depth="3"
            data-testid="selection-count"
          >
            ({{ store.selectedCount }} selected)
          </n-text>
          <n-divider vertical />
          <n-checkbox
            :checked="store.showPending"
            data-testid="filter-pending"
            @update:checked="(v: boolean) => store.setStatusFilter('pending', v)"
          >
            Pending
          </n-checkbox>
          <n-checkbox
            :checked="store.showKept"
            data-testid="filter-kept"
            @update:checked="(v: boolean) => store.setStatusFilter('kept', v)"
          >
            Kept
          </n-checkbox>
          <n-checkbox
            :checked="store.showRejected"
            data-testid="filter-rejected"
            @update:checked="(v: boolean) => store.setStatusFilter('rejected', v)"
          >
            Rejected
          </n-checkbox>
          <n-divider vertical />
          <n-checkbox
            :checked="store.showDuplicates"
            data-testid="filter-duplicates"
            @update:checked="(v: boolean) => store.setShowDuplicates(v)"
          >
            Show duplicates
          </n-checkbox>
          <n-divider vertical />
          <n-button
            size="small"
            :disabled="!store.focusedId"
            data-testid="reset-edits-btn"
            @click="onResetEdits"
          >
            Reset Edits
          </n-button>
        </n-space>
      </div>
    </div>

    <n-spin :show="store.loading" data-testid="sample-grid-loading">
      <div
        v-if="store.error"
        class="sample-grid-error"
        data-testid="sample-grid-error"
      >
        <n-text type="error">{{ store.error }}</n-text>
      </div>

      <div
        v-else-if="!store.loading && filteredCount === 0"
        class="sample-grid-empty"
        data-testid="sample-grid-empty"
      >
        <n-empty description="No samples match current filters" />
      </div>

      <div
        v-else
        ref="scrollContainer"
        class="sample-grid-scroll"
        data-testid="sample-grid-scroll"
        @scroll="onScroll"
      >
        <div :style="{ height: totalHeight + 'px', position: 'relative' }">
          <div
            class="sample-grid"
            :style="{ transform: `translateY(${offsetY}px)` }"
          >
            <div
              v-for="sample in visibleSamples"
              :key="sample.id"
              class="sample-card"
              :class="{
                'card-selected': store.isSelected(sample.id),
                'card-focused': store.focusedId === sample.id,
              }"
              :data-testid="`sample-card-${sample.id}`"
              :data-sample-id="sample.id"
              @click="onCardClick(sample.id, $event)"
              @dblclick="onCardDblclick(sample.id)"
            >
              <SampleThumbnail
                :sample-id="sample.id"
                :thumbnail-path="sample.thumbnail_path"
                :status="sample.status"
                :is-duplicate="sample.is_duplicate"
                :duplicate-of="sample.duplicate_of"
                :selected="store.isSelected(sample.id)"
                :focused="store.focusedId === sample.id"
              />
            </div>
          </div>
        </div>
      </div>
    </n-spin>

    <!-- Lightbox modal: double-click a card to open it with full edit controls -->
    <n-modal
      v-model:show="lightboxOpen"
      data-testid="sample-lightbox"
      :mask-closable="true"
      preset="card"
      style="width: 90vw; max-width: 1200px; height: 80vh;"
      :title="`Sample ${lightboxSampleId ?? ''}`"
      @after-leave="onLightboxClose"
    >
      <LightboxEditor
        v-if="lightboxSampleId"
        :sample-id="lightboxSampleId"
      />
    </n-modal>
  </div>
</template>

<script setup lang="ts">
/**
 * SampleGridView: Virtual-scrolled sample review grid with selection and keyboard shortcuts.
 *
 * Keyboard shortcuts (PRD §8.3.1):
 * - K: Keep selected/focused sample(s)
 * - R: Reject selected/focused sample(s)
 * - Arrow left/right: Navigate prev/next sample
 * - Space: Toggle selection on focused sample
 * - Shift+Click: Range select
 * - Ctrl+A: Select all visible
 * - Ctrl+Shift+A: Deselect all
 * - Escape: Clear selection
 * - Double-click card: Open lightbox with full edit controls
 */
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NCheckbox, NDivider, NEmpty, NModal, NSpace, NSpin, NText } from 'naive-ui'
import { useSamplesStore } from '../stores/samples'
import SampleThumbnail from '../components/SampleThumbnail.vue'
import LightboxEditor from '../components/LightboxEditor.vue'

const route = useRoute()
const store = useSamplesStore()

const scrollContainer = ref<HTMLElement | null>(null)

// Lightbox state.
const lightboxOpen = ref(false)
const lightboxSampleId = ref<string | null>(null)

// Virtual scroll configuration.
const CARD_SIZE = 180 // px (card width/height including gap)
const BUFFER_ROWS = 3 // extra rows rendered above/below viewport

const scrollTop = ref(0)
const containerHeight = ref(800)

const filteredCount = computed(() => store.filteredSamples.length)
const totalCount = computed(() => store.totalCount)

const columnsPerRow = computed(() => {
  if (!scrollContainer.value) return 4
  const width = scrollContainer.value.clientWidth
  return Math.max(1, Math.floor(width / CARD_SIZE))
})

const totalRows = computed(() =>
  Math.ceil(filteredCount.value / columnsPerRow.value),
)

const totalHeight = computed(() => totalRows.value * CARD_SIZE)

const visibleStartRow = computed(() =>
  Math.max(0, Math.floor(scrollTop.value / CARD_SIZE) - BUFFER_ROWS),
)

const visibleEndRow = computed(() => {
  const rowsInView = Math.ceil(containerHeight.value / CARD_SIZE)
  return Math.min(totalRows.value, Math.floor(scrollTop.value / CARD_SIZE) + rowsInView + BUFFER_ROWS)
})

const offsetY = computed(() => visibleStartRow.value * CARD_SIZE)

const visibleSamples = computed(() => {
  const startIdx = visibleStartRow.value * columnsPerRow.value
  const endIdx = visibleEndRow.value * columnsPerRow.value
  return store.filteredSamples.slice(startIdx, endIdx)
})

function onScroll(): void {
  if (scrollContainer.value) {
    scrollTop.value = scrollContainer.value.scrollTop
  }
}

function updateContainerSize(): void {
  if (scrollContainer.value) {
    containerHeight.value = scrollContainer.value.clientHeight
  }
}

/** Reset edits for the focused sample (saves immediately). */
async function onResetEdits(): Promise<void> {
  if (store.focusedId) {
    await store.resetEdits(store.focusedId)
  }
}

/** Handle click on a sample card with modifier key support. */
function onCardClick(id: string, event: MouseEvent): void {
  if (event.shiftKey) {
    // AC: Shift+click range select
    store.rangeSelect(id)
  } else if (event.ctrlKey || event.metaKey) {
    // Ctrl/Cmd+click toggle selection
    store.toggleSelect(id)
  } else {
    // Plain click: single select
    store.selectSample(id)
  }
}

/** AC: Double-click opens the lightbox editor for that sample. */
function onCardDblclick(id: string): void {
  lightboxSampleId.value = id
  lightboxOpen.value = true
}

/** Called when the lightbox modal finishes closing. */
function onLightboxClose(): void {
  lightboxSampleId.value = null
}

/**
 * Capture-phase keyboard handler (per DEVELOPMENT_PRACTICES §4.10).
 * Handles all grid-level keyboard shortcuts.
 */
function onKeydown(event: KeyboardEvent): void {
  // AC: Ctrl+A select all; Ctrl+Shift+A deselect
  if ((event.ctrlKey || event.metaKey) && event.key === 'a') {
    event.preventDefault()
    if (event.shiftKey) {
      store.clearSelection()
    } else {
      store.selectAll()
    }
    return
  }

  // AC: K=keep, R=reject on selected samples
  if (event.key === 'k' || event.key === 'K') {
    event.preventDefault()
    store.applyStatus('kept')
    return
  }

  if (event.key === 'r' || event.key === 'R') {
    event.preventDefault()
    store.applyStatus('rejected')
    return
  }

  // Arrow left/right: navigate
  if (event.key === 'ArrowLeft') {
    event.preventDefault()
    store.moveFocus('prev')
    return
  }

  if (event.key === 'ArrowRight') {
    event.preventDefault()
    store.moveFocus('next')
    return
  }

  // Space: toggle selection on focused sample
  if (event.key === ' ') {
    event.preventDefault()
    if (store.focusedId) {
      store.toggleSelect(store.focusedId)
    }
    return
  }

  // Escape: clear selection
  if (event.key === 'Escape') {
    event.preventDefault()
    store.clearSelection()
    return
  }
}

let resizeObserver: ResizeObserver | null = null

onMounted(() => {
  const sid = route.params.sid as string
  if (sid) {
    store.connectAndFetch(sid)
  }
})

onUnmounted(() => {
  store.disconnect()
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
})

// Attach ResizeObserver when scroll container becomes available (conditionally rendered).
watch(scrollContainer, (el) => {
  if (el && !resizeObserver) {
    resizeObserver = new ResizeObserver(() => updateContainerSize())
    resizeObserver.observe(el)
    updateContainerSize()
  }
})

// Re-connect if subject changes (e.g., route param change).
watch(
  () => route.params.sid,
  (newSid) => {
    if (newSid && typeof newSid === 'string') {
      store.disconnect()
      store.connectAndFetch(newSid)
    }
  },
)
</script>

<style scoped>
.sample-grid-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 16px;
  outline: none;
}

.sample-grid-header {
  margin-bottom: 16px;
}

.sample-grid-header h2 {
  margin: 0 0 8px 0;
}

.sample-grid-controls {
  margin-bottom: 8px;
}

.sample-grid-scroll {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

.sample-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 12px;
}

.sample-card {
  aspect-ratio: 1;
  border-radius: 6px;
  overflow: hidden;
  position: relative;
  background: var(--n-color-embedded);
  border: 2px solid var(--n-border-color);
  cursor: pointer;
  transition: border-color 0.15s ease;
}

.sample-card:hover {
  border-color: var(--n-primary-color-hover, #7fe7c4);
}

/* AC: Visual indicators for selection */
.card-selected {
  border-color: var(--n-primary-color, #63e2b7);
}

.card-focused {
  border-color: var(--n-primary-color, #63e2b7);
}

.sample-grid-empty,
.sample-grid-error {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 200px;
}
</style>
