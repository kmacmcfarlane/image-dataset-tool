<template>
  <div class="sample-detail-view" data-testid="view-sample-detail">
    <!-- Loading state -->
    <n-spin v-if="loadingEdits" data-testid="sample-detail-loading" />

    <!-- Error state -->
    <div v-else-if="loadError" class="detail-error" data-testid="sample-detail-error">
      <n-text type="error">{{ loadError }}</n-text>
    </div>

    <!-- Main layout -->
    <div v-else class="detail-layout">
      <!-- Image preview with CSS rotation applied (raw bytes untouched) -->
      <div class="detail-image-wrap" data-testid="sample-detail-image-wrap">
        <img
          class="detail-image"
          data-testid="sample-detail-image"
          :src="imageUrl"
          :alt="`Sample ${sampleId}`"
          :style="{ transform: `rotate(${currentRotation}deg)` }"
          @error="onImgError"
        />
        <div v-if="imgError" class="img-error-overlay" data-testid="sample-detail-img-error">
          Image unavailable
        </div>
      </div>

      <!-- Edit controls panel -->
      <div class="detail-controls" data-testid="sample-detail-controls">
        <div class="controls-header">
          <n-text class="controls-title">Edit Controls</n-text>
          <n-button
            size="small"
            :disabled="!hasEdits || store.editSaving"
            data-testid="reset-edits-btn"
            @click="onResetEdits"
          >
            Reset Edits
          </n-button>
        </div>

        <!-- Rotation section -->
        <div class="control-section" data-testid="rotation-section">
          <n-text class="section-label" depth="3">Rotation</n-text>
          <RotationEditor
            :rotation-deg="currentRotation"
            @update:rotation-deg="onRotationChange"
          />
        </div>

        <!-- Saving indicator -->
        <div v-if="store.editSaving" class="saving-indicator" data-testid="saving-indicator">
          <n-text depth="3">Saving…</n-text>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * SampleDetailView: per-sample editor with rotation controls.
 *
 * - Loads existing edits on mount via the samples store (S-019 infrastructure).
 * - CSS rotation preview: raw bytes are never modified.
 * - Auto-saves via debounced store action.
 * - Reset button clears all edits immediately.
 *
 * Route: /projects/:id/subjects/:sid/samples/:sampleId
 */
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NSpin, NText } from 'naive-ui'
import { useSamplesStore } from '../stores/samples'
import RotationEditor from '../components/RotationEditor.vue'

const route = useRoute()
const store = useSamplesStore()

const sampleId = computed(() => route.params.sampleId as string)

// Derived image URL (full-size, not thumbnail).
const imageUrl = computed(() => `/v1/media/${sampleId.value}/full`)

// Local rotation state (mirrors stored value; drives CSS transform).
const currentRotation = ref(0)

const loadingEdits = ref(false)
const loadError = ref<string | null>(null)
const imgError = ref(false)

// Whether there are any edits loaded (drives Reset button enabled state).
const hasEdits = computed(() => store.editsBySampleId.has(sampleId.value))

function onImgError(): void {
  imgError.value = true
}

// AC: load persisted edits on mount and after sample changes.
async function loadEditsForSample(id: string): Promise<void> {
  loadingEdits.value = true
  loadError.value = null
  imgError.value = false

  try {
    const edits = await store.loadEdits(id)
    // AC: Persists across page reloads — hydrate local state from backend.
    currentRotation.value = edits?.rotation_deg ?? 0
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : 'Failed to load edits'
  } finally {
    loadingEdits.value = false
  }
}

// AC: auto-save rotation via debounced store action.
function onRotationChange(deg: number): void {
  currentRotation.value = deg
  const existingEdits = store.editsBySampleId.get(sampleId.value)
  store.saveEditsDebounced(sampleId.value, {
    rotation_deg: deg,
    crop_box: existingEdits?.crop_box ?? null,
    auto_actions: existingEdits?.auto_actions ?? [],
  })
}

// AC: reset button clears all edits immediately.
async function onResetEdits(): Promise<void> {
  await store.resetEdits(sampleId.value)
  currentRotation.value = 0
}

onMounted(() => {
  if (sampleId.value) {
    loadEditsForSample(sampleId.value)
  }
})

// Re-load if the route param changes (e.g., navigating between samples).
watch(sampleId, (id) => {
  if (id) {
    loadEditsForSample(id)
  }
})
</script>

<style scoped>
.sample-detail-view {
  display: flex;
  flex-direction: column;
  height: 100%;
  padding: 16px;
}

.detail-layout {
  display: flex;
  gap: 24px;
  height: 100%;
  min-height: 0;
}

.detail-image-wrap {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--n-color-embedded, #1a1a1a);
  border-radius: 6px;
  overflow: hidden;
  position: relative;
}

.detail-image {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  /* AC: Preview via CSS transform; raw bytes untouched */
  transition: transform 0.2s ease;
}

.img-error-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--n-text-color-3, #888);
  font-size: 14px;
}

.detail-controls {
  width: 280px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.controls-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.controls-title {
  font-weight: 600;
  font-size: 15px;
}

.control-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.section-label {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.saving-indicator {
  font-size: 12px;
}

.detail-error {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
}
</style>
