<template>
  <!--
    LightboxEditor: full-size image viewer with rotation and crop controls.

    This component is rendered inside the SampleGridView lightbox modal
    (double-click on a thumbnail). It shares the same edit store as
    SampleDetailView so changes persist immediately.

    Events: none (communicates via store).
  -->
  <div class="lightbox-editor" data-testid="lightbox-editor">
    <!-- Loading state -->
    <n-spin v-if="loadingEdits" data-testid="lightbox-loading" />

    <!-- Error state -->
    <div v-else-if="loadError" class="lightbox-error" data-testid="lightbox-error">
      <n-text type="error">{{ loadError }}</n-text>
    </div>

    <!-- Main layout -->
    <div v-else class="lightbox-layout">
      <!--
        Image: rotation applied to wrapper, CropEditor overlaid inside.
        AC: rotate first, then crop — crop overlay is inside the rotated wrapper.
      -->
      <div class="lightbox-image-wrap" data-testid="lightbox-image-wrap">
        <div
          class="lightbox-image-rotated"
          :style="{ transform: `rotate(${currentRotation}deg)` }"
        >
          <img
            class="lightbox-image"
            data-testid="lightbox-image"
            :src="imageUrl"
            :alt="`Sample ${props.sampleId}`"
            :style="cropClipStyle"
            @error="onImgError"
          />
          <CropEditor
            :crop-box="currentCrop"
            @update:crop-box="onCropChange"
          />
        </div>
        <div v-if="imgError" class="lightbox-img-error" data-testid="lightbox-img-error">
          Image unavailable
        </div>
      </div>

      <!-- Controls panel -->
      <div class="lightbox-controls" data-testid="lightbox-controls">
        <div class="controls-header">
          <n-text class="controls-title">Edit Controls</n-text>
          <n-button
            size="small"
            :disabled="!hasEdits || store.editSaving"
            data-testid="lightbox-reset-btn"
            @click="onResetEdits"
          >
            Reset Edits
          </n-button>
        </div>

        <!-- Rotation section -->
        <div class="control-section" data-testid="lightbox-rotation-section">
          <n-text class="section-label" depth="3">Rotation</n-text>
          <RotationEditor
            :rotation-deg="currentRotation"
            @update:rotation-deg="onRotationChange"
          />
        </div>

        <!-- Crop section -->
        <div class="control-section" data-testid="lightbox-crop-section">
          <n-text class="section-label" depth="3">Crop</n-text>
          <div class="crop-info" data-testid="lightbox-crop-info">
            <template v-if="currentCrop">
              <n-text depth="3" style="font-size: 12px;">
                x: {{ fmt(currentCrop.x) }}% &nbsp;
                y: {{ fmt(currentCrop.y) }}% &nbsp;
                w: {{ fmt(currentCrop.w) }}% &nbsp;
                h: {{ fmt(currentCrop.h) }}%
              </n-text>
            </template>
            <n-text v-else depth="3" style="font-size: 12px;">
              No crop — drag image to set
            </n-text>
          </div>
          <n-button
            v-if="currentCrop"
            size="small"
            data-testid="lightbox-clear-crop-btn"
            @click="onClearCrop"
          >
            Clear Crop
          </n-button>
        </div>

        <!-- Saving indicator -->
        <div v-if="store.editSaving" class="saving-indicator" data-testid="lightbox-saving-indicator">
          <n-text depth="3">Saving…</n-text>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * LightboxEditor: per-sample rotation + crop editor used inside the grid lightbox modal.
 *
 * Events: none — communicates changes via the samples Pinia store.
 */
import { ref, computed, onMounted } from 'vue'
import { NButton, NSpin, NText } from 'naive-ui'
import { useSamplesStore } from '../stores/samples'
import RotationEditor from './RotationEditor.vue'
import CropEditor from './CropEditor.vue'
import type { CropBox } from '../api/samples'

const props = defineProps<{
  sampleId: string
}>()

const store = useSamplesStore()

const imageUrl = computed(() => `/v1/media/${props.sampleId}/full`)

const currentRotation = ref(0)
const currentCrop = ref<CropBox | null>(null)
const loadingEdits = ref(false)
const loadError = ref<string | null>(null)
const imgError = ref(false)

const hasEdits = computed(() => store.editsBySampleId.has(props.sampleId))

function onImgError(): void {
  imgError.value = true
}

/**
 * CSS clip-path to preview the crop region.
 * AC: Crop and rotation compose correctly — clip-path is applied to the image
 * inside the rotated wrapper.
 */
const cropClipStyle = computed(() => {
  if (!currentCrop.value) return {}
  const { x, y, w, h } = currentCrop.value
  return {
    clipPath: `inset(${y}% ${100 - x - w}% ${100 - y - h}% ${x}%)`,
  }
})

function fmt(n: number): string {
  return n.toFixed(1)
}

async function loadEditsForSample(id: string): Promise<void> {
  loadingEdits.value = true
  loadError.value = null
  imgError.value = false
  try {
    const edits = await store.loadEdits(id)
    currentRotation.value = edits?.rotation_deg ?? 0
    currentCrop.value = edits?.crop_box ?? null
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : 'Failed to load edits'
  } finally {
    loadingEdits.value = false
  }
}

function onRotationChange(deg: number): void {
  currentRotation.value = deg
  const existingEdits = store.editsBySampleId.get(props.sampleId)
  store.saveEditsDebounced(props.sampleId, {
    rotation_deg: deg,
    crop_box: currentCrop.value,
    auto_actions: existingEdits?.auto_actions ?? [],
  })
}

function onCropChange(box: CropBox | null): void {
  currentCrop.value = box
  const existingEdits = store.editsBySampleId.get(props.sampleId)
  store.saveEditsDebounced(props.sampleId, {
    rotation_deg: currentRotation.value,
    crop_box: box,
    auto_actions: existingEdits?.auto_actions ?? [],
  })
}

function onClearCrop(): void {
  onCropChange(null)
}

async function onResetEdits(): Promise<void> {
  await store.resetEdits(props.sampleId)
  currentRotation.value = 0
  currentCrop.value = null
}

onMounted(() => {
  loadEditsForSample(props.sampleId)
})
</script>

<style scoped>
.lightbox-editor {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.lightbox-layout {
  display: flex;
  gap: 24px;
  height: 100%;
  min-height: 0;
}

.lightbox-image-wrap {
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

.lightbox-image-rotated {
  position: relative;
  display: inline-block;
  transition: transform 0.2s ease;
  max-width: 100%;
  max-height: 100%;
}

.lightbox-image {
  display: block;
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

.lightbox-img-error {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--n-text-color-3, #888);
  font-size: 14px;
}

.lightbox-controls {
  width: 260px;
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

.crop-info {
  min-height: 20px;
}

.saving-indicator {
  font-size: 12px;
}

.lightbox-error {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 200px;
}
</style>
